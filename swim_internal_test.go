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
	"net"
	"sync"
	"testing"
	"time"

	"github.com/rs/xid"

	"github.com/DE-labtory/swim/pb"
	"github.com/stretchr/testify/assert"
)

type MockMbrStatsMsgStore struct {
	LenFunc     func() int
	PushFunc    func(mbrStatsMsg pb.MbrStatsMsg)
	GetFunc     func() (pb.MbrStatsMsg, error)
	IsEmptyFunc func() bool
}

func (p MockMbrStatsMsgStore) Len() int {
	return p.LenFunc()
}
func (p MockMbrStatsMsgStore) Push(mbrStatsMsg pb.MbrStatsMsg) {
	p.PushFunc(mbrStatsMsg)
}
func (p MockMbrStatsMsgStore) Get() (pb.MbrStatsMsg, error) {
	return p.GetFunc()
}
func (p MockMbrStatsMsgStore) IsEmpty() bool {
	return p.IsEmptyFunc()
}

func TestSWIM_ShutDown(t *testing.T) {
	s := New(&Config{
		K:             2,
		T:             4000,
		AckTimeOut:    1000,
		MaxlocalCount: 1,
		MaxNsaCounter: 8,
		BindAddress:   "127.0.0.1",
		BindPort:      3000,
	},
		&SuspicionConfig{},
		MessageEndpointConfig{
			CallbackCollectInterval: 1000,
		},
		TCPMessageEndpointConfig{
			EncryptionEnabled: false,
			TCPTimeout:        0,
			IP:                "127.0.0.1",
			Port:              3000,
		},
		&Member{},
	)

	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{}, nil
	}
	s.mbrStatsMsgStore = pbkStore

	m := NewMemberMap(&SuspicionConfig{})
	m.Alive(AliveMessage{
		MemberMessage: MemberMessage{
			ID: "1",
		},
	})

	m.Alive(AliveMessage{
		MemberMessage: MemberMessage{
			ID: "2",
		},
	})

	s.memberMap = m

	wg := sync.WaitGroup{}
	wg.Add(1)

	// Check whether the startFailureDetector ends or not
	go func() {
		s.Start()
		wg.Done()
	}()

	time.Sleep(5 * time.Second)

	// End startFailureDetector
	s.ShutDown()

	wg.Wait()
}

// When you receive message that you are suspected, you refute
// inc always sets the standard in small inc
// TEST : When your inc is bigger than(or same) received MbrStatsMsg data.
func TestSWIM_handlembrStatsMsg_refute_bigger(t *testing.T) {
	heapSize := 1
	q := setUpHeap(heapSize)

	mbrStatsMsgStore := MockMbrStatsMsgStore{}
	mbrStatsMsgStore.PushFunc = func(mbrStatsMsg pb.MbrStatsMsg) {
		item := Item{
			value:    mbrStatsMsg,
			priority: 1,
		}
		q.Push(&item)
	}

	mbrStatsMsgStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return q.Pop().(*Item).value.(pb.MbrStatsMsg), nil
	}

	swim := SWIM{}
	mI := createMessageEndpoint(t, &swim, time.Second, 11150)

	swim.awareness = NewAwareness(8)
	swim.messageEndpoint = mI
	swim.mbrStatsMsgStore = mbrStatsMsgStore
	swim.member = &Member{
		ID:          MemberID{"abcde"},
		Incarnation: 5,
	}

	msg := pb.Message{
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: &pb.MbrStatsMsg{
				Id:          "abcde",
				Incarnation: 5,
				UdpAddress:     "127.0.0.1:11151",
			},
		},
	}

	swim.handleMbrStatsMsg(msg.PiggyBack.MbrStatsMsg)

	assert.Equal(t, swim.member.Incarnation, uint32(6))

	mbrStatsMsg, err := swim.mbrStatsMsgStore.Get()
	assert.NoError(t, err)
	assert.Equal(t, &mbrStatsMsg, msg.PiggyBack.MbrStatsMsg)
}

// When you receive message that you are suspected, you refute
// inc always sets the standard in small inc
// TEST : When your inc is less than your MbrStatsMsg data.
func TestSWIM_handlembrStatsMsg_refute_less(t *testing.T) {
	heapSize := 1
	q := setUpHeap(heapSize)

	mbrStatsMsgStore := MockMbrStatsMsgStore{}
	mbrStatsMsgStore.PushFunc = func(mbrStatsMsg pb.MbrStatsMsg) {
		item := Item{
			value:    mbrStatsMsg,
			priority: 1,
		}
		q.Push(&item)
	}

	mbrStatsMsgStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return q.Pop().(*Item).value.(pb.MbrStatsMsg), nil
	}

	swim := SWIM{}
	mI := createMessageEndpoint(t, &swim, time.Second, 11152)

	swim.awareness = NewAwareness(8)
	swim.messageEndpoint = mI
	swim.mbrStatsMsgStore = mbrStatsMsgStore
	swim.member = &Member{
		ID:          MemberID{"abcde"},
		Incarnation: 3,
	}

	msg := pb.Message{
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: &pb.MbrStatsMsg{
				Id:          "abcde",
				Incarnation: 5,
				UdpAddress:     "127.0.0.1:11153",
			},
		},
	}

	swim.handleMbrStatsMsg(msg.PiggyBack.MbrStatsMsg)

	assert.Equal(t, swim.member.Incarnation, uint32(4))

	mbrStatsMsg, err := swim.mbrStatsMsgStore.Get()
	assert.NoError(t, err)
	assert.Equal(t, &mbrStatsMsg, msg.PiggyBack.MbrStatsMsg)
}

func TestSWIM_HandleMbrStatsMsg_AliveMsg(t *testing.T) {
	// setup MbrStatsMsgStore
	var pushedMbrStatsMsg pb.MbrStatsMsg

	mbrStatsMsgStore := &MockMbrStatsMsgStore{}
	mbrStatsMsgStore.PushFunc = func(mbrStatsMsg pb.MbrStatsMsg) {
		pushedMbrStatsMsg = mbrStatsMsg
	}

	// setup member_map
	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[MemberID{ID: "1"}] = &Member{
		ID:     MemberID{ID: "1"},
		Status: Dead,
	}

	// setup SWIM
	swim := &SWIM{
		member: &Member{
			ID: MemberID{ID: "123"},
		},
		memberMap:        mm,
		mbrStatsMsgStore: mbrStatsMsgStore,
	}

	// given
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Alive,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	// when
	swim.handleMbrStatsMsg(stats)

	// then
	assert.Equal(t, pushedMbrStatsMsg.Type, stats.Type)
	assert.Equal(t, pushedMbrStatsMsg.Id, stats.Id)
	assert.Equal(t, pushedMbrStatsMsg.Incarnation, stats.Incarnation)
	assert.Equal(t, pushedMbrStatsMsg.UdpAddress, stats.UdpAddress)

	assert.Equal(t, mm.members[MemberID{ID: "1"}].Status, Alive)
	assert.Equal(t, mm.members[MemberID{ID: "1"}].Incarnation, stats.Incarnation)
}

func TestSWIM_HandleMbrStatsMsg_AliveMsg_Error(t *testing.T) {
	// setup MbrStatsMsgStore
	var pushedMbrStatsMsg pb.MbrStatsMsg

	mbrStatsMsgStore := &MockMbrStatsMsgStore{}
	mbrStatsMsgStore.PushFunc = func(mbrStatsMsg pb.MbrStatsMsg) {
		pushedMbrStatsMsg = mbrStatsMsg
	}

	// setup member_map
	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[MemberID{ID: "1"}] = &Member{
		ID:          MemberID{ID: "1"},
		Status:      Dead,
		Incarnation: uint32(1),
	}

	// setup SWIM
	swim := &SWIM{
		member: &Member{
			ID: MemberID{ID: "123"},
		},
		memberMap:        mm,
		mbrStatsMsgStore: mbrStatsMsgStore,
	}

	// given
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Alive,
		Id:          "1",
		Incarnation: uint32(2),
		// error occured becuase of invalid address format
		UdpAddress: "1.2.3.4.555:5555",
	}

	// when
	swim.handleMbrStatsMsg(stats)

	// then
	assert.Equal(t, pushedMbrStatsMsg, pb.MbrStatsMsg{})

	assert.Equal(t, mm.members[MemberID{ID: "1"}].Status, Dead)
	assert.Equal(t, mm.members[MemberID{ID: "1"}].Incarnation, uint32(1))
}

func TestSWIM_HandleMbrStatsMsg_SuspectMsg(t *testing.T) {
	// setup MbrStatsMsgStore
	var pushedMbrStatsMsg pb.MbrStatsMsg

	mbrStatsMsgStore := &MockMbrStatsMsgStore{}
	mbrStatsMsgStore.PushFunc = func(mbrStatsMsg pb.MbrStatsMsg) {
		pushedMbrStatsMsg = mbrStatsMsg
	}

	// setup member_map
	mm := NewMemberMap(&SuspicionConfig{
		K:        3,
		MinParam: 5,
		MaxParam: 1,
	})
	mm.members[MemberID{ID: "1"}] = &Member{
		ID:     MemberID{ID: "1"},
		Status: Alive,
	}

	// setup SWIM
	swim := &SWIM{
		member: &Member{
			ID: MemberID{ID: "123"},
		},
		memberMap:        mm,
		mbrStatsMsgStore: mbrStatsMsgStore,
		awareness:        NewAwareness(5),
		config: &Config{
			T:          4000,
			AckTimeOut: 3000,
		},
	}

	// given
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Suspect,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	// when
	swim.handleMbrStatsMsg(stats)
	//&{{1} <nil> 0 2 2019-04-23 16:41:59.464422 +0900 KST m=+0.002258696 2 0xc0000c04b0}
	//&{{1} <nil> 0 2 2019-04-23 16:42:20.00995 +0900 KST m=+0.004020539 2 0xc0000c24b0}
	// then
	assert.Equal(t, pushedMbrStatsMsg.Type, stats.Type)
	assert.Equal(t, pushedMbrStatsMsg.Id, stats.Id)
	assert.Equal(t, pushedMbrStatsMsg.Incarnation, stats.Incarnation)
	assert.Equal(t, pushedMbrStatsMsg.UdpAddress, stats.UdpAddress)
	assert.Equal(t, mm.members[MemberID{ID: "1"}].Status, Suspected)
	assert.Equal(t, mm.members[MemberID{ID: "1"}].Incarnation, stats.Incarnation)
}

func TestSWIM_HandleMbrStatsMsg_SuspectMsg_Error(t *testing.T) {
	// setup MbrStatsMsgStore
	var pushedMbrStatsMsg pb.MbrStatsMsg

	mbrStatsMsgStore := &MockMbrStatsMsgStore{}
	mbrStatsMsgStore.PushFunc = func(mbrStatsMsg pb.MbrStatsMsg) {
		pushedMbrStatsMsg = mbrStatsMsg
	}

	// setup member_map
	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[MemberID{ID: "1"}] = &Member{
		ID:          MemberID{ID: "1"},
		Status:      Alive,
		Incarnation: uint32(1),
	}

	// setup SWIM
	swim := &SWIM{
		member: &Member{
			ID: MemberID{ID: "123"},
		},
		memberMap:        mm,
		mbrStatsMsgStore: mbrStatsMsgStore,
	}

	// given
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Suspect,
		Id:          "1",
		Incarnation: uint32(2),
		// error occured becuase of invalid address format
		UdpAddress: "1.2.3.4.555:5555",
	}

	// when
	swim.handleMbrStatsMsg(stats)

	// then
	assert.Equal(t, pushedMbrStatsMsg, pb.MbrStatsMsg{})

	assert.Equal(t, mm.members[MemberID{ID: "1"}].Status, Alive)
	assert.Equal(t, mm.members[MemberID{ID: "1"}].Incarnation, uint32(1))
}

func TestSWIM_handlePing(t *testing.T) {
	id := "1"

	mbrStatsMsgStore := MockMbrStatsMsgStore{}
	mbrStatsMsgStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          "mbrStatsMsg_id1",
			Incarnation: 123,
			UdpAddress:     "address123",
		}, nil
	}

	mIMessageHandler := MockMessageHandler{}

	//mJ
	swim := SWIM{}

	mI := createMessageEndpoint(t, &mIMessageHandler, time.Second, 11143)
	mJ := createMessageEndpoint(t, &swim, time.Second, 11144)

	swim.messageEndpoint = mJ
	swim.mbrStatsMsgStore = mbrStatsMsgStore
	swim.member = &Member{
		ID:          MemberID{"abcde2"},
		Incarnation: 3,
	}

	go mI.Listen()
	go mJ.Listen()

	ping := pb.Message{
		Id: id,
		// address of source member
		Address: "127.0.0.1:11143",
		Payload: &pb.Message_Ping{
			Ping: &pb.Ping{},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: &pb.MbrStatsMsg{},
		},
	}

	defer func() {
		mI.Shutdown()
		mJ.Shutdown()
	}()

	resp, err := mI.SyncSend("127.0.0.1:11144", ping)
	assert.NoError(t, err)
	assert.NotNil(t, resp.Payload.(*pb.Message_Ack))
	assert.Equal(t, resp.Id, id)
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.UdpAddress, "address123")
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.Incarnation, uint32(123))
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.Id, "mbrStatsMsg_id1")
}

// This is successful scenario when target member response in time with
// ack message, then mediator sends back ack message to source member
func TestSWIM_handleIndirectPing(t *testing.T) {
	id := "1"

	mbrStatsMsgStore := MockMbrStatsMsgStore{}
	mbrStatsMsgStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          "mbrStatsMsg_id1",
			Incarnation: 123,
			UdpAddress:     "address123",
		}, nil
	}

	self := &Member{
		ID:      MemberID{ID: "mi"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: uint16(11146),
	}
	config := &Config{
		BindAddress: "127.0.0.1",
		BindPort:    11146,
	}
	swim := SWIM{}
	swim.member = self
	swim.config = config

	mIMessageHandler := MockMessageHandler{}
	mJMessageHandler := MockMessageHandler{}

	mK := createMessageEndpoint(t, &swim, time.Second, 11145)
	mI := createMessageEndpoint(t, &mIMessageHandler, time.Second, 11146)
	mJ := createMessageEndpoint(t, &mJMessageHandler, time.Second, 11147)

	swim.messageEndpoint = mK
	swim.mbrStatsMsgStore = mbrStatsMsgStore
	swim.member = &Member{
		ID:          MemberID{"abcde"},
		Incarnation: 3,
	}

	// ** m_k's handleIndirectPing is implicitly called when m_k received
	// indirect-ping message from other member **
	go mK.Listen()
	go mI.Listen()
	go mJ.Listen()

	ind := pb.Message{
		Id: id,
		// address of source member
		Address: "127.0.0.1:11146",
		Payload: &pb.Message_IndirectPing{
			IndirectPing: &pb.IndirectPing{
				// target address
				Target: "127.0.0.1:11147",
			},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: &pb.MbrStatsMsg{},
		},
	}

	defer func() {
		mK.Shutdown()
		mI.Shutdown()
		mJ.Shutdown()
	}()

	mJMessageHandler.handleFunc = func(msg pb.Message) {
		// check whether msg.Payload type is *pb.Message_Ping
		assert.NotNil(t, msg.Payload.(*pb.Message_Ping))
		assert.Equal(t, msg.PiggyBack.MbrStatsMsg.UdpAddress, "address123")
		assert.Equal(t, msg.PiggyBack.MbrStatsMsg.Incarnation, uint32(123))
		assert.Equal(t, msg.PiggyBack.MbrStatsMsg.Id, "mbrStatsMsg_id1")

		ack := pb.Message{Id: msg.Id, Payload: &pb.Message_Ack{Ack: &pb.Ack{}}, PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: &pb.MbrStatsMsg{},
		}}
		mJ.Send("127.0.0.1:11145", ack)
	}

	resp, err := mI.SyncSend("127.0.0.1:11145", ind)
	assert.NoError(t, err)
	assert.NotNil(t, resp.Payload.(*pb.Message_Ack))
	assert.Equal(t, resp.Id, id)
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.UdpAddress, "address123")
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.Incarnation, uint32(123))
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.Id, "mbrStatsMsg_id1")
}

// This is NOT-successful scenario when target member DID NOT response in time
// ack message, then mediator sends back NACK message to source member
func TestSWIM_handleIndirectPing_Target_Timeout(t *testing.T) {
	id := "1"

	mbrStatsMsgStore := MockMbrStatsMsgStore{}
	mbrStatsMsgStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          "mbrStatsMsg_id1",
			Incarnation: 123,
			UdpAddress:     "address123",
		}, nil
	}

	self := &Member{
		ID:      MemberID{ID: "mi"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: uint16(11179),
	}
	config := &Config{
		BindAddress: "127.0.0.1",
		BindPort:    11179,
	}

	swim := SWIM{}
	swim.member = self
	swim.config = config
	mIMessageHandler := MockMessageHandler{}
	mJMessageHandler := MockMessageHandler{}

	mK := createMessageEndpoint(t, &swim, time.Second, 11178)
	// source should have larger send timeout, because source should give mediator
	// enough time to ping to target
	mI := createMessageEndpoint(t, &mIMessageHandler, time.Second*3, 11179)
	mJ := createMessageEndpoint(t, &mJMessageHandler, time.Second, 11180)

	swim.messageEndpoint = mK
	swim.mbrStatsMsgStore = mbrStatsMsgStore
	swim.member = &Member{
		ID:          MemberID{"abcde"},
		Incarnation: 3,
	}

	// ** m_k's handleIndirectPing is implicitly called when m_k received
	// indirect-ping message from other member **
	go mK.Listen()

	go mI.Listen()
	go mJ.Listen()

	ind := pb.Message{
		Id: id,
		// address of source member
		Address: "127.0.0.1:11179",
		Payload: &pb.Message_IndirectPing{
			IndirectPing: &pb.IndirectPing{
				// target address
				Target: "127.0.0.1:11180",
			},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: &pb.MbrStatsMsg{},
		},
	}

	defer func() {
		mI.Shutdown()
		mK.Shutdown()
		mJ.Shutdown()
	}()

	mJMessageHandler.handleFunc = func(msg pb.Message) {
		// check whether msg.Payload type is *pb.Message_Ping
		assert.NotNil(t, msg.Payload.(*pb.Message_Ping))
		assert.Equal(t, msg.PiggyBack.MbrStatsMsg.UdpAddress, "address123")
		assert.Equal(t, msg.PiggyBack.MbrStatsMsg.Incarnation, uint32(123))
		assert.Equal(t, msg.PiggyBack.MbrStatsMsg.Id, "mbrStatsMsg_id1")

		// DO NOT ANYTHING: do not response back to m
	}

	resp, err := mI.SyncSend("127.0.0.1:11178", ind)
	assert.NoError(t, err)
	assert.NotNil(t, resp.Payload.(*pb.Message_Nack))
	assert.Equal(t, resp.Id, id)
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.UdpAddress, "address123")
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.Incarnation, uint32(123))
	assert.Equal(t, resp.PiggyBack.MbrStatsMsg.Id, "mbrStatsMsg_id1")
}

func TestConvMbrStatsToAliveMsg_InvalidMsgType(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Suspect,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	swim := &SWIM{}
	// when
	msg, err := swim.convMbrStatsToAliveMsg(stats)

	// then
	assert.Equal(t, msg, AliveMessage{})
	assert.Equal(t, err, ErrInvalidMbrStatsMsgType)
}

func TestConvMbrStatsToAliveMsg_InvalidAddressFormat(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Suspect,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4.5:5555",
	}

	swim := &SWIM{}
	// when
	msg, err := swim.convMbrStatsToAliveMsg(stats)

	// then
	assert.Equal(t, msg, AliveMessage{})
	assert.Error(t, err)
}

func TestConvMbrStatsToAliveMsg_Success(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Alive,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	swim := &SWIM{}
	// when
	msg, err := swim.convMbrStatsToAliveMsg(stats)

	// then
	assert.Equal(t, msg.ID, stats.Id)
	assert.Equal(t, msg.Incarnation, stats.Incarnation)
	assert.Equal(t, msg.Addr, net.ParseIP("1.2.3.4"))
	assert.Equal(t, msg.UDPPort, uint16(5555))
	assert.NoError(t, err)
}

func TestConvMbrStatsToSuspectMsg_InvalidMsgType(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Alive,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	swim := &SWIM{}
	// when
	msg, err := swim.convMbrStatsToSuspectMsg(stats)

	// then
	assert.Equal(t, msg, SuspectMessage{})
	assert.Equal(t, err, ErrInvalidMbrStatsMsgType)
}

func TestConvMbrStatsToSuspectMsg_InvalidAddressFormat(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Alive,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4.555:5555",
	}

	swim := &SWIM{}
	// when
	msg, err := swim.convMbrStatsToSuspectMsg(stats)

	// then
	assert.Equal(t, msg, SuspectMessage{})
	assert.Error(t, err)
}

func TestConvMbrStatsToSuspectMsg_Success(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Suspect,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	swim := &SWIM{
		member: &Member{
			ID: MemberID{ID: "123"},
		},
	}
	// when
	msg, err := swim.convMbrStatsToSuspectMsg(stats)

	// then
	assert.Equal(t, msg.ID, stats.Id)
	assert.Equal(t, msg.Incarnation, stats.Incarnation)
	assert.Equal(t, msg.Addr, net.ParseIP("1.2.3.4"))
	assert.Equal(t, msg.UDPPort, uint16(5555))
	assert.Equal(t, msg.ConfirmerID, "123")
	assert.NoError(t, err)
}

func TestHandleAliveMbrStats_InvalidMsgType(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Suspect,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	swim := &SWIM{}
	// when
	result, err := swim.handleAliveMbrStatsMsg(stats)

	// then
	assert.Equal(t, result, false)
	assert.Equal(t, err, ErrInvalidMbrStatsMsgType)
}

func TestHandleAliveMbrStats_Success(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Alive,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[MemberID{ID: "1"}] = &Member{
		ID:     MemberID{ID: "1"},
		Status: Dead,
	}

	swim := &SWIM{
		memberMap: mm,
	}

	// when
	result, err := swim.handleAliveMbrStatsMsg(stats)

	// then
	assert.Equal(t, result, true)
	assert.NoError(t, err)
}

func TestHandleSuspectMbrStats_InvalidMsgType(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Alive,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	swim := &SWIM{}
	// when
	result, err := swim.handleSuspectMbrStatsMsg(stats)

	// then
	assert.Equal(t, result, false)
	assert.Equal(t, err, ErrInvalidMbrStatsMsgType)
}

func TestHandleSuspectMbrStats_Success(t *testing.T) {
	stats := &pb.MbrStatsMsg{
		Type:        pb.MbrStatsMsg_Suspect,
		Id:          "1",
		Incarnation: uint32(2),
		UdpAddress:     "1.2.3.4:5555",
	}

	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[MemberID{ID: "1"}] = &Member{
		ID:          MemberID{ID: "1"},
		Incarnation: uint32(1),
		Status:      Alive,
	}

	swim := &SWIM{
		member: &Member{
			ID: MemberID{ID: "123"},
		},
		memberMap: mm,
		awareness: NewAwareness(5),
		config: &Config{
			T: 5000,
			K: 3,
		},
	}

	// when
	result, err := swim.handleSuspectMbrStatsMsg(stats)

	// then
	assert.Equal(t, true, result)
	assert.NoError(t, err)
}

func TestSWIM_indirectPing_When_Response_Success(t *testing.T) {
	mKAddr := "1.2.3.4:11111"
	mJAddr := "3.4.5.6:22222"

	pbk := pb.PiggyBack{
		MbrStatsMsg: &pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          "123",
			Incarnation: uint32(123),
			UdpAddress:     "pbk-addr1",
		},
	}
	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.PushFunc = func(pbk pb.MbrStatsMsg) {}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return *pbk.MbrStatsMsg, nil
	}

	ack := pb.Message{
		Id:      "responseID",
		Address: mJAddr,
		Payload: &pb.Message_Ack{
			Ack: &pb.Ack{
				Payload: "ack-Payload",
			},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: &pb.MbrStatsMsg{
				Type:        pb.MbrStatsMsg_Alive,
				Id:          "555",
				Incarnation: uint32(345),
				UdpAddress:     "pbk-addr2",
			},
		},
	}
	messageEndpoint := MockMessageEndpoint{}
	messageEndpoint.SyncSendFunc = func(addr string, msg pb.Message) (pb.Message, error) {
		// this addr should be mediator address
		assert.Equal(t, addr, mKAddr)

		// msg.Address should be local-node address
		assert.Equal(t, msg.Address, "9.8.7.6:33333")
		assert.Equal(t, msg.PiggyBack.MbrStatsMsg, pbk.MbrStatsMsg)
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing.Target, "3.4.5.6:22222")

		return ack, nil
	}

	mIMember := &Member{
		ID:      MemberID{ID: "mi"},
		Addr:    net.ParseIP("9.8.7.6"),
		UDPPort: uint16(33333),
	}
	mKMember := &Member{
		ID:      MemberID{ID: "mk"},
		Addr:    net.ParseIP("1.2.3.4"),
		UDPPort: uint16(11111),
	}
	mJMember := &Member{
		ID:      MemberID{ID: "mj"},
		Addr:    net.ParseIP("3.4.5.6"),
		UDPPort: uint16(22222),
	}

	indSucc := make(chan pb.Message)
	defer func() {
		close(indSucc)
	}()

	config := &Config{
		BindAddress: "9.8.7.6",
		BindPort:    33333,
	}

	swim := &SWIM{}
	swim.member = mIMember
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.messageEndpoint = messageEndpoint

	swim.indirectPing(*mKMember, *mJMember)
}

func TestSWIM_indirectPing_When_Response_Failed(t *testing.T) {
	mIAddr := "9.8.7.6:33333"
	mKAddr := "1.2.3.4:11111"

	pbk := pb.PiggyBack{
		MbrStatsMsg: &pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          "123",
			Incarnation: uint32(123),
			UdpAddress:     "pbk-addr1",
		},
	}
	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.PushFunc = func(pbk pb.MbrStatsMsg) {}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return *pbk.MbrStatsMsg, nil
	}

	messageEndpoint := MockMessageEndpoint{}
	messageEndpoint.SyncSendFunc = func(addr string, msg pb.Message) (pb.Message, error) {
		// this addr should be mediator address
		assert.Equal(t, addr, mKAddr)

		// msg.Address should be local-node address
		assert.Equal(t, msg.Address, mIAddr)
		assert.Equal(t, msg.PiggyBack, &pbk)
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing.Target, "3.4.5.6:22222")

		// in this case, send out Timeout error
		return pb.Message{}, ErrSendTimeout
	}

	mIMember := &Member{
		ID:      MemberID{ID: "memberID"},
		Addr:    net.ParseIP("9.8.7.6"),
		UDPPort: uint16(33333),
	}
	mKMember := &Member{
		ID:      MemberID{ID: "memberID"},
		Addr:    net.ParseIP("1.2.3.4"),
		UDPPort: uint16(11111),
	}
	mJMember := &Member{
		ID:      MemberID{ID: "targetID"},
		Addr:    net.ParseIP("3.4.5.6"),
		UDPPort: uint16(22222),
	}

	indSucc := make(chan pb.Message)
	defer func() {
		close(indSucc)
	}()

	config := &Config{
		BindAddress: "9.8.7.6",
		BindPort:    33333,
	}

	swim := &SWIM{}
	swim.member = mIMember
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.messageEndpoint = messageEndpoint

	swim.indirectPing(*mKMember, *mJMember)
}

func TestSWIM_indirectPing_When_One_Of_Other_Member_Sent_Ack(t *testing.T) {
	pbk := pb.PiggyBack{
		MbrStatsMsg: &pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          "123",
			Incarnation: uint32(123),
			UdpAddress:     "pbk-addr1",
		},
	}
	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.PushFunc = func(pbk pb.MbrStatsMsg) {}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return *pbk.MbrStatsMsg, nil
	}

	messageEndpoint := MockMessageEndpoint{}
	messageEndpoint.SyncSendFunc = func(addr string, msg pb.Message) (pb.Message, error) {
		// this addr should be mediator address
		assert.Equal(t, addr, "1.2.3.4:11111")

		// msg.Address should be local-node address
		assert.Equal(t, msg.Address, "9.8.7.6:33333")
		assert.Equal(t, msg.PiggyBack, &pbk)
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing.Target, "3.4.5.6:22222")

		// sleep 1 seconds
		time.Sleep(time.Second)

		return pb.Message{
			Id:      "responseID",
			Address: "3.4.5.6:22222",
			Payload: &pb.Message_Ack{
				Ack: &pb.Ack{
					Payload: "ack-Payload",
				},
			},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Type:        pb.MbrStatsMsg_Alive,
					Id:          "555",
					Incarnation: uint32(345),
					UdpAddress:     "pbk-addr2",
				},
			},
		}, nil
	}

	self := &Member{
		ID:      MemberID{ID: "memberID"},
		Addr:    net.ParseIP("9.8.7.6"),
		UDPPort: uint16(33333),
	}
	member := &Member{
		ID:      MemberID{ID: "memberID"},
		Addr:    net.ParseIP("1.2.3.4"),
		UDPPort: uint16(11111),
	}
	target := &Member{
		ID:      MemberID{ID: "targetID"},
		Addr:    net.ParseIP("3.4.5.6"),
		UDPPort: uint16(22222),
	}

	indSucc := make(chan pb.Message)
	defer func() {
		close(indSucc)
	}()

	config := &Config{
		BindAddress: "9.8.7.6",
		BindPort:    33333,
	}

	swim := &SWIM{}
	swim.member = self
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.messageEndpoint = messageEndpoint

	swim.indirectPing(*member, *target)
}

func TestSWIM_ping_When_Response_Success(t *testing.T) {
	pbk := pb.PiggyBack{
		MbrStatsMsg: &pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          "123",
			Incarnation: uint32(123),
			UdpAddress:     "pbk-addr1",
		},
	}
	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.PushFunc = func(pbk pb.MbrStatsMsg) {}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return *pbk.MbrStatsMsg, nil
	}

	respMsg := pb.Message{
		Id:      "responseID",
		Address: "3.4.5.6:22222",
		Payload: &pb.Message_Ack{
			Ack: &pb.Ack{
				Payload: "ack-Payload",
			},
		},
		PiggyBack: &pb.PiggyBack{
			MbrStatsMsg: &pb.MbrStatsMsg{
				Type:        pb.MbrStatsMsg_Alive,
				Id:          "555",
				Incarnation: uint32(345),
				UdpAddress:     "pbk-addr2",
			},
		},
	}
	messageEndpoint := MockMessageEndpoint{}
	messageEndpoint.SyncSendFunc = func(addr string, msg pb.Message) (pb.Message, error) {
		assert.Equal(t, addr, "1.2.3.4:11111")
		assert.Equal(t, msg.Address, "9.8.7.6:33333")
		assert.Equal(t, msg.PiggyBack, &pbk)
		assert.Equal(t, msg.Payload.(*pb.Message_Ping).Ping, &pb.Ping{})

		return respMsg, nil
	}

	self := &Member{
		ID:      MemberID{ID: "self"},
		Addr:    net.ParseIP("9.8.7.6"),
		UDPPort: uint16(33333),
	}
	member := &Member{
		ID:      MemberID{ID: "memberID"},
		Addr:    net.ParseIP("1.2.3.4"),
		UDPPort: uint16(11111),
	}

	end := make(chan struct{})
	pingFailed := make(chan struct{})
	defer func() {
		close(end)
		close(pingFailed)
	}()

	config := &Config{
		BindAddress: "9.8.7.6",
		BindPort:    33333,
	}

	swim := &SWIM{}
	swim.member = self
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.messageEndpoint = messageEndpoint

	err := swim.ping(member)
	assert.NoError(t, err)
}

func TestSWIM_ping_When_Response_Failed(t *testing.T) {
	pbk := pb.PiggyBack{
		MbrStatsMsg: &pb.MbrStatsMsg{
			Type:        pb.MbrStatsMsg_Alive,
			Id:          "123",
			Incarnation: uint32(123),
			UdpAddress:     "pbk-addr1",
		},
	}
	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.PushFunc = func(pbk pb.MbrStatsMsg) {}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return *pbk.MbrStatsMsg, nil
	}

	messageEndpoint := MockMessageEndpoint{}
	messageEndpoint.SyncSendFunc = func(addr string, msg pb.Message) (pb.Message, error) {
		assert.Equal(t, addr, "1.2.3.4:11111")
		assert.Equal(t, msg.Address, "9.8.7.6:33333")
		assert.Equal(t, msg.PiggyBack, &pbk)
		assert.Equal(t, msg.Payload.(*pb.Message_Ping).Ping, &pb.Ping{})

		// for mocking response failed, return ErrSendTimeout
		return pb.Message{}, ErrSendTimeout
	}

	self := &Member{
		ID:      MemberID{ID: "self"},
		Addr:    net.ParseIP("9.8.7.6"),
		UDPPort: uint16(33333),
	}
	member := &Member{
		ID:      MemberID{ID: "memberID"},
		Addr:    net.ParseIP("1.2.3.4"),
		UDPPort: uint16(11111),
	}

	end := make(chan struct{})
	pingFailed := make(chan struct{})
	defer func() {
		close(end)
		close(pingFailed)
	}()

	config := &Config{
		BindAddress: "9.8.7.6",
		BindPort:    33333,
	}

	swim := &SWIM{}
	swim.member = self
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.messageEndpoint = messageEndpoint

	err := swim.ping(member)
	assert.Error(t, err, ErrSendTimeout)
}

// test when one of K-members response with other than ACK or NACK
func TestSWIM_indirectProbe_When_Successfully_Probed(t *testing.T) {
	mIAddr := "127.0.0.1:11184"
	mJAddr := "127.0.0.1:11183"
	mK1Addr := "127.0.0.1:11181"
	mK2Addr := "127.0.0.1:11182"

	// Setup M_K1
	mK1MessageHandler := &MockMessageHandler{}
	mK1MessageEndpoint := createMessageEndpoint(t, mK1MessageHandler, time.Second*2, 11181)
	mK1MessageHandler.handleFunc = func(msg pb.Message) {
		ping := createPingMessage(xid.New().String(), mK1Addr, &pb.MbrStatsMsg{})
		_, err := mK1MessageEndpoint.SyncSend(mJAddr, ping)
		assert.NoError(t, err)

		ack := pb.Message{
			Id:      msg.Id,
			Payload: &pb.Message_Ack{Ack: &pb.Ack{}},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Id: "mbrID-1",
				},
			},
		}
		err = mK1MessageEndpoint.Send(mIAddr, ack)
		assert.NoError(t, err)
	}

	// Setup M_K2
	mK2MessageHandler := &MockMessageHandler{}
	mK2MessageEndpoint := createMessageEndpoint(t, mK2MessageHandler, time.Second*2, 11182)
	mK2MessageHandler.handleFunc = func(msg pb.Message) {
		ping := createPingMessage(xid.New().String(), mK2Addr, &pb.MbrStatsMsg{})
		mK2MessageEndpoint.SyncSend(mJAddr, ping)

		ack := pb.Message{
			Id:      msg.Id,
			Payload: &pb.Message_Ack{Ack: &pb.Ack{}},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Id: "mbrID-2",
				},
			},
		}
		mK2MessageEndpoint.Send(mIAddr, ack)
	}

	go mK1MessageEndpoint.Listen()
	go mK2MessageEndpoint.Listen()
	defer func() {
		mK1MessageEndpoint.Shutdown()
		mK2MessageEndpoint.Shutdown()
	}()

	mJMessageHandler := &MockMessageHandler{}
	mJMessageEndpoint := createMessageEndpoint(t, mJMessageHandler, time.Second, 11183)
	go mJMessageEndpoint.Listen()
	defer mJMessageEndpoint.Shutdown()

	mJMessageHandler.handleFunc = func(msg pb.Message) {
		ack := pb.Message{
			Id:      msg.Id,
			Payload: &pb.Message_Ack{Ack: &pb.Ack{}},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Id: "mbrID-3",
				},
			},
		}
		mJMessageEndpoint.Send(msg.Address, ack)
	}
	mJ := &Member{
		ID:      MemberID{ID: "mJ"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 11183,
	}

	// setup local-node
	mI := &Member{
		ID:      MemberID{ID: "mJ"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 11184,
	}
	config := &Config{
		BindAddress: "127.0.0.1",
		BindPort:    11184,
		K:           2,
	}

	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{}, nil
	}

	m1 := &Member{ID: MemberID{ID: "m1"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11181, Status: Alive}
	m2 := &Member{ID: MemberID{ID: "m2"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11182, Status: Alive}

	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[m1.ID] = m1
	mm.members[m2.ID] = m2

	swim := &SWIM{}
	swim.member = mI
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.memberMap = mm

	// setup SWIM's message endpoint
	tc := PacketTransportConfig{
		BindAddress: "127.0.0.1",
		BindPort:    11184,
	}
	p, _ := NewPacketTransport(&tc)

	meConfig := MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             time.Second * 3,
		CallbackCollectInterval: time.Hour,
	}
	messageEndpoint, _ := NewMessageEndpoint(meConfig, p, swim)

	swim.messageEndpoint = messageEndpoint

	go messageEndpoint.Listen()
	defer messageEndpoint.Shutdown()

	err := swim.indirectProbe(mJ)
	assert.NoError(t, err)
}

func TestSWIM_indirectProbe_When_All_Sent_Nack_Message(t *testing.T) {
	mIAddr := "127.0.0.1:11394"
	mK1Addr := "127.0.0.1:11391"
	mK2Addr := "127.0.0.1:11392"
	mJAddr := "127.0.0.1:11393"

	// Setup M_K1
	mK1MessageHandler := &MockMessageHandler{}
	mK1MessageEndpoint := createMessageEndpoint(t, mK1MessageHandler, time.Second*2, 11391)
	mK1MessageHandler.handleFunc = func(msg pb.Message) {
		ping := createPingMessage(xid.New().String(), mK1Addr, &pb.MbrStatsMsg{})
		_, err := mK1MessageEndpoint.SyncSend(mJAddr, ping)
		assert.Error(t, err, ErrSendTimeout)

		nack := pb.Message{
			Id:      msg.Id,
			Payload: &pb.Message_Nack{Nack: &pb.Nack{}},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Id: "mbrID-1",
				},
			},
		}
		err = mK1MessageEndpoint.Send(mIAddr, nack)
		assert.NoError(t, err)
	}

	// Setup M_K2
	mK2MessageHandler := &MockMessageHandler{}
	mK2MessageEndpoint := createMessageEndpoint(t, mK2MessageHandler, time.Second*2, 11392)
	mK2MessageHandler.handleFunc = func(msg pb.Message) {
		ping := createPingMessage(xid.New().String(), mK2Addr, &pb.MbrStatsMsg{})
		mK2MessageEndpoint.SyncSend(mJAddr, ping)

		nack := pb.Message{
			Id:      msg.Id,
			Payload: &pb.Message_Nack{Nack: &pb.Nack{}},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Id: "mbrID-2",
				},
			},
		}
		mK2MessageEndpoint.Send(mIAddr, nack)
	}

	go mK1MessageEndpoint.Listen()
	go mK2MessageEndpoint.Listen()
	defer func() {
		mK1MessageEndpoint.Shutdown()
		mK2MessageEndpoint.Shutdown()
	}()

	// setup M_J
	mJMessageHandler := &MockMessageHandler{}
	mJMessageEndpoint := createMessageEndpoint(t, mJMessageHandler, time.Second, 11393)
	go mJMessageEndpoint.Listen()
	defer mJMessageEndpoint.Shutdown()

	mJMessageHandler.handleFunc = func(msg pb.Message) {
		// m_j send nothing
	}
	mJ := &Member{
		ID:      MemberID{ID: "mJ"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 11393,
	}

	// setup local-node

	config := &Config{
		BindAddress: "127.0.0.1",
		BindPort:    11394,
		K:           2,
		T:           5000,
	}

	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{}, nil
	}

	m1 := &Member{ID: MemberID{ID: "m1"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11391, Status: Alive}
	m2 := &Member{ID: MemberID{ID: "m2"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11392, Status: Alive}

	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[m1.ID] = m1
	mm.members[m2.ID] = m2

	mI := &Member{
		ID:      MemberID{ID: "mIAA"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 11394,
	}

	swim := &SWIM{}
	swim.member = mI
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.memberMap = mm

	// setup SWIM's message endpoint
	tc := PacketTransportConfig{
		BindAddress: "127.0.0.1",
		BindPort:    11394,
	}
	p, _ := NewPacketTransport(&tc)

	meConfig := MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             time.Second * 3,
		CallbackCollectInterval: time.Hour,
	}
	messageEndpoint, _ := NewMessageEndpoint(meConfig, p, swim)

	swim.messageEndpoint = messageEndpoint

	end, indFailed := make(chan struct{}), make(chan struct{})
	defer func() {
		close(end)
		close(indFailed)
	}()

	go messageEndpoint.Listen()
	defer messageEndpoint.Shutdown()

	err := swim.indirectProbe(mJ)
	assert.Error(t, err, ErrIndProbeFailed)
}

func TestSWIM_indirectProbe_When_Some_Member_Sent_Nack_Message(t *testing.T) {
	mIAddr := "127.0.0.1:11254"
	mK1Addr := "127.0.0.1:11251"
	mK2Addr := "127.0.0.1:11252"
	mJAddr := "127.0.0.1:11253"

	// Setup M_K1
	mK1MessageHandler := &MockMessageHandler{}
	mK1MessageEndpoint := createMessageEndpoint(t, mK1MessageHandler, time.Second*2, 11251)
	mK1MessageHandler.handleFunc = func(msg pb.Message) {
		ping := createPingMessage(xid.New().String(), mK1Addr, &pb.MbrStatsMsg{})
		mK1MessageEndpoint.SyncSend(mJAddr, ping)

		ack := pb.Message{
			Id:      msg.Id,
			Payload: &pb.Message_Ack{Ack: &pb.Ack{}},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Id: "mbrID-1",
				},
			},
		}
		mK1MessageEndpoint.Send(mIAddr, ack)
	}

	// Setup M_K2
	mK2MessageHandler := &MockMessageHandler{}
	mK2MessageEndpoint := createMessageEndpoint(t, mK2MessageHandler, time.Second*2, 11252)
	mK2MessageHandler.handleFunc = func(msg pb.Message) {
		ping := createPingMessage(xid.New().String(), mK2Addr, &pb.MbrStatsMsg{})
		mK2MessageEndpoint.SyncSend(mJAddr, ping)

		nack := pb.Message{
			Id:      msg.Id,
			Payload: &pb.Message_Nack{Nack: &pb.Nack{}},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Id: "mbrID-2",
				},
			},
		}
		mK2MessageEndpoint.Send(mIAddr, nack)
	}

	go mK1MessageEndpoint.Listen()
	go mK2MessageEndpoint.Listen()
	defer func() {
		mK1MessageEndpoint.Shutdown()
		mK2MessageEndpoint.Shutdown()
	}()

	mJMessageHandler := &MockMessageHandler{}
	mJMessageEndpoint := createMessageEndpoint(t, mJMessageHandler, time.Second, 11253)
	go mJMessageEndpoint.Listen()
	defer mJMessageEndpoint.Shutdown()

	// half of the members will send to m_i ACK message
	i := 1
	mJMessageHandler.handleFunc = func(msg pb.Message) {
		if i == 1 {
			ack := pb.Message{Id: msg.Id, Payload: &pb.Message_Ack{Ack: &pb.Ack{}}, PiggyBack: &pb.PiggyBack{}}
			err := mJMessageEndpoint.Send(msg.Address, ack)
			assert.NoError(t, err)
		}
		i++
	}
	mJ := &Member{
		ID:      MemberID{ID: "mJ"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 11253,
	}

	// setup local-node

	config := &Config{
		BindAddress: "127.0.0.1",
		BindPort:    11254,
		K:           2,
	}

	pbkStore := MockMbrStatsMsgStore{}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{}, nil
	}

	m1 := &Member{ID: MemberID{ID: "m1"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11251, Status: Alive}
	m2 := &Member{ID: MemberID{ID: "m2"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11252, Status: Alive}

	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[m1.ID] = m1
	mm.members[m2.ID] = m2

	mI := &Member{
		ID:      MemberID{ID: "mI"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 11254,
	}

	swim := &SWIM{}
	swim.member = mI
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.memberMap = mm

	// setup SWIM's message endpoint
	tc := PacketTransportConfig{
		BindAddress: "127.0.0.1",
		BindPort:    11254,
	}
	p, _ := NewPacketTransport(&tc)

	meConfig := MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             time.Second * 3,
		CallbackCollectInterval: time.Hour,
	}
	messageEndpoint, _ := NewMessageEndpoint(meConfig, p, swim)

	swim.messageEndpoint = messageEndpoint

	end, indFailed := make(chan struct{}), make(chan struct{})
	defer func() {
		close(end)
		close(indFailed)
	}()

	go messageEndpoint.Listen()
	defer messageEndpoint.Shutdown()

	err := swim.indirectProbe(mJ)
	assert.NoError(t, err)
}

func TestSWIM_probe_When_Member_Is_Dead(t *testing.T) {
	swim := &SWIM{}
	member := Member{
		ID:     MemberID{ID: "111"},
		Status: Dead,
	}

	swim.probe(member, time.Second)
}

func TestSWIM_probe_When_Target_Respond_To_Ping(t *testing.T) {
	// setup M_J member
	mJMember := Member{ID: MemberID{ID: "mj"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11161, Status: Alive}
	mJMessageHandler := &MockMessageHandler{}

	mJMessageEndpoint := createMessageEndpoint(t, mJMessageHandler, time.Second, 11161)
	mJMessageHandler.handleFunc = func(msg pb.Message) {
		ack := pb.Message{
			Id:      msg.Id,
			Payload: &pb.Message_Ack{Ack: &pb.Ack{}},
			PiggyBack: &pb.PiggyBack{
				MbrStatsMsg: &pb.MbrStatsMsg{
					Id: "mbrID-1",
				},
			},
		}
		mJMessageEndpoint.Send("127.0.0.1:11162", ack)
	}
	go mJMessageEndpoint.Listen()
	defer mJMessageEndpoint.Shutdown()

	// setup M_I member
	config := &Config{
		BindAddress: "127.0.0.1",
		BindPort:    11162,
		K:           2,
		T:           5000,
	}

	pbkStore := &MockMbrStatsMsgStore{}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{}, nil
	}

	mm := NewMemberMap(&SuspicionConfig{
		K:        2,
		MinParam: 5,
		MaxParam: 1,
	})

	mI := &Member{
		ID:      MemberID{ID: "mI"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 11162,
	}

	awareness := NewAwareness(8)
	awareness.ApplyDelta(2)

	swim := &SWIM{}
	swim.member = mI
	swim.awareness = awareness
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.memberMap = mm

	// setup SWIM's message endpoint
	tc := PacketTransportConfig{
		BindAddress: "127.0.0.1",
		BindPort:    11162,
	}
	p, _ := NewPacketTransport(&tc)

	meConfig := MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             time.Second * 5,
		CallbackCollectInterval: time.Hour,
	}
	mIMessageEndpoint, _ := NewMessageEndpoint(meConfig, p, swim)

	swim.messageEndpoint = mIMessageEndpoint

	go mIMessageEndpoint.Listen()
	defer mIMessageEndpoint.Shutdown()

	swim.probe(mJMember, 5*time.Second)

	assert.Equal(t, swim.awareness.score, 1)
}

// This test case is for testing if we failed at ping to M_J, then indirect-probing is execute
// with M_J respond ACK message
//
// Test case scenario:
//
//   1. M_I send ping message to M_J
//   2. M_J ignore ping message
//   3. M_I start indirect-probe, so send indirect-ping message to mk1, mk2
//   4. After mk1, mk2 received indirect-ping, then ping to M_J
//   5. M_J response with ACK message to both of mk1, mk2
//   6. mk1, mk2 send back ACK message to M_I
//   7. M_I successfully finish indirect-probing
//
func TestSWIM_probe_When_Target_Respond_To_Indirect_Ping(t *testing.T) {
	mJAddress := "127.0.0.1:13161"
	mIAddress := "127.0.0.1:13162"
	mK1Address := "127.0.0.1:13163"
	mK2Address := "127.0.0.1:13164"

	//
	// setup M_J message endpoint, SWIM
	//
	mJMessageHandler := &MockMessageHandler{}
	mJMessageEndpoint := createMessageEndpoint(t, mJMessageHandler, time.Second, 13161)

	// only ignore first message
	i := 0
	mJMessageHandler.handleFunc = func(msg pb.Message) {
		// First message handling should ignore, for starting
		// indirect-probing
		if i == 0 {
			i++
			return
		}

		// handle messages from mk1, mk2, response with ACK message
		t.Logf("handling message from [%s]", msg.Address)
		ack := pb.Message{Id: msg.Id, Payload: &pb.Message_Ack{Ack: &pb.Ack{}}, PiggyBack: &pb.PiggyBack{}}
		mJMessageEndpoint.Send(msg.Address, ack)
	}

	//
	// setup M_I message endpoint, SWIM
	//
	config := &Config{
		BindAddress: "127.0.0.1",
		BindPort:    13162,
		K:           2,
		T:           5000,
	}

	pbkStore := &MockMbrStatsMsgStore{}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{}, nil
	}

	mI := &Member{
		ID:      MemberID{ID: "mI"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 13162,
	}

	m1Member := &Member{ID: MemberID{ID: "m1"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 13163, Status: Alive}
	m2Member := &Member{ID: MemberID{ID: "m2"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 13164, Status: Alive}

	mm := NewMemberMap(&SuspicionConfig{
		K:        2,
		MinParam: 5,
		MaxParam: 1,
	})
	mm.members[m1Member.ID] = m1Member
	mm.members[m2Member.ID] = m2Member

	awareness := NewAwareness(8)
	awareness.ApplyDelta(2)

	swim := &SWIM{}
	swim.member = mI
	swim.awareness = awareness
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.memberMap = mm

	// setup SWIM's message endpoint
	tc := PacketTransportConfig{
		BindAddress: "127.0.0.1",
		BindPort:    13162,
	}
	p, _ := NewPacketTransport(&tc)

	meConfig := MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             time.Second * 3,
		CallbackCollectInterval: time.Hour,
	}
	mIMessageEndpoint, _ := NewMessageEndpoint(meConfig, p, swim)

	swim.messageEndpoint = mIMessageEndpoint

	go mIMessageEndpoint.Listen()
	defer mIMessageEndpoint.Shutdown()

	//
	// setup M_K_1 message endpoint
	//
	mk1MessageHandler := &MockMessageHandler{}
	mk1MessageEndpoint := createMessageEndpoint(t, mk1MessageHandler, time.Second, 13163)

	mk1MessageHandler.handleFunc = func(msg pb.Message) {
		// test whether received message type is indirect-ping
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing, &pb.IndirectPing{Target: "127.0.0.1:13161"})
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing.Target, "127.0.0.1:13161")
		assert.Equal(t, msg.Address, mIAddress)

		ping := createPingMessage(xid.New().String(), mK1Address, &pb.MbrStatsMsg{})
		// ExchangeMessage ping message to mj
		mk1MessageEndpoint.SyncSend(mJAddress, ping)
		ack := createAckMessage(msg.Id, mK1Address, &pb.MbrStatsMsg{})
		// send back to mi
		time.Sleep(time.Second)
		mk1MessageEndpoint.Send(mIAddress, ack)
		return
	}

	//
	// setup M_K_2 message endpoint,
	//
	mk2MessageHandler := &MockMessageHandler{}
	mk2MessageEndpoint := createMessageEndpoint(t, mk2MessageHandler, time.Second, 13164)

	mk2MessageHandler.handleFunc = func(msg pb.Message) {
		// test whether received message type is indirect-ping
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing, &pb.IndirectPing{Target: "127.0.0.1:13161"})
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing.Target, "127.0.0.1:13161")
		assert.Equal(t, msg.Address, mIAddress)

		ping := createPingMessage(xid.New().String(), mK2Address, &pb.MbrStatsMsg{})
		// ExchangeMessage ping message to mj
		mk2MessageEndpoint.SyncSend(mJAddress, ping)
		ack := createAckMessage(msg.Id, mK2Address, &pb.MbrStatsMsg{})
		// send back to mi
		mk2MessageEndpoint.Send(mIAddress, ack)
		return
	}

	go mJMessageEndpoint.Listen()
	go mIMessageEndpoint.Listen()
	go mk1MessageEndpoint.Listen()
	go mk2MessageEndpoint.Listen()
	defer func() {
		mJMessageEndpoint.Shutdown()
		mIMessageEndpoint.Shutdown()
		mk1MessageEndpoint.Shutdown()
		mk2MessageEndpoint.Shutdown()
	}()

	mJMember := Member{ID: MemberID{ID: "mj"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 13161, Status: Alive}
	swim.probe(mJMember, 5*time.Second)

	assert.Equal(t, swim.awareness.score, 1)
}

// This test case is for testing if we failed at ping to M_J, then indirect-probing is execute
// but M_J not response to indirect-ping message so probe finally failed
//
// Test case scenario:
//
//   1. M_I send ping message to M_J
//   2. M_J ignore ping message
//   3. M_I start indirect-probe, so send indirect-ping message to mk1, mk2
//   4. After mk1, mk2 received indirect-ping, then ping to M_J
//   5. M_J NOT response to both of mk1, mk2
//   6. mk1, mk2 send back NACK message to M_I
//   7. M_I finally failed indirect-probing, then suspect M_J
//
func TestSWIM_probe_When_Target_Not_Respond_To_Indirect_Ping(t *testing.T) {
	mJAddress := "127.0.0.1:11161"
	mIAddress := "127.0.0.1:11162"
	mK1Address := "127.0.0.1:11163"
	mK2Address := "127.0.0.1:11164"

	//
	// setup M_J message endpoint, SWIM
	//
	mJMessageHandler := &MockMessageHandler{}
	mJMessageEndpoint := createMessageEndpoint(t, mJMessageHandler, time.Second, 11161)

	// M_J not response to both of ping message and indirect-ping message
	mJMessageHandler.handleFunc = func(msg pb.Message) {
		return
	}

	//
	// setup M_I message endpoint, SWIM
	//
	config := &Config{
		BindAddress: "127.0.0.1",
		BindPort:    11162,
		K:           2,
		T:           5000,
	}

	pbkStore := &MockMbrStatsMsgStore{}
	pbkStore.GetFunc = func() (pb.MbrStatsMsg, error) {
		return pb.MbrStatsMsg{}, nil
	}

	mI := &Member{
		ID:      MemberID{ID: "mI"},
		Addr:    net.ParseIP("127.0.0.1"),
		UDPPort: 11162,
	}

	m1Member := &Member{ID: MemberID{ID: "m1"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11163, Status: Alive}
	m2Member := &Member{ID: MemberID{ID: "m2"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11164, Status: Alive}

	mm := NewMemberMap(&SuspicionConfig{})
	mm.members[m1Member.ID] = m1Member
	mm.members[m2Member.ID] = m2Member

	awareness := NewAwareness(8)
	awareness.ApplyDelta(2)

	swim := &SWIM{}
	swim.member = mI
	swim.awareness = awareness
	swim.config = config
	swim.mbrStatsMsgStore = pbkStore
	swim.memberMap = mm

	// setup SWIM's message endpoint
	tc := PacketTransportConfig{
		BindAddress: "127.0.0.1",
		BindPort:    11162,
	}
	p, _ := NewPacketTransport(&tc)

	meConfig := MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             time.Second * 3,
		CallbackCollectInterval: time.Hour,
	}
	mIMessageEndpoint, _ := NewMessageEndpoint(meConfig, p, swim)

	swim.messageEndpoint = mIMessageEndpoint

	go mIMessageEndpoint.Listen()
	defer mIMessageEndpoint.Shutdown()

	//
	// setup M_K_1 message endpoint
	//
	mk1MessageHandler := &MockMessageHandler{}
	mk1MessageEndpoint := createMessageEndpoint(t, mk1MessageHandler, time.Second, 11163)

	mk1MessageHandler.handleFunc = func(msg pb.Message) {
		// test whether received message type is indirect-ping
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing, &pb.IndirectPing{Target: "127.0.0.1:11161"})
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing.Target, "127.0.0.1:11161")
		assert.Equal(t, msg.Address, mIAddress)

		ping := createPingMessage(xid.New().String(), mK1Address, &pb.MbrStatsMsg{})
		// ExchangeMessage ping message to mj
		_, err := mk1MessageEndpoint.SyncSend(mJAddress, ping)

		assert.Error(t, err, ErrSendTimeout)

		// mk1 response with nack message
		nack := createNackMessage(msg.Id, mK1Address, &pb.MbrStatsMsg{})
		// send back to mi
		mk1MessageEndpoint.Send(mIAddress, nack)
		return
	}

	//
	// setup M_K_2 message endpoint,
	//
	mk2MessageHandler := &MockMessageHandler{}
	mk2MessageEndpoint := createMessageEndpoint(t, mk2MessageHandler, time.Second, 11164)

	mk2MessageHandler.handleFunc = func(msg pb.Message) {
		// test whether received message type is indirect-ping
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing, &pb.IndirectPing{Target: "127.0.0.1:11161"})
		assert.Equal(t, msg.Payload.(*pb.Message_IndirectPing).IndirectPing.Target, "127.0.0.1:11161")
		assert.Equal(t, msg.Address, mIAddress)

		ping := createPingMessage(xid.New().String(), mK2Address, &pb.MbrStatsMsg{})
		// ExchangeMessage ping message to mj
		_, err := mk2MessageEndpoint.SyncSend(mJAddress, ping)

		assert.Error(t, err, ErrSendTimeout)

		nack := createNackMessage(msg.Id, mK2Address, &pb.MbrStatsMsg{})

		// send back to mi
		mk2MessageEndpoint.Send(mIAddress, nack)
		return
	}

	go mJMessageEndpoint.Listen()
	go mIMessageEndpoint.Listen()
	go mk1MessageEndpoint.Listen()
	go mk2MessageEndpoint.Listen()
	defer func() {
		mJMessageEndpoint.Shutdown()
		mIMessageEndpoint.Shutdown()
		mk1MessageEndpoint.Shutdown()
		mk2MessageEndpoint.Shutdown()
	}()

	mJMember := Member{ID: MemberID{ID: "mj"}, Addr: net.ParseIP("127.0.0.1"), UDPPort: 11161, Status: Alive}
	swim.probe(mJMember, 5*time.Second)

	assert.Equal(t, swim.awareness.score, 3)
}

func createMessageEndpoint(t *testing.T, messageHandler MessageHandler, sendTimeout time.Duration, port int) MessageEndpoint {
	mConfig := MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             sendTimeout,
		CallbackCollectInterval: time.Hour,
	}

	tConfig := &PacketTransportConfig{
		BindAddress: "127.0.0.1",
		BindPort:    port,
	}

	transport, err := NewPacketTransport(tConfig)
	assert.NoError(t, err)

	m, _ := NewMessageEndpoint(mConfig, transport, messageHandler)
	return m
}
