package main

import (
	"fmt"
	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim"
	"net"
	"time"
	"github.com/DE-labtory/swim/cui"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"strconv"
	"strings"
	"sort"
	"sync"
)

type Evaluator struct {
	ExactMember []swim.Member
	Swimmer     map[string]*swim.SWIM
	MsgEndpoint map[string]*swim.EvaluatorMessageEndpoint
	lastID      int
	cmdLog      *widgets.List
	renderLock  sync.Mutex
}

func main() {

	ev := &Evaluator{
		ExactMember: make([]swim.Member, 0),
		Swimmer:     make(map[string]*swim.SWIM),
		MsgEndpoint: make(map[string]*swim.EvaluatorMessageEndpoint),
		lastID:      0,
		cmdLog:      widgets.NewList(),
		renderLock:  sync.Mutex{},
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

	sl2 := widgets.NewSparkline()
	sl2.Title = "Out"
	sl2.Data = sparkData
	sl2.LineColor = ui.ColorMagenta

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
	g.Percent = 50
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
	g3.Percent = 50
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
			time.Sleep(50 * time.Millisecond)
			str, _ := processMemberStatus(ev.ExactMember, ev.Swimmer)
			nodeInfo.Text = str
			//g2.Percent = int(avgProb)
			ev.Render(nodeInfo)
			//ui.Render(g2)
			tick++
			if tick%40 == 0 {
				nodeInfo.Text = " "
				ev.Render(nodeInfo)
			}

		}
	}()

	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			in, out := processInOutPacketSparkle(ev.MsgEndpoint)
			slg1.Sparklines[0].Data = append(slg1.Sparklines[0].Data[1:], in)
			slg1.Sparklines[1].Data = append(slg1.Sparklines[1].Data[1:], out)
			ev.Render(slg1)

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
			tcpPort := 20000 + e.lastID*2
			udpPort := 20000 + e.lastID*2 + 1
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
	default:
		return "invalid cmd : " + cmdDomain
	}

	return "unknown"
}

func before2() {

	swimObjList := make([]*swim.SWIM, 0)
	for i := 0; i < 500; i++ {
		swimObj, _ := SetupSwim("127.0.0.1", 45000+i, 55000+i, strconv.Itoa(i))
		go swimObj.Start()
		swimObjList = append(swimObjList, swimObj)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(3 * time.Second)

	for idx, sObj := range swimObjList {
		if idx == 0 {
			continue
		}
		sObj.Join([]string{"127.0.0.1:55000"})
		time.Sleep(10 * time.Millisecond)
	}

	counter := 0
	time.Sleep(3000 * time.Millisecond)

	startTime := time.Now()
	for {
		s2mm := swimObjList[2].GetMemberMap()
		passTime := fmt.Sprintf("%.2f", time.Now().Sub(startTime).Seconds())

		infoStr := "time pass: " + passTime + "s, I know : " + strconv.Itoa(len(s2mm.GetMembers())) + ","
		for _, a := range s2mm.GetMembers() {
			infoStr = infoStr + "IP : " + a.UDPAddress() + ",status :" + a.Status.String() + "||"
		}
		iLogger.Info(nil, infoStr)
		time.Sleep(1000 * time.Millisecond)
		counter++
		if counter == 30 {
			swimObjList[50].ShutDown()
		}
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

	avgInPacket := float64(totalInPacket) / float64(len(swimmers))
	avgOutPacket := float64(totalOutPacket) / float64(len(swimmers))

	return avgInPacket, avgOutPacket
}

type IdProbStruct struct {
	id    string
	prob  float64
	isRun bool
}

func processMemberStatus(exactMembers []swim.Member, swimmers map[string]*swim.SWIM) (string, float64) {

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
	avgProb := totalProb / float64(len(swimmers))
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
		case oneData.prob >= float64(99):
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
			

		}
		counter++
		if counter == 12 {
			visualizeString += "\n"
			counter = 0
		}

	}
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
