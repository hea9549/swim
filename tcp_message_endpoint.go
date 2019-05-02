package swim

import (
	"errors"
	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim/pb"
	"github.com/golang/protobuf/proto"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	fillBufChar = ":"
	// sendDataSizeInfoLength is used to check size of data to send
	sendDataSizeInfoLength = 30
	tcpBufferSize          = 65536
)

var ErrConnIsNotExist = errors.New("no connection in map")

type TCPMessageEndpointConfig struct {
	EncryptionEnabled bool
	TCPTimeout        time.Duration
	IP                string
	Port              int
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

	server, err := net.Listen("tcp", t.config.IP+":"+strconv.Itoa(t.config.Port))
	if err != nil {
		iLogger.Panic(nil, "[TCPMessageEndpoint] panic in initial listen")
	}
	for {
		if t.isShutdown {
			//todo is enough ?
			return
		}
		conn, err := server.Accept()
		if err != nil {
			conn.Close()
			iLogger.Error(nil, "[TCPMessageEndpoint] error in accept")
			continue
		}

		t.connMap[conn.RemoteAddr().String()] = conn
		t.sendLockMap[conn.RemoteAddr().String()] = &sync.Mutex{}
		go t.startReceiver(conn)
	}
}
func (t *TCPMessageEndpoint) Send(addr string, msg pb.Message) error {
	conn, l, err := t.getConnAndLock(addr)

	if err == ErrConnIsNotExist {
		conn, l, err = t.makeConnAndLock(addr)
	}

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

func (t *TCPMessageEndpoint) Shutdown() {
	t.isShutdown = true

	for _, conn := range t.connMap {
		conn.Close()
	}
}
func (t *TCPMessageEndpoint) startReceiver(conn net.Conn) {

	for {
		// todo check shutdown gracefully
		if t.isShutdown {
			return
		}

		bufferDataSize := make([]byte, sendDataSizeInfoLength)

		_, err := conn.Read(bufferDataSize)
		if err != nil {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver, read data size")

			conn.Close()
			t.removeAddrInfo(conn.RemoteAddr().String())
			return
		}

		dataSize, err := strconv.ParseInt(strings.Trim(string(bufferDataSize), fillBufChar), 10, 64)

		if err != nil {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver parse int")

			conn.Close()
			t.removeAddrInfo(conn.RemoteAddr().String())
			return
		}
		var receivedBytes int64
		receivedData := make([]byte, 0)
		for {

			if (dataSize - receivedBytes) < tcpBufferSize {
				tempReceived := make([]byte, dataSize-receivedBytes)
				_, err := conn.Read(tempReceived)

				if err != nil {
					iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver read received")

					conn.Close()
					t.removeAddrInfo(conn.RemoteAddr().String())
					return
				}
				receivedData = append(receivedData, tempReceived...)
				break
			}
			tempReceived := make([]byte, tcpBufferSize)
			_, err := conn.Read(tempReceived)

			if err != nil {
				iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver read max received")

				conn.Close()
				t.removeAddrInfo(conn.RemoteAddr().String())
				return
			}

			receivedData = append(receivedData, tempReceived...)
			receivedBytes += tcpBufferSize
		}

		msg := &pb.Message{}
		if err := proto.Unmarshal(receivedData, msg); err != nil {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver unmarshal received data")

			conn.Close()
			t.removeAddrInfo(conn.RemoteAddr().String())
			return
		}
		t.messageHandler.handle(*msg)

	}

}
func (t *TCPMessageEndpoint) makeConnAndLock(addr string) (net.Conn, *sync.Mutex, error) {
	c, l, e := t.getConnAndLock(addr)
	if e == nil {
		return c, l, e
	}
	dialer := &net.Dialer{
		Timeout: t.config.TCPTimeout,
		LocalAddr: &net.TCPAddr{
			IP:   net.ParseIP(t.config.IP),
			Port: t.config.Port,
		},

	}
	c, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	t.connMap[addr] = c

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
