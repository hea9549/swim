package main

import (
	"fmt"
	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim"
	"github.com/DE-labtory/swim/cui"
	"github.com/DE-labtory/swim/evaluator"
	ui "github.com/gizak/termui/v3"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)


func main() {
	iLogger.EnableStd(false)
	//go http.ListenAndServe("localhost:6060", nil)

	vis,err := evaluator.NewVisualizer()
	if err !=nil{
		return
	}
	defer vis.Shutdown()

	worker := evaluator.NewWorker()
	ev := evaluator.NewEvaluator()

	uiEvents := ui.PollEvents()
	go func() {
		tick := 0
		for {
			ev.processLock.Lock()
			time.Sleep(100 * time.Millisecond)
			//ev.AppendLog("1")
			str, avgProb := processMemberStatus(ev.ExactMember, ev.Swimmer, ev.nodeInfoCur)
			l.Rows[0] = "node num : " + strconv.Itoa(len(ev.ExactMember))
			ev.Render(l)
			//ev.AppendLog("2")
			nodeInfo.Text = str
			ev.Render(nodeInfo)
			//ev.AppendLog("3")
			in, out := processInOutPacketSparkle(ev.MsgEndpoint)
			//ev.AppendLog("4")
			slg1.Sparklines[0].Data = append(slg1.Sparklines[0].Data[1:], in)
			slg1.Sparklines[1].Data = append(slg1.Sparklines[1].Data[1:], out)
			ev.Render(slg1)
			if avgProb < 0 || avgProb > 100 || math.IsNaN(avgProb) {
				ev.AppendLog("non detected")
			}
			g2.Percent = int(avgProb)
			ui.Render(g2)
			tick++
			if tick%40 == 0 {
				nodeInfo.Text = " "
				ev.Render(nodeInfo)
			}

			ev.processLock.Unlock()
		}
	}()

	for {
		e := <-uiEvents
		switch e.Type {
		case ui.KeyboardEvent:
			switch e.ID {
			case "<C-c>":
				return
			case "<Backspace>", "<C-<Backspace>>":
				tb.Backspace()
				break
			case "<Left>":
				tb.MoveCursorLeft()
				break
			case "<Up>":
				if ev.nodeInfoCur > 0 {
					ev.nodeInfoCur -= 1
				}
				break
			case "<Down>":
				ev.nodeInfoCur += 1
				break
			case "<Right>":
				tb.MoveCursorRight()
				break
			case "<Space>":
				tb.InsertText(" ")
				break
			case "<Enter>":
				//ev.c(tb.GetRawText())
				tb.ClearText()
				break
			default:
				if cui.ContainsString(cui.PRINTABLE_KEYS, e.ID) {
					tb.InsertText(e.ID)
				}
				break
			}
			ev.Render(tb)
		case ui.ResizeEvent:

		}
	}

}


func processInOutPacketSparkle(swimmers map[string]*swim.EvaluatorMessageEndpoint) (float64, float64) {

	var totalInPacket, totalOutPacket int

	for _, m := range swimmers {
		totalInPacket += m.PopInPacketCounter()
		totalOutPacket += m.PopOutPacketCounter()
	}
	lenSwimmer := float64(len(swimmers))
	if lenSwimmer == 0 {
		lenSwimmer = float64(1)
	}
	avgInPacket := float64(totalInPacket) / lenSwimmer
	avgOutPacket := float64(totalOutPacket) / lenSwimmer

	return avgInPacket, avgOutPacket
}

func processMemberStatus(exactMembers []swim.Member, swimmers map[string]*swim.SWIM, nodeInfoCur int) (string, float64) {

	dataList := make([]evaluator.IdProbStruct, 0)
	totalProb := float64(0)

	for _, swimmer := range swimmers {
		memberMap := swimmer.GetMemberMap()
		prob := compareMemberList(exactMembers, memberMap.GetMembers())

		totalProb += prob
		dataList = append(dataList, evaluator.IdProbStruct{
			Id:    swimmer.GetMyInfo().ID.ID,
			Prob:  prob,
			IsRun: swimmer.IsRun(),
		})
	}
	memLen := len(dataList)
	if memLen == 0 {
		memLen = 1
	}
	avgProb := totalProb / float64(memLen)
	if avgProb < 0 {
		avgProb = float64(0)
	}
	if avgProb > 100 {
		avgProb = float64(100)
	}
	sort.Slice(dataList[:], func(i, j int) bool {
		iInt, err := strconv.Atoi(dataList[i].Id)
		jInt, err2 := strconv.Atoi(dataList[j].Id)
		if err != nil || err2 != nil {
			return dataList[i].Id < dataList[j].Id
		}
		return iInt < jInt
	})
	visualizeString := ""
	counter := 0
	for _, oneData := range dataList {
		switch {
		case !oneData.IsRun:
			visualizeString += fmt.Sprintf("%4sW", oneData.Id)
			break
		case oneData.Prob >= float64(100):
			visualizeString += fmt.Sprintf("%4sG", oneData.Id)
			break
		case oneData.Prob >= float64(95):
			visualizeString += fmt.Sprintf("%4sB", oneData.Id)
			break
		case oneData.Prob >= float64(90):
			visualizeString += fmt.Sprintf("%4sC", oneData.Id)
			break
		case oneData.Prob >= float64(85):
			visualizeString += fmt.Sprintf("%4sY", oneData.Id)
			break
		case oneData.Prob >= float64(80):
			visualizeString += fmt.Sprintf("%4sM", oneData.Id)
			break
		case oneData.Prob < float64(80):
			visualizeString += fmt.Sprintf("%4sR", oneData.Id)
			break
		default:
			visualizeString += ""
		}
		counter++
		if counter == 12 {
			visualizeString += "\n"
			counter = 0
		}

	}

	nodeInfoData := strings.Split(visualizeString, "\n")
	if nodeInfoCur < 0 {
		nodeInfoCur = 0
	}
	if nodeInfoCur < len(nodeInfoData) {
		nodeInfoData = nodeInfoData[nodeInfoCur:]
	}

	visualizeString = strings.Join(nodeInfoData[:], "\n")

	return visualizeString, avgProb
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
		if m.ID.ID == checkMember.ID.ID && m.Status.String() == checkMember.Status.String() {
			return true
		}
	}
	return false

}
func (e *Evaluator) StartScheduler() {

}
func (e *Evaluator) AppendLog(log string) {
	e.cmdLog.Rows = append(e.cmdLog.Rows[1:], log)
	e.Render(e.cmdLog)
}

func (e *Evaluator) Render(item ui.Drawable) {
	ui.Render(item)
}

func (e *Evaluator) processCmd(ch chan string) {
	for {
		cmd := <-ch
		cmdList := strings.Split(cmd, " ")
		if len(cmdList) < 1 {
			e.AppendLog("you have to input cmd")
		}
		cmdDomain := cmdList[0]
		switch cmdDomain {
		case "create":
			if len(cmdList) < 2 {
				e.AppendLog("you have to input create node num. [ex : create 5]")
			}

			num, err := strconv.Atoi(cmdList[1])
			if err != nil {
				e.AppendLog("you have to input create node number, not " + cmdList[1])
			}
			for i := 0; i < num; i++ {

				e.processLock.Lock()
				e.lastID++
				tcpPort := evaluator.GetAvailablePort(20000)
				udpPort := tcpPort
				id := strconv.Itoa(e.lastID)

				s, msgEndpoint := SetupSwim("127.0.0.1", tcpPort, udpPort, id)
				e.Swimmer[id] = s
				e.MsgEndpoint[id] = msgEndpoint
				e.ExactMember = append(e.ExactMember, *s.GetMyInfo())
				go s.Start()
				e.processLock.Unlock()
				time.Sleep(10 * time.Millisecond)
			}
			e.AppendLog("successfully create nodes " + strconv.Itoa(num))
		case "connect":
			if len(cmdList) < 3 {
				e.AppendLog("you have to input src, dst [ex : connect all 2, connect 2 6, connect {src} {dst}]")
			}

			src := cmdList[1]
			dst := cmdList[2]
			e.processLock.Lock()
			defer e.processLock.Unlock()
			if src == "all" {
				if dst == "all" {
					e.AppendLog("all to all connect is not available now")
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
					e.AppendLog("input swim dst id is invalid. input : " + dst)
				}
				e.AppendLog("successfully join " + src + " to " + dst)
			}
			if srcSwimmer, ok := e.Swimmer[src]; ok {
				if dstSwimmer, ok := e.Swimmer[dst]; ok {
					dstInfo := dstSwimmer.GetMyInfo()
					_ = srcSwimmer.Join([]string{dstInfo.TCPAddress()})
					e.AppendLog("successfully join " + src + " to " + dst)
				} else {
					e.AppendLog("input swim dst id is invalid. input : " + dst)
				}
			} else {
				e.AppendLog("input swim src id is invalid. input : " + src)
			}
		case "delete", "remove":
			if len(cmdList) < 2 {
				e.AppendLog("you have to input id to remove [ex : remove 2, delete 6]")
			}
			id := cmdList[1]
			e.processLock.Lock()
			defer e.processLock.Unlock()
			if swimmer, ok := e.Swimmer[id]; ok {
				if !swimmer.IsRun() {
					e.AppendLog("that swimmer is already die...")
				}
				swimmer.ShutDown()
				for idx, mem := range e.ExactMember {
					if mem.ID.ID == id {
						e.ExactMember = append(e.ExactMember[:idx], e.ExactMember[idx+1:]...)
						e.AppendLog("successfully remove swimmer : " + id)
					}
				}
				e.AppendLog("successfully remove swimmer : " + id)
			} else {
				e.AppendLog("there is no swimmer : " + id)
			}
		default:
			e.AppendLog("invalid cmd : " + cmdDomain)
		}
	}
}
