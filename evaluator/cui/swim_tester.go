package main

import (
	"fmt"
	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim"
	"github.com/DE-labtory/swim/cui"
	"github.com/DE-labtory/swim/evaluator"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"math"
)

type Evaluator struct {
	ExactMember    []swim.Member
	Swimmer        map[string]*swim.SWIM
	MsgEndpoint    map[string]*swim.EvaluatorMessageEndpoint
	lastID         int
	lastCheckPort  int
	cmdLog         *widgets.List
	renderLock     sync.Mutex
	dataAccessLock sync.Mutex
	nodeInfoCur    int
}

func main() {
	iLogger.EnableStd(false)
	ev := &Evaluator{
		ExactMember:    make([]swim.Member, 0),
		Swimmer:        make(map[string]*swim.SWIM),
		MsgEndpoint:    make(map[string]*swim.EvaluatorMessageEndpoint),
		lastID:         0,
		lastCheckPort:  0,
		cmdLog:         widgets.NewList(),
		renderLock:     sync.Mutex{},
		dataAccessLock: sync.Mutex{},
		nodeInfoCur:    0,
	}

	if err := ui.Init(); err != nil {
		iLogger.Fatalf(nil, "failed to initialize termui: %v", err)
	}
	defer ui.Close()
	sparkData := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	sl1 := widgets.NewSparkline()
	sl1.Title = "In"
	sl1.Data = sparkData
	sl1.LineColor = ui.ColorRed
	sl1.MaxVal = 3

	sl2 := widgets.NewSparkline()
	sl2.Title = "Out"
	sl2.Data = sparkData
	sl2.LineColor = ui.ColorMagenta
	sl2.MaxVal = 3

	slg1 := widgets.NewSparklineGroup(sl1, sl2)
	slg1.Title = "Avg network load"
	slg1.TitleStyle.Fg = ui.ColorCyan
	slg1.SetRect(62, 0, 82, 8)

	listData := []string{
		"<total node range>",
		"1~1000",
		"<dead node list>",
		"too long item data how to handle it",
		"<dead num>",
		"24",
	}

	l := widgets.NewList()
	l.Title = "Cur Node Info"
	l.TitleStyle.Fg = ui.ColorCyan
	l.Rows = listData
	l.SetRect(62, 8, 82, 16)

	nodeInfo := cui.NewNodeStatParagraph()
	nodeInfo.SetRect(0, 0, 62, 15)
	nodeInfo.Text = fmt.Sprintf("%4dR%4dG%4dB%4dM%4dC%4dW%4dY%4dY%4dY%4dY%4dY%4dY", 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 1223)

	infoExplain := cui.NewNodeStatParagraph()
	infoExplain.SetRect(0, 15, 62, 16)
	infoExplain.Border = false
	infoExplain.Text = "G:100 B:~95 C:~90 Y:~85 M:~80 R:80~ W:Dead"

	ev.cmdLog.Title = "command log"
	ev.cmdLog.TitleStyle.Fg = ui.ColorCyan
	ev.cmdLog.Rows = []string{"", "", "", "", ""}
	ev.cmdLog.SetRect(0, 16, 82, 23)

	tb := cui.NewTextBox()
	tb.SetRect(0, 23, 82, 26)
	tb.Title = "command"
	tb.TitleStyle.Fg = ui.ColorCyan

	g := widgets.NewGauge()
	g.Title = "Last Data Cover Rate"
	g.Percent = 0
	g.SetRect(82, 0, 110, 3)
	g.BarColor = ui.ColorRed
	g.BorderStyle.Fg = ui.ColorWhite
	g.TitleStyle.Fg = ui.ColorCyan

	g2 := widgets.NewGauge()
	g2.Title = "Avg Node Sync Rate"
	g2.Percent = 10
	g2.SetRect(82, 3, 110, 6)
	g2.BarColor = ui.ColorRed
	g2.BorderStyle.Fg = ui.ColorWhite
	g2.TitleStyle.Fg = ui.ColorCyan

	g3 := widgets.NewGauge()
	g3.Title = "Custom Command Gauge"
	g3.Percent = 0
	g3.SetRect(82, 6, 110, 9)
	g3.BarColor = ui.ColorRed
	g3.BorderStyle.Fg = ui.ColorWhite
	g3.TitleStyle.Fg = ui.ColorCyan

	recentData := widgets.NewList()
	recentData.Title = "Recent change know"
	recentData.TitleStyle.Fg = ui.ColorCyan
	recentData.Rows = listData
	recentData.SetRect(82, 9, 110, 26)

	ui.Render(nodeInfo, tb, slg1, l, infoExplain, ev.cmdLog, g, g2, g3, recentData)
	uiEvents := ui.PollEvents()

	go func() {
		tick := 0
		for {
			ev.dataAccessLock.Lock()
			time.Sleep(100 * time.Millisecond)
			ev.AppendLog("1")
			str, avgProb := processMemberStatus(ev.ExactMember, ev.Swimmer, ev.nodeInfoCur)
			ev.AppendLog("2")
			nodeInfo.Text = str
			ev.Render(nodeInfo)
			ev.AppendLog("3")
			in, out := processInOutPacketSparkle(ev.MsgEndpoint)
			ev.AppendLog("4")
			slg1.Sparklines[0].Data = append(slg1.Sparklines[0].Data[1:], in)
			slg1.Sparklines[1].Data = append(slg1.Sparklines[1].Data[1:], out)
			ev.Render(slg1)
			if avgProb<0 || avgProb>100 || math.IsNaN(avgProb){
				ev.AppendLog("non detected")
			}
			g2.Percent = int(avgProb)
			ui.Render(g2)
			tick++
			if tick%40 == 0 {
				nodeInfo.Text = " "
				ev.Render(nodeInfo)
			}
			ev.dataAccessLock.Unlock()
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
			case "<Left>":
				tb.MoveCursorLeft()
			case "<Up>":
				if ev.nodeInfoCur > 0 {
					ev.nodeInfoCur -= 1
				}
			case "<Down>":
				ev.nodeInfoCur += 1

			case "<Right>":
				tb.MoveCursorRight()
			case "<Space>":
				tb.InsertText(" ")
			case "<Enter>":
				log := ev.processCommand(tb.GetRawText())
				ev.AppendLog(log)
				tb.ClearText()
			default:
				if cui.ContainsString(cui.PRINTABLE_KEYS, e.ID) {
					tb.InsertText(e.ID)
				}
			}
			ev.Render(tb)
		case ui.ResizeEvent:

		}
	}

}
func (e *Evaluator) processCommand(cmd string) string {
	cmdList := strings.Split(cmd, " ")
	if len(cmdList) < 1 {
		return "you have to input cmd"
	}
	cmdDomain := cmdList[0]
	switch cmdDomain {
	case "create":
		if len(cmdList) < 2 {
			return "you have to input create node num. [ex : create 5]"
		}

		num, err := strconv.Atoi(cmdList[1])
		if err != nil {
			return "you have to input create node number, not " + cmdList[1]
		}
		for i := 0; i < num; i++ {

			e.lastID++
			tcpPort := evaluator.GetAvailablePort(20000)
			udpPort := tcpPort
			id := strconv.Itoa(e.lastID)

			s, msgEndpoint := SetupSwim("127.0.0.1", tcpPort, udpPort, id)
			e.Swimmer[id] = s
			e.MsgEndpoint[id] = msgEndpoint
			e.ExactMember = append(e.ExactMember, *s.GetMyInfo())
			go s.Start()
			time.Sleep(50 * time.Millisecond)
		}
		return "successfully create nodes " + strconv.Itoa(num)
	case "connect":
		if len(cmdList) < 3 {
			return "you have to input src, dst [ex : connect all 2, connect 2 6, connect {src} {dst}]"
		}

		src := cmdList[1]
		dst := cmdList[2]

		if src == "all" {
			if dst == "all" {
				return "all to all connect is not available now"
			}
			if dstSwimmer, ok := e.Swimmer[dst]; ok {
				dstInfo := dstSwimmer.GetMyInfo()
				for _, srcSwimmer := range e.Swimmer {
					if srcSwimmer.GetMyInfo().ID == dstInfo.ID {
						continue
					}
					srcSwimmer.Join([]string{dstInfo.TCPAddress()})
				}
			} else {
				return "input swim dst id is invalid. input : " + dst
			}
			return "successfully join " + src + " to " + dst
		}
		if srcSwimmer, ok := e.Swimmer[src]; ok {
			if dstSwimmer, ok := e.Swimmer[dst]; ok {
				dstInfo := dstSwimmer.GetMyInfo()
				srcSwimmer.Join([]string{dstInfo.TCPAddress()})
				return "successfully join " + src + " to " + dst
			} else {
				return "input swim dst id is invalid. input : " + dst
			}
		} else {
			return "input swim src id is invalid. input : " + src
		}
	case "delete", "remove":
		if len(cmdList) < 2 {
			return "you have to input id to remove [ex : remove 2, delete 6]"
		}
		id := cmdList[1]
		if swimmer, ok := e.Swimmer[id]; ok {
			if !swimmer.IsRun() {
				return "that swimmer is already die..."
			}
			swimmer.ShutDown()
			for idx, mem := range e.ExactMember {
				if mem.ID.ID == id {
					e.ExactMember = append(e.ExactMember[:idx], e.ExactMember[idx+1:]...)
					break
				}
			}
			return "successfully remove swimmer : " + id
		} else {
			return "there is no swimmer : " + id
		}
	default:
		return "invalid cmd : " + cmdDomain
	}
}

func SetupSwim(ip string, udpPort int, tcpPort int, id string) (*swim.SWIM, *swim.EvaluatorMessageEndpoint) {
	swimConfig := swim.Config{
		MaxlocalCount: 5,
		MaxNsaCounter: 5,
		T:             500,
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
		SendTimeout:             1000 * time.Millisecond,
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

func processInOutPacketSparkle(swimmers map[string]*swim.EvaluatorMessageEndpoint) (float64, float64) {

	var totalInPacket, totalOutPacket int

	for _, m := range swimmers {
		totalInPacket += m.PopInPacketCounter()
		totalOutPacket += m.PopOutPacketCounter()
	}
	lenSwimmer := float64(len(swimmers))
	if lenSwimmer ==0{
		lenSwimmer = float64(1)
	}
	avgInPacket := float64(totalInPacket) / lenSwimmer
	avgOutPacket := float64(totalOutPacket) / lenSwimmer

	return avgInPacket, avgOutPacket
}

type IdProbStruct struct {
	id    string
	prob  float64
	isRun bool
}

func processMemberStatus(exactMembers []swim.Member, swimmers map[string]*swim.SWIM, nodeInfoCur int) (string, float64) {

	dataList := make([]IdProbStruct, 0)
	totalProb := float64(0)

	for _, swimmer := range swimmers {
		memberMap := swimmer.GetMemberMap()
		prob := compareMemberList(exactMembers, memberMap.GetMembers())
		totalProb += prob
		dataList = append(dataList, IdProbStruct{
			id:    swimmer.GetMyInfo().ID.ID,
			prob:  prob,
			isRun: swimmer.IsRun(),
		})
	}
	memLen := len(swimmers)
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
		return dataList[i].id < dataList[j].id
	})
	visualizeString := ""
	counter := 0
	for _, oneData := range dataList {
		switch {
		case !oneData.isRun:
			visualizeString += fmt.Sprintf("%4sW", oneData.id)
			break
		case oneData.prob >= float64(100):
			visualizeString += fmt.Sprintf("%4sG", oneData.id)
			break
		case oneData.prob >= float64(95):
			visualizeString += fmt.Sprintf("%4sB", oneData.id)
			break
		case oneData.prob >= float64(90):
			visualizeString += fmt.Sprintf("%4sC", oneData.id)
			break
		case oneData.prob >= float64(85):
			visualizeString += fmt.Sprintf("%4sY", oneData.id)
			break
		case oneData.prob >= float64(80):
			visualizeString += fmt.Sprintf("%4sM", oneData.id)
			break
		case oneData.prob < float64(80):
			visualizeString += fmt.Sprintf("%4sR", oneData.id)
			break
		default:
			visualizeString+=""
		}
		counter++
		if counter == 12 {
			visualizeString += "\n"
			counter = 0
		}

	}

	nodeInfoData := strings.Split(visualizeString, "\n")
	if nodeInfoCur <0{
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
		if !isInSameDataMember(oneMember, expected) {
			miss++
		}
	}
	miss--
	probability := 1 - float64(miss)/float64(correctNum)
	if probability < 0 {
		probability = 0
	}
	return probability * 100
}

func isInSameDataMember(checkMember swim.Member, memberList []swim.Member) bool {
	for _, m := range memberList {
		if m.ID == checkMember.ID && m.Status == checkMember.Status {
			return true
		}
	}
	return false

}

func (e *Evaluator) AppendLog(log string) {
	e.cmdLog.Rows = append(e.cmdLog.Rows[1:], log)
	e.Render(e.cmdLog)
}

func (e *Evaluator) Render(item ui.Drawable) {
	e.renderLock.Lock()
	defer e.renderLock.Unlock()
	ui.Render(item)
}
