package swim

import (
	"github.com/DE-labtory/iLogger"
	"net"
	"sync"
	)


func NewSwimForEvaluate(config *Config, suspicionConfig *SuspicionConfig, messageEndpointConfig MessageEndpointConfig,
	tcpMessageEndpointConfig TCPMessageEndpointConfig, member *Member) (*SWIM, *MemberMap) {
	if config.T < config.AckTimeOut {
		iLogger.Panic(nil, "T time must be longer than ack time-out")
	}

	swim := SWIM{
		config:           config,
		awareness:        NewAwareness(config.MaxNsaCounter),
		memberMap:        NewMemberMap(suspicionConfig),
		member:           member,
		quitFD:           make(chan struct{}),
		mbrStatsMsgStore: NewPriorityMbrStatsMsgStore(config.MaxlocalCount),
	}

	packetTransportConfig := PacketTransportConfig{
		BindAddress: config.BindAddress,
		BindPort:    config.BindPort,
	}
	ip := net.ParseIP(config.BindAddress)
	port := config.BindPort

	udpAddr := &net.UDPAddr{IP: ip, Port: port}
	packetListener, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		iLogger.Panic(nil, err.Error())
	}

	if err := setUDPRecvBuf(packetListener); err != nil {
		iLogger.Panic(nil, err.Error())
	}

	if err != nil {
		iLogger.Panic(nil, err.Error())
	}

	t := PacketTransport{
		config:         &packetTransportConfig,
		packetCh:       make(chan *Packet),
		packetListener: packetListener,
		isShutDown:     false,
		lock:           sync.RWMutex{},
	}

	go t.listen()

	if err != nil {
		iLogger.Panic(nil, err.Error())
	}

	messageEndpoint, err := NewEvaluatorMessageEndpoint(messageEndpointConfig, &t, &swim)
	if err != nil {
		iLogger.Panic(nil, err.Error())
	}

	swim.messageEndpoint = messageEndpoint

	tcpMessageEndpoint := NewTCPMessageEndpoint(tcpMessageEndpointConfig, &swim)
	swim.tcpMessageEndpoint = tcpMessageEndpoint

	return &swim, swim.memberMap
}
