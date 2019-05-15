/* 
 * Copyright 2019 hea9549
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package evaluator

import (
	"github.com/DE-labtory/swim"
	"github.com/DE-labtory/swim/cui"
	ui "github.com/gizak/termui/v3"
	"strconv"
	"strings"
	"time"
)

type Evaluator struct {
	ExactMember             []swim.Member
	Swimmer                 map[string]*swim.SWIM
	SpecificSwimmer         *swim.SWIM
	MsgEndpoint             map[string]*swim.EvaluatorMessageEndpoint
	lastID                  int
	vis                     *Visualizer
	ProcessCmdChan          chan string
	ProcessMemberStatusChan chan interface{}
	ProcessNetworkLoadChan  chan interface{}
	toStop                  bool
	scheduleStopCh          chan interface{}
	keyboardStopCh          chan interface{}
}

func NewEvaluator() (*Evaluator, error) {
	vis, err := NewVisualizer()
	if err != nil {
		return nil, err
	}
	return &Evaluator{
		ExactMember:             make([]swim.Member, 0),
		Swimmer:                 make(map[string]*swim.SWIM),
		MsgEndpoint:             make(map[string]*swim.EvaluatorMessageEndpoint),
		lastID:                  0,
		vis:                     vis,
		ProcessCmdChan:          make(chan string),
		ProcessMemberStatusChan: make(chan interface{}),
		ProcessNetworkLoadChan:  make(chan interface{}),
		keyboardStopCh:          make(chan interface{}),
		scheduleStopCh:          make(chan interface{}),
		toStop:                  false,
	}, nil
}
func (e *Evaluator) scheduling() {
	go func() {
		for {
			if e.toStop {
				return
			}
			e.ProcessMemberStatusChan <- struct{}{}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	go func() {
		for {
			if e.toStop {
				return
			}
			e.ProcessNetworkLoadChan <- struct{}{}
			time.Sleep(time.Second)
		}
	}()
}

func (e *Evaluator) Start() {
	e.toStop = false
	go e.vis.Start()
	go e.scheduling()
	go func() {
		uiEvents := ui.PollEvents()
		for {
			if e.toStop {
				return
			}
			select {
			case event := <-uiEvents:
				switch event.Type {
				case ui.KeyboardEvent:
					switch event.ID {
					case "<C-c>":
						e.Shutdown()
						return
					case "<Backspace>", "<C-<Backspace>>", "<Left>", "<Right>", "<Space>":
						e.vis.HandleCmdTextBox(event.ID)
						break
					case "<Enter>":
						cmd := e.vis.HandleCmdTextBox(event.ID)
						e.ProcessCmdChan <- cmd
						break
					case "<Up>":
						e.vis.UpdateMemberViewCursorChan <- UP
						break
					case "<Down>":
						e.vis.UpdateMemberViewCursorChan <- DOWN
						break
					default:
						if cui.ContainsString(cui.PRINTABLE_KEYS, event.ID) {
							e.vis.HandleCmdTextBox(event.ID)
						}
						break
					}
				}
				break
			case <-e.keyboardStopCh:
				return
			}
		}
	}()

	for {
		if e.toStop {
			return
		}

		select {
		case <-e.scheduleStopCh:
			return
		case cmd := <-e.ProcessCmdChan:
			e.processCmd(cmd)
			break
		case <-e.ProcessMemberStatusChan:
			e.vis.UpdateMemberDataChan <- e.processMemberStatus()
			if e.SpecificSwimmer != nil{
				liveNodeNum := 0
				suspectNodeNum := 0
				deadNodeNum := 0
				suspectNodeList := make([]string,0)
				deadNodeList := make([]string,0)
				for _,m := range e.SpecificSwimmer.GetMemberMap().GetMembers(){
					switch m.Status{
					case swim.Alive:
						liveNodeNum++
					case swim.Suspected:
						suspectNodeNum++
						suspectNodeList = append(suspectNodeList, m.ID.ID)
					case swim.Dead:
						deadNodeNum++
						deadNodeList = append(deadNodeList, m.ID.ID)
					}
				}
				info := e.SpecificSwimmer.GetMyInfo()
				sInfo := selectedInfo{
					ID:              info.ID.ID,
					LiveNodeNum:     liveNodeNum,
					SuspectNodeNum:  suspectNodeNum,
					DeadNodeNum:     deadNodeNum,
					DeadNodeList:    deadNodeList,
					SuspectNodeList: suspectNodeList,
					Incarnation:     int(info.Incarnation),
				}
				e.vis.UpdateSelectedInfoViewChan <- sInfo
			}
			break
		case <-e.ProcessNetworkLoadChan:
			in, out := e.processInOutPacketSparkle()
			networkInfoData := networkLoadInfo{
				avgInMsg:  in,
				avgOutMsg: out,
			}
			e.vis.AppendNetworkLoadChan <- networkInfoData
			break
		}
	}
}

func (e *Evaluator) Shutdown() {
	e.vis.Shutdown()
	e.keyboardStopCh <- struct{}{}
	e.scheduleStopCh <- struct{}{}
	e.toStop = true
}

func (e *Evaluator) processCmd(cmd string) {
	cmdList := strings.Split(cmd, " ")
	if len(cmdList) < 1 {
		e.vis.AppendLogChan <- "you have to input cmd"
		return
	}
	cmdDomain := cmdList[0]
	switch cmdDomain {
	case "create":
		if len(cmdList) < 2 {
			e.vis.AppendLogChan <- "you have to input create node num. [ex : create 5]"
		}

		num, err := strconv.Atoi(cmdList[1])
		if err != nil {
			e.vis.AppendLogChan <- "you have to input create node number, not " + cmdList[1]
			return
		}

		for i := 0; i < num; i++ {
			e.lastID++
			tcpPort := GetAvailablePort(20000)
			udpPort := tcpPort
			id := strconv.Itoa(e.lastID)
			s, msgEndpoint := SetupSwim(GetDefaultSwimmerConfig(id, "127.0.0.1", tcpPort, udpPort))
			e.Swimmer[id] = s
			e.MsgEndpoint[id] = msgEndpoint
			e.ExactMember = append(e.ExactMember, *s.GetMyInfo())
			go s.Start()
			time.Sleep(10 * time.Millisecond)
		}
		e.vis.AppendLogChan <- "successfully create nodes " + strconv.Itoa(num)
	case "connect":
		if len(cmdList) < 3 {
			e.vis.AppendLogChan <- "you have to input src, dst [ex : connect all 2, connect 2 6, connect {src} {dst}]"
			return
		}

		src := cmdList[1]
		dst := cmdList[2]
		if src == "all" {
			if dst == "all" {
				e.vis.AppendLogChan <- "all to all connect is not available now"
				return
			}
			if dstSwimmer, ok := e.Swimmer[dst]; ok {
				dstInfo := dstSwimmer.GetMyInfo()
				for _, srcSwimmer := range e.Swimmer {
					if srcSwimmer.GetMyInfo().ID == dstInfo.ID {
						continue
					}
					_ = srcSwimmer.Join([]string{dstInfo.TCPAddress()})
				}
			} else {
				e.vis.AppendLogChan <- "input swim dst id is invalid. input : " + dst
				return
			}
			e.vis.AppendLogChan <- "successfully join " + src + " to " + dst
			return
		}
		if srcSwimmer, ok := e.Swimmer[src]; ok {

			if dstSwimmer, ok := e.Swimmer[dst]; ok {
				dstInfo := dstSwimmer.GetMyInfo()
				_ = srcSwimmer.Join([]string{dstInfo.TCPAddress()})

				if msgEndpoint , ok := e.MsgEndpoint[src]; ok{
					msgEndpoint.RemoveDisconnectAddr(dstInfo.UDPAddress())
				}
				e.vis.AppendLogChan <- "successfully join " + src + " to " + dst
				return
			} else {
				e.vis.AppendLogChan <- "input swim dst id is invalid. input : " + dst
				return
			}
		} else {
			e.vis.AppendLogChan <- "input swim src id is invalid. input : " + src
			return
		}
	case "delete", "remove":
		if len(cmdList) < 2 {
			e.vis.AppendLogChan <- "you have to input id to remove [ex : remove 2, delete 6]"
			return
		}
		id := cmdList[1]
		if swimmer, ok := e.Swimmer[id]; ok {
			if !swimmer.IsRun() {
				e.vis.AppendLogChan <- "that swimmer is already die..."
				return
			}
			swimmer.ShutDown()
			for idx, mem := range e.ExactMember {
				if mem.ID.ID == id {
					e.ExactMember = append(e.ExactMember[:idx], e.ExactMember[idx+1:]...)
					e.vis.AppendLogChan <- "successfully remove swimmer : " + id
					return
				}
			}
			e.vis.AppendLogChan <- "successfully remove swimmer : " + id
		} else {
			e.vis.AppendLogChan <- "there is no swimmer : " + id
		}
	case "set":
		if len(cmdList) < 4 {
			e.vis.AppendLogChan <- "set usage : set [domain] [value] [target]"
			return
		}
		domain := cmdList[1]
		value := cmdList[2]
		target := cmdList[3]
		switch domain {
		case "sendpacketloss":
			fData, err := strconv.ParseFloat(value, 64)
			if err != nil {
				e.vis.AppendLogChan <- "invalid float value, input : " + value
				return
			}
			if target == "all" {
				for _, msgEndpoint := range e.MsgEndpoint {
					msgEndpoint.PacketLossRate = fData
				}
				e.vis.AppendLogChan <- "successfully set send packet loss: " + value + ", swimmer : " + target
				return
			} else {
				if msgEndpoint, ok := e.MsgEndpoint[target]; ok {
					msgEndpoint.PacketLossRate = fData
					e.vis.AppendLogChan <- "successfully set send packet loss: " + value + ", swimmer : " + target
					return
				} else {
					e.vis.AppendLogChan <- "there is no swimmer : " + target
					return
				}
			}
		}
	case "view":
		if len(cmdList) < 2 {
			e.vis.AppendLogChan <- "view usage : view [member ID]"
			return
		}
		target := cmdList[1]
		if swimmer, ok := e.Swimmer[target]; ok {
			e.SpecificSwimmer = swimmer
			e.vis.AppendLogChan <- "successfully view change: " + target
			return
		} else {
			e.vis.AppendLogChan <- "there is no swimmer : " + target
			return
		}
	case "disconnect":
		if len(cmdList) < 3 {
			e.vis.AppendLogChan <- "view usage : disconnect [src ID] [dst ID]"
			return
		}
		src := cmdList[1]
		dst := cmdList[2]
		if msgEndpoint, ok := e.MsgEndpoint[src]; ok {
			if dstSwimmer, ok := e.Swimmer[dst]; ok{
				msgEndpoint.AddDisconnectAddr(dstSwimmer.GetMyInfo().UDPAddress())
				e.vis.AppendLogChan <- "successfully disconnect " +src+" -> "+ dst
				return
			}else{
				e.vis.AppendLogChan <- "there is no swimmer : " + dst
				return
			}

		}else{
			e.vis.AppendLogChan <- "there is no swimmer : " + src
			return
		}
	default:
		e.vis.AppendLogChan <- "invalid cmd : " + cmdDomain
	}
}

func (e *Evaluator) processMemberStatus() []IdProbStruct {

	dataList := make([]IdProbStruct, 0)

	for _, swimmer := range e.Swimmer {
		memberMap := swimmer.GetMemberMap()
		prob := compareMemberList(e.ExactMember, memberMap.GetMembers())
		dataList = append(dataList, IdProbStruct{
			Id:    swimmer.GetMyInfo().ID.ID,
			Prob:  prob,
			IsRun: swimmer.IsRun(),
		})
	}

	return dataList
}

func compareMemberList(expected []swim.Member, data []swim.Member) float64 {
	correctNum := len(expected)
	miss := 0

	for _, oneMember := range expected {
		if !isInSameDataMember(oneMember, data) {
			miss++
		}
	}

	for _, oneMember := range data {
		if !isInSameDataMember(oneMember, expected) && oneMember.Status != swim.Dead {
			miss++
		}
	}
	miss--
	//if correctNum < 1 {
	//		correctNum = 1
	//}
	probability := 1 - float64(miss)/float64(correctNum)
	if probability < 0 {
		probability = 0
	}
	return probability * 100
}

func isInSameDataMember(checkMember swim.Member, memberList []swim.Member) bool {
	for _, m := range memberList {
		if m.ID.ID == checkMember.ID.ID && m.Status!= swim.Dead && checkMember.Status != swim.Dead {
			return true
		}
	}
	return false

}

func (e *Evaluator) processInOutPacketSparkle() (float64, float64) {

	var totalInPacket, totalOutPacket int

	for _, m := range e.MsgEndpoint {
		totalInPacket += m.PopInPacketCounter()
		totalOutPacket += m.PopOutPacketCounter()
	}
	lenSwimmer := float64(len(e.MsgEndpoint))
	if lenSwimmer == 0 {
		lenSwimmer = float64(1)
	}
	avgInPacket := float64(totalInPacket) / lenSwimmer
	avgOutPacket := float64(totalOutPacket) / lenSwimmer

	return avgInPacket, avgOutPacket
}
