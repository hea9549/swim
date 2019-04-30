/*
 * Copyright 2018 De-labtory
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package swim

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim/pb"
	"github.com/gogo/protobuf/proto"
)

var ErrSendTimeout = errors.New("Error send timeout")
var ErrUnreachable = errors.New("Error this shouldn't reach")
var ErrInvalidMessage = errors.New("Error invalid message")
var ErrCallbackCollectIntervalNotSpecified = errors.New("Error callback collect interval should be specified")

// callback is called when target member sent back to local member a message
// created field is for clean up the old callback
type callback struct {
	fn      func(msg pb.Message)
	created time.Time
}

// responseHandler manages callback functions
type responseHandler struct {
	callbacks       map[string]callback
	collectInterval time.Duration
	lock            sync.RWMutex
}

func newResponseHandler(collectInterval time.Duration) *responseHandler {
	h := &responseHandler{
		callbacks:       make(map[string]callback),
		collectInterval: collectInterval,
		lock:            sync.RWMutex{},
	}

	go h.collectGarbageCallback()

	return h
}

func (r *responseHandler) addCallback(seq string, cb callback) {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.callbacks[seq] = cb
}

// Handle, each time other member sent back
// a message, callback matching message's seq is called
func (r *responseHandler) handle(msg pb.Message) {
	r.lock.Lock()
	defer r.lock.Unlock()

	seq := msg.Id
	cb, exist := r.callbacks[seq]

	if exist == false {
		iLogger.Error(nil, "Panic, no matching callback function")
	}

	cb.fn(msg)
	delete(r.callbacks, seq)
}

func (r *responseHandler) hasCallback(seq string) bool {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for s := range r.callbacks {
		if seq == s {
			return true
		}
	}
	return false
}

// collectCallback every time callbackCollectInterval expired clean up
// the old (time.now - callback.created > time interval) callback delete from map
// callbackCollectInterval specified in config
func (r *responseHandler) collectGarbageCallback() {
	timeout := r.collectInterval
	T := time.NewTicker(timeout)

	for {
		select {
		case <-T.C:
			for seq, cb := range r.callbacks {
				if time.Now().Sub(cb.created) > timeout {
					delete(r.callbacks, seq)
				}
			}
		}
	}
}

type MessageEndpoint interface {
	Listen()
	SyncSend(addr string, msg pb.Message) (pb.Message, error)
	Send(addr string, msg pb.Message) error
	Shutdown()
}

type MessageEndpointConfig struct {
	ID                string
	EncryptionEnabled bool
	SendTimeout       time.Duration

	// callbackCollect Interval indicate time interval to clean up old
	// callback function
	CallbackCollectInterval time.Duration
}

// MessageEndpoint basically do receiving packet and determine
// which logic executed based on the packet.
type DefaultMessageEndpoint struct {
	config         MessageEndpointConfig
	transport      UDPTransport
	messageHandler MessageHandler
	resHandler     *responseHandler
	quit           chan struct{}
}

func NewMessageEndpoint(config MessageEndpointConfig, transport UDPTransport, messageHandler MessageHandler) (MessageEndpoint, error) {
	if config.CallbackCollectInterval == time.Duration(0) {
		return nil, ErrCallbackCollectIntervalNotSpecified
	}

	return &DefaultMessageEndpoint{
		config:         config,
		transport:      transport,
		messageHandler: messageHandler,
		resHandler:     newResponseHandler(config.CallbackCollectInterval),
		quit:           make(chan struct{}),
	}, nil
}

// Listen is a log running goroutine that pulls packet from the
// transport and pass it for processing
func (m *DefaultMessageEndpoint) Listen() {
	for {
		select {
		case packet := <-m.transport.PacketCh():
			// validate packet then convert it to message
			msg, err := m.processPacket(*packet)
			if err != nil {
				iLogger.Error(nil, err.Error())
			}

			// before message that come from other handle by MessageHandler
			// check whether this message is sent-back message from other member
			// this is determined by message's Seq property which work as message id

			if m.resHandler.hasCallback(msg.Id) {
				go m.resHandler.handle(msg)
			} else {
				go m.handleMessage(msg)
			}

		case <-m.quit:
			return
		}
	}
}

// ProcessPacket process given packet, this procedure may include
// decrypting data and converting it to message
func (m *DefaultMessageEndpoint) processPacket(packet Packet) (pb.Message, error) {
	msg := &pb.Message{}
	if m.config.EncryptionEnabled {
		// TODO: decrypt packet
	}

	if err := proto.Unmarshal(packet.Buf, msg); err != nil {
		return pb.Message{}, err
	}

	return *msg, nil
}

func validateMessage(msg pb.Message) bool {
	if msg.Id == "" {
		iLogger.Info(nil, "message seq value empty")
		return false
	}

	if msg.Payload == nil {
		iLogger.Info(nil, "message payload value empty")
		return false
	}

	return true
}

// with given message handleMessage determine which logic should be executed
// based on the message type. Additionally handleMessage can call MemberDelegater
// to update member status and encrypt messages
func (m *DefaultMessageEndpoint) handleMessage(msg pb.Message) error {
	// validate message
	if !validateMessage(msg) {
		return ErrInvalidMessage
	}

	// call delegate func to update members states
	m.messageHandler.handle(msg)
	return nil

}

// SyncSend synchronously send message to member of addr, waits until response come back,
// whether it is timeout or send failed, SyncSend can be used in the case of pinging to other members.
// if @timeout is provided then set send timeout to given parameters, if not then calculate
// timeout based on the its awareness
func (m *DefaultMessageEndpoint) SyncSend(addr string, msg pb.Message) (pb.Message, error) {
	onSucc := make(chan pb.Message)
	defer close(onSucc)

	d, err := proto.Marshal(&msg)
	if err != nil {
		return pb.Message{}, err
	}

	// register callback function, this callback function is called when
	// member with @addr sent back us packet
	m.resHandler.addCallback(msg.Id, callback{
		fn: func(msg pb.Message) {
			switch p := msg.Payload.(type) {
			case *pb.Message_Ping:
				fmt.Println("PROCESS : " + m.config.ID + ",MSG : ping," + msg.Address+","+p.Ping.String())
			case *pb.Message_Ack:
				fmt.Println("PROCESS : " + m.config.ID + ",MSG : ack," + msg.Address)
			case *pb.Message_IndirectPing:
				fmt.Println("PROCESS : " + m.config.ID + ",MSG : indp" + msg.Address)
			case *pb.Message_Membership:
				fmt.Println("PROCESS : " + m.config.ID + ",MSG : mem" + msg.Address)
			default:
				fmt.Println("PROCESS : " + m.config.ID + ",MSG : DEFA!!,")
			}

			onSucc <- msg
		},
		created: time.Now(),
	})
	iLogger.Warn(nil, "add callback, id : "+msg.Id+"to : "+msg.Address)

	// send the message
	_, err = m.transport.WriteTo(d, addr)
	if err != nil {
		iLogger.Error(nil, err.Error())
		return pb.Message{}, err
	}

	// start timer
	T := time.NewTimer(m.config.SendTimeout)

	select {
	case msg := <-onSucc:
		return msg, nil
	case <-T.C:
		return pb.Message{}, ErrSendTimeout
	}

	return pb.Message{}, ErrUnreachable
}

// Send asynchronously send message to member of addr, don't wait until response come back,
// after response came back, callback function executed, Send can be used in the case of
// gossip message to other members
func (m *DefaultMessageEndpoint) Send(addr string, msg pb.Message) error {
	d, err := proto.Marshal(&msg)
	if err != nil {
		return err
	}

	// send the message
	_, err = m.transport.WriteTo(d, addr)
	if err != nil {
		iLogger.Info(nil, err.Error())
		return err
	}

	return nil
}

func (m *DefaultMessageEndpoint) Shutdown() {
	// close transport first
	m.transport.Shutdown()

	// then close message endpoint
	m.quit <- struct{}{}
}
