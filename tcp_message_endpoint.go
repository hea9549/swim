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
	"fmt"
)

const (
	fillBufChar = ":"
	// sendDataSizeInfoLength is used to check size of data to send
	sendDataSizeInfoLength = 30
	sendIDInfoLength       = 60
	tcpBufferSize          = 1024
	endOfIdChar            = "*EOI*"
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
	sendLockMap    map[MemberID]*sync.Mutex
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
		connMap:        make(map[MemberID]net.Conn),
		sendLockMap:    make(map[MemberID]*sync.Mutex),
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

		idBuf := make([]byte, 1024)
		_, err = conn.Read(idBuf)
		if err != nil {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in read ID in listen")
			conn.Close()
			continue
		}
		recvStr := string(idBuf[:])
		if strings.Contains(recvStr, endOfIdChar) != true {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in recv Id in listen")
			conn.Close()
			continue
		}


		_, err = conn.Write([]byte(t.config.MyId.ID + endOfIdChar))
		if err != nil {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in write Id in listen")
			conn.Close()
			continue
		}


		recvId := recvStr[:len(recvStr)-len(endOfIdChar)]
		t.connMap[MemberID{ID: recvId}] = conn
		lock := &sync.Mutex{}
		t.sendLockMap[MemberID{ID: recvId}] = lock
		go t.startReceiver(MemberID{ID: recvId}, conn)
	}
}

func (t *TCPMessageEndpoint) Send(mbr Member, msg pb.Message) error {
	if mbr.ID == (MemberID{}) {
		_, _, err := t.makeConnAndLock(mbr.Address())
		if err != nil{
			return err
		}
	}
	conn, l, err := t.getConnAndLock(mbr.ID)

	if err == ErrConnIsNotExist {
		conn, l, err = t.makeConnAndLock(mbr.Address())
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
		t.removeTCPInfo(mbr.ID)
		return err
	}

	sendBuffer := make([]byte, tcpBufferSize)
	for len(data) > 0 {
		if len(data) > tcpBufferSize {

			copy(sendBuffer, data[:tcpBufferSize])

			_, err = conn.Write(sendBuffer)
			if err != nil {
				t.removeTCPInfo(mbr.ID)
				return err
			}
			data = data[tcpBufferSize+1:]
			continue
		}

		copy(sendBuffer, data)

		_, err = conn.Write(sendBuffer)
		if err != nil {
			t.removeTCPInfo(mbr.ID)
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
func (t *TCPMessageEndpoint) startReceiver(mbrId MemberID, conn net.Conn) {

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
			t.removeTCPInfo(mbrId)
			return
		}

		dataSize, err := strconv.ParseInt(strings.Trim(string(bufferDataSize), fillBufChar), 10, 64)

		if err != nil {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver parse int")

			conn.Close()
			t.removeTCPInfo(mbrId)
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
					t.removeTCPInfo(mbrId)
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
				t.removeTCPInfo(mbrId)
				return
			}

			receivedData = append(receivedData, tempReceived...)
			receivedBytes += tcpBufferSize
		}

		msg := &pb.Message{}
		if err := proto.Unmarshal(receivedData, msg); err != nil {
			iLogger.Error(nil, "[TCPMessageEndpoint] error in TCP receiver unmarshal received data")

			conn.Close()
			t.removeTCPInfo(mbrId)
			return
		}
		t.messageHandler.handle(*msg)

	}

}
func (t *TCPMessageEndpoint) makeConnAndLock(addr string) (net.Conn, *sync.Mutex, error) {
	c, err := net.DialTimeout("tcp", addr, t.config.TCPTimeout)
	if err != nil {
		return nil, nil, err
	}

	// exchange id
	arr := []byte(t.config.MyId.ID + endOfIdChar)
	strt := string(arr[:])
	fmt.Println(strt)
	_, err = c.Write([]byte(t.config.MyId.ID + endOfIdChar))
	if err != nil {
		return nil, nil, err
	}
	idBuf := make([]byte, 1024)
	_, err = c.Read(idBuf)
	if err != nil {
		return nil, nil, err
	}
	recvStr := string(idBuf[:])
	if strings.Contains(recvStr, endOfIdChar) != true {
		return nil, nil, errors.New("error in recv Id")
	}
	recvId := recvStr[:len(recvStr)-len(endOfIdChar)]

	t.connMap[MemberID{ID: recvId}] = c

	lock := &sync.Mutex{}
	t.sendLockMap[MemberID{ID: recvId}] = lock

	return c, lock, nil
}

func (t *TCPMessageEndpoint) getConnAndLock(id MemberID) (net.Conn, *sync.Mutex, error) {
	t.processLock.Lock()
	defer t.processLock.Unlock()

	conn, ok := t.connMap[id]
	if !ok {
		return nil, nil, ErrConnIsNotExist
	}

	sLock, ok := t.sendLockMap[id]
	if !ok {
		lock := sync.Mutex{}
		t.sendLockMap[id] = &lock
		sLock = &lock
	}

	return conn, sLock, nil
}

func (t *TCPMessageEndpoint) removeTCPInfo(id MemberID) {
	if conn, ok := t.connMap[id]; ok {
		conn.Close()
		delete(t.connMap, id)
	}
	delete(t.sendLockMap, id)
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
