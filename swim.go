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
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"sync/atomic"

	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim/pb"
	"github.com/rs/xid"
)

var ErrInvalidMbrStatsMsgType = errors.New("error invalid mbrStatsMsg type")
var ErrPingFailed = errors.New("error ping failed")
var ErrIndProbeFailed = errors.New("error when indirect-probe failed")
var ErrInvalidPayloadType = errors.New("invalid indirect-ping response Payload type")

type ProbeResponse struct {
	err error
}

func (r ProbeResponse) Ok() bool {
	return r.err == nil
}

type IndProbeResponse struct {
	err error
	msg pb.Message
}

func (r IndProbeResponse) Ok() bool {
	return r.err == nil && reflect.TypeOf(r.msg.Payload) == reflect.TypeOf(&pb.Message_Ack{})
}

type Config struct {
	// The maximum number of times the same piggyback data can be queried
	MaxlocalCount int

	// The maximum number of node-self-awareness counter
	MaxNsaCounter int

	// T is the the period of the probe
	T int

	// Timeout of ack after ping to a member
	AckTimeOut int

	// K is the number of members to send indirect ping
	K int

	// my address and port
	BindAddress string
	BindPort    int

	// Exchange Membership Period
	TryExchangeMembershipPeriod int
}

type SWIM struct {
	// Swim Config
	config *Config

	// Currently connected memberList
	memberMap *MemberMap

	// Awareness manages health of the local node.
	awareness *Awareness

	// messageEndpoint work both as message transmitter and message receiver
	messageEndpoint MessageEndpoint

	// tcpMessageEndpoint work both as message transmitter and message receiver in tcp
	tcpMessageEndpoint TCPMessageEndpoint

	// Information of this node
	member *Member

	// stopFlag tells whether SWIM is running or not
	stopFlag int32

	// FailureDetector quit channel
	quitFD chan struct{}

	// MbrStatsMsgStore which store messages about recent state changes of member.
	mbrStatsMsgStore MbrStatsMsgStore

	// calculated current probe interval duration by awareness
	curProbeInterval time.Duration

	// calculated current probe timeout duration by awareness
	curProbeTimeout time.Duration
}

func New(config *Config, suspicionConfig *SuspicionConfig, messageEndpointConfig MessageEndpointConfig,
	tcpMessageEndpointConfig TCPMessageEndpointConfig, member *Member) *SWIM {
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

	messageEndpoint := messageEndpointFactory(config, messageEndpointConfig, &swim)
	swim.messageEndpoint = messageEndpoint

	tcpMessageEndpoint := NewTCPMessageEndpoint(tcpMessageEndpointConfig, &swim, func() pb.Message {
		membership := swim.createMembership()

		msg := pb.Message{
			Address: swim.member.UDPAddress(),
			Id:      xid.New().String(),
			Payload: &pb.Message_Membership{
				Membership: membership,
			},
		}
		return msg
	})
	swim.tcpMessageEndpoint = tcpMessageEndpoint

	return &swim
}

func messageEndpointFactory(config *Config, messageEndpointConfig MessageEndpointConfig, messageHandler MessageHandler) MessageEndpoint {
	packetTransportConfig := PacketTransportConfig{
		BindAddress: config.BindAddress,
		BindPort:    config.BindPort,
	}

	packetTransport, err := NewPacketTransport(&packetTransportConfig)
	if err != nil {
		iLogger.Panic(nil, err.Error())
	}

	messageEndpoint, err := NewMessageEndpoint(messageEndpointConfig, packetTransport, messageHandler)
	if err != nil {
		iLogger.Panic(nil, err.Error())
	}

	return messageEndpoint
}

// Start SWIM protocol.
func (s *SWIM) Start() {
	atomic.CompareAndSwapInt32(&s.stopFlag, DIE, AVAILABLE)
	go s.messageEndpoint.Listen()
	go s.tcpMessageEndpoint.Listen()
	s.startFailureDetector()
}

// Dial to the all peerAddresses and exchange memberList.
func (s *SWIM) Join(peerAddresses []string) error {

	for _, address := range peerAddresses {
		host, port, err := ParseHostPort(address)
		m := Member{Addr: host, TCPPort: port}
		err = s.exchangeMembership(m)
		if err != nil {
			iLogger.Error(nil, "error while join : "+err.Error())
		}

	}

	return nil
}

func (s *SWIM) GetMemberMap() *MemberMap {
	return s.memberMap
}

func (s *SWIM) exchangeMembership(mbr Member) error {
	// Create membership message
	membership := s.createMembership()

	msg := pb.Message{
		Address: s.member.UDPAddress(),
		Id:      xid.New().String(),
		Payload: &pb.Message_Membership{
			Membership: membership,
		},
	}

	err := s.tcpMessageEndpoint.ExchangeMessage(mbr.TCPAddress(), msg)
	if err != nil {
		return err
	}

	return nil
}

func (s *SWIM) createMembership() *pb.Membership {

	membership := &pb.Membership{
		SenderId:         s.member.ID.ID,
		SenderUdpAddress: s.member.UDPAddress(),
		SenderTcpAddress: s.member.TCPAddress(),
		MbrStatsMsgs:     make([]*pb.MbrStatsMsg, 0),
	}
	for _, m := range s.memberMap.GetMembers() {
		membership.MbrStatsMsgs = append(membership.MbrStatsMsgs, &pb.MbrStatsMsg{
			Incarnation: m.Incarnation,
			Id:          m.GetIDString(),
			Type:        pb.MbrStatsMsg_Type(m.Status.Int()),
			UdpAddress:  m.UDPAddress(),
			TcpAddress:  m.TCPAddress(),
		})
	}

	return membership
}

func (s *SWIM) handleMbrStatsMsg(mbrStatsMsg *pb.MbrStatsMsg) {
	msgHandleErr := error(nil)
	hasChanged := false

	if s.member.ID.ID == mbrStatsMsg.Id {
		if mbrStatsMsg.Type == pb.MbrStatsMsg_Alive {
			return
		}
		s.refute(mbrStatsMsg)
		s.mbrStatsMsgStore.Push(*mbrStatsMsg)
		return
	}

	switch mbrStatsMsg.Type {
	case pb.MbrStatsMsg_Alive:
		hasChanged, msgHandleErr = s.handleAliveMbrStatsMsg(mbrStatsMsg)
	case pb.MbrStatsMsg_Suspect:
		hasChanged, msgHandleErr = s.handleSuspectMbrStatsMsg(mbrStatsMsg)
	default:
		msgHandleErr = ErrInvalidMbrStatsMsgType
	}

	if msgHandleErr != nil {
		iLogger.Errorf(nil, "error occured when handling mbrStatsMsg [%s], error [%s]", mbrStatsMsg, msgHandleErr)
		return
	}

	// Push piggyback when status of membermap has updated.
	// If the content of the piggyback is about a new state change,
	// it must propagate to inform the network of the new state change.
	if hasChanged {
		s.mbrStatsMsgStore.Push(*mbrStatsMsg)
	}
}

func (s *SWIM) handleAliveMbrStatsMsg(stats *pb.MbrStatsMsg) (bool, error) {
	if stats.Type != pb.MbrStatsMsg_Alive {
		return false, ErrInvalidMbrStatsMsgType
	}

	msg, err := s.convMbrStatsToAliveMsg(stats)
	if err != nil {
		return false, err
	}

	return s.memberMap.Alive(msg)
}

func (s *SWIM) handleSuspectMbrStatsMsg(stats *pb.MbrStatsMsg) (bool, error) {
	if stats.Type != pb.MbrStatsMsg_Suspect {
		return false, ErrInvalidMbrStatsMsgType
	}

	msg, err := s.convMbrStatsToSuspectMsg(stats)
	if err != nil {
		return false, err
	}

	curProbeInterval := time.Duration(s.awareness.score+1) * time.Millisecond * time.Duration(s.config.T)

	return s.memberMap.Suspect(msg, curProbeInterval)
}

func (s *SWIM) convMbrStatsToAliveMsg(stats *pb.MbrStatsMsg) (AliveMessage, error) {
	if stats.Type != pb.MbrStatsMsg_Alive {
		return AliveMessage{}, ErrInvalidMbrStatsMsgType
	}

	host, udpPort, err := ParseHostPort(stats.UdpAddress)
	_, tcpPort, err := ParseHostPort(stats.TcpAddress)
	if err != nil {
		return AliveMessage{}, err
	}
	return AliveMessage{
		MemberMessage: MemberMessage{
			ID:          stats.Id,
			Addr:        host,
			UDPPort:     udpPort,
			TCPPort:     tcpPort,
			Incarnation: stats.Incarnation,
		},
	}, nil
}

func (s *SWIM) convMbrStatsToSuspectMsg(stats *pb.MbrStatsMsg) (SuspectMessage, error) {
	if stats.Type != pb.MbrStatsMsg_Suspect {
		return SuspectMessage{}, ErrInvalidMbrStatsMsgType
	}

	host, udpPort, err := ParseHostPort(stats.UdpAddress)
	_, tcpPort, err := ParseHostPort(stats.TcpAddress)
	if err != nil {
		return SuspectMessage{}, err
	}
	return SuspectMessage{
		MemberMessage: MemberMessage{
			ID:          stats.Id,
			Addr:        host,
			UDPPort:     udpPort,
			TCPPort:     tcpPort,
			Incarnation: stats.Incarnation,
		},
		ConfirmerID: s.member.ID.ID,
	}, nil
}

func (s *SWIM) refute(mbrStatsMsg *pb.MbrStatsMsg) {

	accusedInc := mbrStatsMsg.Incarnation
	inc := atomic.AddUint32(&s.member.Incarnation, 1)
	if s.member.Incarnation >= accusedInc {
		inc = atomic.AddUint32(&s.member.Incarnation, accusedInc-inc+1)
	}
	s.member.Incarnation = inc

	// Update piggyBack's incarnation to store to pbkStore
	mbrStatsMsg.Incarnation = inc

	// Change msg state to alive
	mbrStatsMsg.Type = pb.MbrStatsMsg_Alive

	// Increase awareness count(Decrease our health) because we are being asked to refute a problem.
	s.awareness.ApplyDelta(1)
}

// Gossip message to p2p network.
func (s *SWIM) Gossip(msg []byte) {

}
func (s *SWIM) IsRun() bool {
	return atomic.LoadInt32(&(s.stopFlag)) == AVAILABLE
}

// Shutdown the running swim.
func (s *SWIM) ShutDown() {
	atomic.CompareAndSwapInt32(&s.stopFlag, AVAILABLE, DIE)
	go s.tcpMessageEndpoint.Shutdown()
	go s.messageEndpoint.Shutdown()
	s.quitFD <- struct{}{}
}

// toDie tell whether SWIM is stopped or not
func (s *SWIM) toDie() bool {
	return atomic.LoadInt32(&(s.stopFlag)) == DIE
}

// Total Failure Detection is performed for each` T`. (ref: https://github.com/DE-labtory/swim/edit/develop/docs/Docs.md)
//
// 1. SWIM randomly selects a member(j) in the memberMap and ping to the member(j).
//
// 2. SWIM waits for ack of the member(j) during the ack-timeout (time less than T).
//    End failure Detector if ack message arrives on ack-timeout.
//
// 3. SWIM selects K number of members from the memberMap and sends indirect-ping(request K members to ping the member(j)).
//    The nodes (that receive the indirect-ping) ping to the member(j) and ack when they receive ack from the member(j).
//
// 4. At the end of T, SWIM checks to see if ack was received from K members, and if there is no message,
//    The member(j) is judged to be failed, so check the member(j) as suspected or delete the member(j) from memberMap.
//
// ** When performing ping, ack, and indirect-ping in the above procedure, piggybackdata is sent together. **
//
//
// startFailureDetector function
//
// 1. Pick a member from memberMap.
// 2. Probe the member.
// 3. After finishing probing all members, Reset memberMap
//
func (s *SWIM) startFailureDetector() {
	go func() {
		baseInterval := time.Millisecond * time.Duration(s.config.T)
		if s.config.TryExchangeMembershipPeriod == 0 {
			s.config.TryExchangeMembershipPeriod = 10
		}
		periodTick := 0
		for !s.toDie() {
			members := s.memberMap.GetMembers()
			currentInterval := time.Duration(s.awareness.GetHealthScore()+1) * baseInterval
			if len(members) < 1 {
				time.Sleep(currentInterval)
				continue
			}

			for _, m := range members {
				if s.toDie(){
					return
				}
				currentInterval := time.Duration(s.awareness.GetHealthScore()+1) * baseInterval
				iLogger.Info(nil, "[swim] start probing ..."+s.member.ID.ID+"->"+m.ID.ID)
				s.probe(m, currentInterval)
				periodTick++
				if periodTick >= s.config.TryExchangeMembershipPeriod {
					go s.exchangeMembership(m)
					periodTick = 0
				}
				iLogger.Info(nil, "[swim] done probing !"+s.member.ID.ID+"->"+m.ID.ID)

			}

			// Reset memberMap.
			s.memberMap.Reset()
		}
	}()

	<-s.quitFD
}

// probe function
//
// 1. ExchangeMessage ping to the member(j) during the ack-timeout (time less than T).
//    Return if ack message arrives on ack-timeout.
//
// 2. selects K number of members from the memberMap and sends indirect-ping(request K members to ping the member(j)).
//    The nodes (that receive the indirect-ping) ping to the member(j) and ack when they receive ack from the member(j).
//
// 3. At the end of T, SWIM checks to see if ack was received from K members, and if there is no message,
//    The member(j) is judged to be failed, so check the member(j) as suspected or delete the member(j) from memberMap.
//
func (s *SWIM) probe(member Member, curInterval time.Duration) {

	if member.Status == Dead {
		return
	}

	end := make(chan ProbeResponse, 1)
	defer func() {
		close(end)
	}()

	// this context and wait group are used to cancel probe procedure
	// when probing time is out
	wg := sync.WaitGroup{}
	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())

	timer := time.NewTimer(curInterval)
	defer timer.Stop()

	go func() {
		task := func() (interface{}, error) {
			defer wg.Done()

			err := s.ping(&member)
			if err == nil {
				return nil, nil
			}

			if err != ErrSendTimeout {
				return nil, ErrPingFailed
			}
			iLogger.Error(nil,"error ping :"+s.member.ID.ID+"->"+member.ID.ID)
			err = s.indirectProbe(&member)
			if err != nil {
				return nil, err
			}

			return nil, nil
		}
		resp := NewTaskRunner(task, ctx).Start()
		SafeProbeResponseSend(end,ProbeResponse{err: resp.Err})
	}()

	select {
	// if timed-out, then suspect member
	case <-timer.C:
		iLogger.Infof(nil, "[SWIM] probe member [%s] timed out, start suspect", member.ID)
		// when probing time is out, then cancel the probing procedure
		cancel()
		wg.Wait()
		s.awareness.ApplyDelta(1)
		s.suspect(&member, curInterval)

		// if probe ended with error then suspect member and increase Awareness
		// otherwise just decrease Awareness score
	case resp := <-end:
		if !resp.Ok() {
			s.awareness.ApplyDelta(1)
			s.suspect(&member, curInterval)
			return
		}
		<-timer.C
		s.awareness.ApplyDelta(-1)
	}
}

// indirectProbe select K-random member from MemberMap, sends
// indirect-ping to them. if one of them sends back Ack message
// then indirectProbe success, otherwise failed.
// if one of K-member successfully received ACK message, then cancel
// K-1 member's probe
func (s *SWIM) indirectProbe(target *Member) error {
	kMembers := s.memberMap.SelectKRandomMemberID(s.config.K, &target.ID)

	indirectProbeNum := s.config.K
	if len(kMembers) < s.config.K {
		indirectProbeNum = len(kMembers)
	}

	if indirectProbeNum == 0 {
		return errors.New("empty indirect probe member")
	}

	wg := &sync.WaitGroup{}
	wg.Add(indirectProbeNum)

	// with cancel we can send the signal to goroutines which share
	// this @ctx context
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan IndProbeResponse, indirectProbeNum)

	defer func() {
		cancel()
		close(done)
	}()

	for _, m := range kMembers {
		go func(mediator Member) {
			defer wg.Done()

			task := func() (interface{}, error) {
				return s.indirectPing(mediator, *target)
			}

			resp := NewTaskRunner(task, ctx).Start()
			if resp.Err != nil {
				iLogger.Error(nil,"error indirect ping :"+s.member.ID.ID+"->"+m.ID.ID)
				SafeIndProbeResponseSend(done,IndProbeResponse{
					err: resp.Err,
					msg: pb.Message{},
				})
				return
			}

			msg, ok := resp.Payload.(pb.Message)
			if !ok {
				SafeIndProbeResponseSend(done,IndProbeResponse{
					err: ErrInvalidPayloadType,
					msg: pb.Message{},
				})
				return
			}
			SafeIndProbeResponseSend(done,IndProbeResponse{
				err: nil,
				msg: msg,
			})
		}(m)
	}

	// wait until K-random member sends back response, if response message
	// is Ack message, then indirectProbe success because one of K-member
	// success UDP communication, if Nack message or Invalid message, increase
	// @unexpectedRespCounter then wait other member's response

	unexpectedResp := make([]IndProbeResponse, 0)

	for {
		resp := <-done

		if !resp.Ok() {
			unexpectedResp = append(unexpectedResp, resp)
			if len(unexpectedResp) >= indirectProbeNum {
				iLogger.Infof(nil, "unexpected responses [%v]", unexpectedResp)
				return ErrIndProbeFailed
			}
			continue
		}

		cancel()
		wg.Wait()
		return nil
	}
}

// ping ping to member with piggyback message after sending ping message
// the result can be:
// 1. timeout
//    in this case, push signal to start indirect-ping request to K random nodes
// 2. successfully probed
//    in the case of successfully probe target node, update member state with
//    piggyback message sent from target member.
func (s *SWIM) ping(target *Member) error {
	var stats *pb.MbrStatsMsg = nil
	if !s.mbrStatsMsgStore.IsEmpty() {
		data, err := s.mbrStatsMsgStore.Get()
		stats = &data
		if err != nil {
			iLogger.Error(nil, err.Error())
			stats = nil
		}

	}

	// send ping message
	addr := target.UDPAddress()
	pingId := xid.New().String()
	ping := createPingMessage(pingId, s.member.UDPAddress(), stats)

	res, err := s.messageEndpoint.SyncSend(addr, ping)
	if err != nil {
		return err
	}

	// update piggyback data to store
	if res.PiggyBack != nil {
		s.handlePbk(res.PiggyBack)
	}

	return nil
}

// indirectPing sends indirect-ping to @member targeting @target member
// ** only when @member sends back to local node, push Message to channel
// otherwise just return **
// @ctx is for sending cancel signal from outside, when one of K-member successfully
// received ACK message or when in the exceptional situation
func (s *SWIM) indirectPing(mediator, target Member) (pb.Message, error) {
	var stats *pb.MbrStatsMsg = nil
	if !s.mbrStatsMsgStore.IsEmpty() {
		data, err := s.mbrStatsMsgStore.Get()
		stats = &data
		if err != nil {
			iLogger.Error(nil, err.Error())
			stats = nil
		}

	}

	// send indirect-ping message
	addr := mediator.UDPAddress()
	indId := xid.New().String()
	ind := createIndMessage(indId, s.member.UDPAddress(), target.UDPAddress(), stats)

	res, err := s.messageEndpoint.SyncSend(addr, ind)

	// when communicating member and target with indirect-ping failed,
	// just return.
	if err != nil {
		return pb.Message{}, err
	}
	// update piggyback data to store
	if res.PiggyBack != nil {
		s.handlePbk(res.PiggyBack)
	}

	return res, nil
}

func (s *SWIM) suspect(member *Member, curProveInterval time.Duration) {
	msg := createSuspectMessage(member, s.member.ID.ID)

	result, err := s.memberMap.Suspect(msg, curProveInterval)
	if err != nil {
		iLogger.Error(nil, err.Error())
	}

	iLogger.Infof(nil, "Result of suspect [%t]", result)
}

// handler interface to handle received message
type MessageHandler interface {
	handle(msg pb.Message)
}

// The handle function does two things.
//
// 1. Update the member map using the piggyback-data contained in the message.
//  1-1. Check if member is me or not.
// 	1-2. Change status of member.
// 	1-3. If the state of the member map changes, store new status in the piggyback store.
//
// 2. Process Ping, Ack, Indirect-ping messages.
//
func (s *SWIM) handle(msg pb.Message) {
	if msg.PiggyBack != nil {
		s.handlePbk(msg.PiggyBack)
	}

	switch msg.Payload.(type) {
	case *pb.Message_Ping:
		s.handlePing(msg)
	case *pb.Message_Ack:
		// handle ack
	case *pb.Message_IndirectPing:
		s.handleIndirectPing(msg)
	case *pb.Message_Membership:
		s.handleMembership(msg)
	default:

	}
}

// handle piggyback related to member status
func (s *SWIM) handlePbk(piggyBack *pb.PiggyBack) {
	if piggyBack.MbrStatsMsg == nil {
		return
	}
	mbrStatsMsg := piggyBack.MbrStatsMsg
	// Check if piggyback message changes memberMap.
	s.handleMbrStatsMsg(mbrStatsMsg)
}

// handlePing send back Ack message by response
func (s *SWIM) handlePing(msg pb.Message) {
	id := msg.Id

	var stats *pb.MbrStatsMsg = nil
	if !s.mbrStatsMsgStore.IsEmpty() {
		data, err := s.mbrStatsMsgStore.Get()
		stats = &data
		if err != nil {
			iLogger.Error(nil, err.Error())
			stats = nil
		}

	}

	// address of messgae source member
	srcAddr := msg.Address

	ack := createAckMessage(id, srcAddr, stats)
	if err := s.messageEndpoint.Send(srcAddr, ack); err != nil {
		iLogger.Error(nil, err.Error())
	}
}

// handleIndirectPing receives IndirectPing message, so send the ping message
// to target member, if successfully receives ACK message then send back again
// ACK message to member who sent IndirectPing message to me.
// If ping was not successful then send back NACK message
func (s *SWIM) handleIndirectPing(msg pb.Message) {
	id := msg.Id

	// retrieve piggyback data from pbkStore
	var stats *pb.MbrStatsMsg = nil
	if !s.mbrStatsMsgStore.IsEmpty() {
		data, err := s.mbrStatsMsgStore.Get()
		stats = &data
		if err != nil {
			iLogger.Error(nil, err.Error())
			stats = nil
		}

	}

	// address of message source member
	srcAddr := msg.Address

	// address of indirect-ping's target
	targetAddr := msg.Payload.(*pb.Message_IndirectPing).IndirectPing.Target

	pingId := xid.New().String()
	ping := createPingMessage(pingId, s.member.UDPAddress(), stats)

	// first send the ping to target member, if target member could not send-back
	// ack message for whatever reason send nack message to source member,
	// if successfully received ack message from target, then send back ack message
	// to source member
	if _, err := s.messageEndpoint.SyncSend(targetAddr, ping); err != nil {
		//nack := createNackMessage(id, srcAddr, &mbrStatsMsg)
		//if Err := s.messageEndpoint.ExchangeMessage(srcAddr, nack); Err != nil {
		//			iLogger.Error(nil, Err.Error())
		//}
		return
	}

	stats = nil
	if !s.mbrStatsMsgStore.IsEmpty() {
		data, err := s.mbrStatsMsgStore.Get()
		stats = &data
		if err != nil {
			iLogger.Error(nil, err.Error())
			stats = nil
		}

	}
	ack := createAckMessage(id, srcAddr, stats)
	if err := s.messageEndpoint.Send(srcAddr, ack); err != nil {
		iLogger.Error(nil, err.Error())
	}
}

// handleMembership receives Membership message.
// create membership message with membermap and reply to the message with it.
func (s *SWIM) handleMembership(msg pb.Message) {
	payload := msg.Payload.(*pb.Message_Membership)
	membership := payload.Membership
	// add member if sender is not in membership
	if !s.memberMap.IsMember(MemberID{ID: payload.Membership.SenderId,}) {
		stats := &pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          payload.Membership.SenderId,
			Incarnation: uint32(0),
			UdpAddress:  payload.Membership.SenderUdpAddress,
			TcpAddress:  payload.Membership.SenderTcpAddress,
		}
		s.handleMbrStatsMsg(stats)

	}


	for _, m := range membership.MbrStatsMsgs {
		if m.UdpAddress == s.member.UDPAddress() || m.Id == s.member.ID.ID {
			continue
		}
		s.handleMbrStatsMsg(m)
	}

}

func (s *SWIM) GetMyInfo() *Member {
	return s.member
}

func createPingMessage(id, src string, mbrStatsMsg *pb.MbrStatsMsg) pb.Message {
	return pb.Message{
		Id:      id,
		Address: src,
		Payload: &pb.Message_Ping{
			Ping: &pb.Ping{},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: mbrStatsMsg,
		},
	}
}

func createIndMessage(id, src, target string, mbrStatsMsg *pb.MbrStatsMsg) pb.Message {
	return pb.Message{
		Id:      id,
		Address: src,
		Payload: &pb.Message_IndirectPing{
			IndirectPing: &pb.IndirectPing{
				Target: target,
			},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: mbrStatsMsg,
		},
	}
}

func createNackMessage(id, src string, mbrStatsMsg *pb.MbrStatsMsg) pb.Message {
	return pb.Message{
		Id:      id,
		Address: src,
		Payload: &pb.Message_Nack{
			Nack: &pb.Nack{},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: mbrStatsMsg,
		},
	}
}

func createAckMessage(id, src string, mbrStatsMsg *pb.MbrStatsMsg) pb.Message {
	return pb.Message{
		Id:      id,
		Address: src,
		Payload: &pb.Message_Ack{
			Ack: &pb.Ack{},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: mbrStatsMsg,
		},
	}
}

func createSuspectMessage(suspect *Member, confirmer string) SuspectMessage {
	return SuspectMessage{
		MemberMessage: MemberMessage{
			ID:          suspect.ID.ID,
			Addr:        suspect.Addr,
			UDPPort:     suspect.UDPPort,
			TCPPort:     suspect.TCPPort,
			Incarnation: suspect.Incarnation,
		},
		ConfirmerID: confirmer,
	}
}
