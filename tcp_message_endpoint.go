package swim

import (
	"errors"
	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim/pb"
	"github.com/golang/protobuf/proto"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	fillBufChar = ":"
	// sendDataSizeInfoLength is used to check size of data to send
	sendDataSizeInfoLength = 30
	tcpBufferSize          = 1024
)

var ErrConnIsNotExist = errors.New("no connection in map")

type TCPMessageEndpointConfig struct {
	EncryptionEnabled bool
	TCPTimeout        time.Duration
	IP                string
	Port              int
	MyId              MemberID
}

type TCPMessageEndpoint struct {
	config         TCPMessageEndpointConfig
	messageHandler MessageHandler
	connMap        map[MemberID]net.Conn
	resMsgGetFunc  func() pb.Message
	isShutdown     bool
}

func NewTCPMessageEndpoint(config TCPMessageEndpointConfig, messageHandler MessageHandler, resMsgGetter func() pb.Message) TCPMessageEndpoint {
	if config.TCPTimeout == time.Duration(0) {
		config.TCPTimeout = 2 * time.Second // def timeout = 2sec
	}

	return TCPMessageEndpoint{
		config:         config,
		messageHandler: messageHandler,
		connMap:        make(map[MemberID]net.Conn),
		resMsgGetFunc:  resMsgGetter,
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
			return
		}
		conn, err := server.Accept()
		if err != nil {
			_ = conn.Close()
			iLogger.Error(nil, "[TCPMessageEndpoint] error in accept")
			continue
		}

		go t.doListen(conn)

	}
}

func (t *TCPMessageEndpoint) doListen(conn net.Conn) {
	if t.isShutdown{
		return
	}
	err := t.recvMessage(conn)
	if err != nil {
		iLogger.Error(nil, "[TCPMessageEndpoint] error in do listen")
		_ = conn.Close()
		return
	}

	err = t.sendMessage(conn, t.resMsgGetFunc())
	if err != nil {
		iLogger.Error(nil, "[TCPMessageEndpoint] error in do listen")
		_ = conn.Close()
		return
	}
}

func (t *TCPMessageEndpoint) ExchangeMessage(addr string, msg pb.Message) error {
	c, err := net.DialTimeout("tcp", addr, t.config.TCPTimeout)
	if err != nil {
		return err
	}
	err = t.sendMessage(c, msg)
	if err != nil {
		return err
	}
	err = t.recvMessage(c)

	if err != nil {
		return err
	}

	_ = c.Close()
	return nil
}

func (t *TCPMessageEndpoint) Shutdown() {
	t.isShutdown = true

	for _, conn := range t.connMap {
		_ = conn.Close()
	}
}

func (t *TCPMessageEndpoint) sendMessage(conn net.Conn, message pb.Message) error {
	data, err := proto.Marshal(&message)
	dataLength := fillString(strconv.Itoa(len(data)), sendDataSizeInfoLength)

	err = conn.SetWriteDeadline(time.Now().Add(t.config.TCPTimeout))
	if err != nil {
		return err
	}
	_, err = conn.Write([]byte(dataLength))
	if err != nil {
		return err
	}

	for len(data) > 0 {
		if len(data) > tcpBufferSize {
			sendBuffer := make([]byte, tcpBufferSize)
			copy(sendBuffer, data[:tcpBufferSize])

			_, err = conn.Write(sendBuffer)
			if err != nil {
				return err
			}
			data = data[tcpBufferSize:]
			continue
		}
		sendBuffer := make([]byte, len(data))
		copy(sendBuffer, data)

		_, err = conn.Write(sendBuffer)
		if err != nil {
			return err
		}

		data = data[len(data):]
	}

	return nil
}

func (t *TCPMessageEndpoint) recvMessage(conn net.Conn) error {
	bufferDataSize := make([]byte, sendDataSizeInfoLength)

	err := conn.SetReadDeadline(time.Now().Add(t.config.TCPTimeout))
	if err != nil {
		iLogger.Error(nil, "[TCPMessageEndpoint] error in recvMessage, deadline: "+err.Error())
	}
	_, err = conn.Read(bufferDataSize)
	if err != nil {
		iLogger.Error(nil, "[TCPMessageEndpoint] error in recvMessage, READ : "+err.Error())
		_ = conn.Close()
		return err
	}

	dataSize, err := strconv.ParseInt(strings.Trim(string(bufferDataSize), fillBufChar), 10, 64)

	if err != nil {
		iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver parse int")
		_ = conn.Close()
		return err
	}
	var receivedBytes int64
	receivedData := make([]byte, 0)
	for {
		if (dataSize - receivedBytes) < tcpBufferSize {
			tempReceived := make([]byte, dataSize-receivedBytes)
			_, err := conn.Read(tempReceived)

			if err != nil {
				iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver read received")

				_ = conn.Close()
				return err
			}
			receivedData = append(receivedData, tempReceived...)
			break
		}
		tempReceived := make([]byte, tcpBufferSize)
		_, err := conn.Read(tempReceived)

		if err != nil {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver read max received")

			_ = conn.Close()
			return err
		}

		receivedData = append(receivedData, tempReceived...)
		receivedBytes += tcpBufferSize
	}

	msg := &pb.Message{}
	if err := proto.Unmarshal(receivedData, msg); err != nil {
		iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver unmarshal received data")

		_ = conn.Close()
		return err
	}
	t.messageHandler.handle(*msg)
	return nil
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
