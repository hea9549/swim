package swim

import (
	"net"
	"time"
	"github.com/DE-labtory/swim/pb"
	"sync"
	"github.com/golang/protobuf/proto"
	"strconv"
	"errors"
)

const (
	fillBufChar = ":"
	// sendDataSizeInfoLength is used to check size of data to send
	sendDataSizeInfoLength = 15
	tcpBufferSize          = 65536
)

var ErrConnIsNotExist = errors.New("no connection in map")

type TCPMessageEndpointConfig struct {
	EncryptionEnabled bool
	TCPTimeout        time.Duration
	ServerAddress     string
}

type TCPMessageEndpoint struct {
	config         TCPMessageEndpointConfig
	messageHandler MessageHandler
	connMap        map[string]net.Conn
	sendLockMap    map[string]*sync.Mutex
	processCh      chan pb.Message
	processLock    sync.Mutex
	isShutdown     bool
}

func NewTCPMessageEndpoint(config TCPMessageEndpointConfig, messageHandler MessageHandler) TCPMessageEndpoint {
	if config.TCPTimeout == time.Duration(0) {
		config.TCPTimeout = 2 * time.Second // def timeout = 2sec
	}

	return TCPMessageEndpoint{
		config:         config,
		messageHandler: messageHandler,
		connMap:        make(map[string]net.Conn),
		sendLockMap:    make(map[string]*sync.Mutex),
		processLock:    sync.Mutex{},
		isShutdown:     false,
	}
}
func (t *TCPMessageEndpoint) Listen() {
	for {
		select {
		case msg := <-t.processCh:
			t.messageHandler.handle(msg)

		}
	}
}
func (t *TCPMessageEndpoint) Send(addr string, msg pb.Message) error {
	conn, l, err := t.getConnAndLock(addr)
	if err != nil {
		return err
	}

	l.Lock()
	defer l.Unlock()

	data, err := proto.Marshal(&msg)
	dataLength := fillString(strconv.Itoa(len(data)), sendDataSizeInfoLength)

	_, err = conn.Write([]byte(dataLength))
	if err != nil {
		t.removeAddrInfo(addr)
		return err
	}

	sendBuffer := make([]byte, tcpBufferSize)
	for len(data) > 0 {
		if len(data) > tcpBufferSize {

			copy(sendBuffer, data[:tcpBufferSize])

			_, err = conn.Write(sendBuffer)
			if err != nil {
				t.removeAddrInfo(addr)
				return err
			}
			data = data[tcpBufferSize+1:]
			continue
		}

		copy(sendBuffer, data)

		_, err = conn.Write(sendBuffer)
		if err != nil {
			t.removeAddrInfo(addr)
			return err
		}

		data = data[len(data):]
	}

	return nil
}
func (t *TCPMessageEndpoint) listen() error {
	server, err := net.Listen("tcp", t.config.ServerAddress)
	if err != nil {
		return err
	}
	for {
		if t.isShutdown {
			//todo make this
			return nil
		}
		conn,err := server.Accept()
		if err!=nil{
			return err
		}

		t.connMap[conn.RemoteAddr().String()] = conn
		t.sendLockMap[conn.RemoteAddr().String()] = &sync.Mutex{}

	}
}
func (t *TCPMessageEndpoint) makeConnAndLock(addr string) (net.Conn, *sync.Mutex, error) {
	c, l, e := t.getConnAndLock(addr)
	if e == nil {
		return c, l, e
	}

	c, err := net.DialTimeout("tcp", addr, t.config.TCPTimeout)
	if err != nil {
		return nil, nil, err
	}

	t.connMap[addr] = &c

	lock := &sync.Mutex{}
	t.sendLockMap[addr] = lock

	return c, lock, nil
}

func (t *TCPMessageEndpoint) getConnAndLock(addr string) (net.Conn, *sync.Mutex, error) {
	t.processLock.Lock()
	defer t.processLock.Unlock()

	conn, ok := t.connMap[addr]
	if !ok {
		return nil, nil, ErrConnIsNotExist
	}

	sLock, ok := t.sendLockMap[addr]
	if !ok {
		lock := sync.Mutex{}
		t.sendLockMap[addr] = &lock
		sLock = &lock
	}

	return conn, sLock, nil
}

func (t *TCPMessageEndpoint) removeAddrInfo(addr string) {
	if conn, ok := t.connMap[addr]; ok {
		conn.Close()
		delete(t.connMap, addr)
	}
	delete(t.sendLockMap, addr)
}

func fillString(rawStr string, toLength int) string {
	for {
		strLen := len(rawStr)
		if strLen < toLength {
			rawStr = rawStr + fillBufChar
			continue
		}
		break
	}
	return rawStr
}
