package evaluator

import (
	"github.com/DE-labtory/swim"
	"strconv"
	"net"
	"time"
)

func GetAvailablePort(startPort int) int {
	portNumber := startPort
	for {
		strPortNumber := strconv.Itoa(portNumber)
		lis, Err := net.Listen("tcp", "127.0.0.1:"+strPortNumber)

		if Err == nil {
			lis.Close()
			return portNumber
		}

		portNumber++

	}
}
func GetAvailablePortList(startPort int, num int) []int {
	portNumber := startPort
	portList := make([]int, 0)
	for {
		strPortNumber := strconv.Itoa(portNumber)
		lis, Err := net.Listen("tcp", "127.0.0.1:"+strPortNumber)

		if Err == nil {
			portList = append(portList, portNumber)
			lis.Close()
			if len(portList) == num {
				return portList
			}
		}

		portNumber++

	}
}

func SetupSwim(ip string, udpPort int, tcpPort int, id string) (*swim.SWIM, *swim.EvaluatorMessageEndpoint) {
	swimConfig := swim.Config{
		MaxlocalCount: 5,
		MaxNsaCounter: 5,
		T:             800,
		AckTimeOut:    100,
		K:             2,
		BindAddress:   ip,
		BindPort:      udpPort,
	}
	suspicionConfig := swim.SuspicionConfig{
		K:        3,
		MinParam: 5,
		MaxParam: 1,
	}
	messageEndpointConfig := swim.MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             100 * time.Millisecond,
		CallbackCollectInterval: time.Minute,
	}

	tcpEndpointconfig := swim.TCPMessageEndpointConfig{
		EncryptionEnabled: false,
		TCPTimeout:        20 * time.Second,
		IP:                ip,
		Port:              tcpPort,
		MyId:              swim.MemberID{ID: id},
	}
	swimObj, msgEndpoint := swim.NewSwimForEvaluate(&swimConfig, &suspicionConfig, messageEndpointConfig, tcpEndpointconfig, &swim.Member{
		ID:               swim.MemberID{ID: id},
		Addr:             net.ParseIP(ip),
		UDPPort:          uint16(udpPort),
		TCPPort:          uint16(tcpPort),
		Status:           swim.Alive,
		LastStatusChange: time.Now(),
		Incarnation:      0,
	})

	return swimObj, msgEndpoint
}