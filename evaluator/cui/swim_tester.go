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
	"github.com/DE-labtory/swim/evaluator"
)

type Evaluator struct {
	ExactMember []swim.Member
	Swimmer     []*swim.SWIM
	lastID      int
}

func main() {

	if err := ui.Init(); err != nil {
		iLogger.Fatalf(nil, "failed to initialize termui: %v", err)
	}
	defer ui.Close()
	sparkData := []float64{1, 2, 3, 4, 5, 6, 7, 8,}
	sl1 := widgets.NewSparkline()
	sl1.Title = "In"
	sl1.Data = sparkData
	sl1.LineColor = ui.ColorRed

	sl2 := widgets.NewSparkline()
	sl2.Title = "Out"
	sl2.Data = sparkData[5:]
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

	cmdLog := widgets.NewList()
	cmdLog.Rows = listData[:3]
	cmdLog.Title = "command log"
	cmdLog.TitleStyle.Fg = ui.ColorCyan
	cmdLog.SetRect(0, 16, 82, 23)

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
	g2.Percent = 50
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

	ui.Render(nodeInfo, tb, slg1, l, infoExplain, cmdLog, g, g2, g3, recentData)
	uiEvents := ui.PollEvents()

	ev := &Evaluator{
		ExactMember: make([]swim.Member,0),
		Swimmer:     make([]*swim.SWIM,0),
		lastID:      0,
	}
	ev.processCommand("create 2")
	go func() {
		for{
			time.Sleep(300*time.Millisecond)
			str := processMemberStatus(ev.ExactMember,ev.Swimmer)
			nodeInfo.Text = str
			ui.Render(nodeInfo)
		}
	}()

	for {
		e := <-uiEvents
		switch e.Type {
		case ui.KeyboardEvent:
			switch e.ID {
			case "<C-c>":
				return
			case "<Backspace>","<C-<Backspace>>":
				tb.Backspace()
			case "<Left>":
				tb.MoveCursorLeft()
			case "<Right>":
				tb.MoveCursorRight()
			case "<Space>":
				tb.InsertText(" ")
			case "<Enter>":
				ev.processCommand(tb.GetRawText())
				tb.ClearText()
			default:
				if cui.ContainsString(cui.PRINTABLE_KEYS, e.ID) {
					tb.InsertText(e.ID)
				}
			}
			ui.Render(tb)
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
			return "you have to input create node num"
		}

		num, err := strconv.Atoi(cmdList[1])
		if err != nil {
			return "you have to input create node number, not " + cmdList[1]
		}
		for i:=0;i<num;i++{
			portList := evaluator.GetAvailablePortList(40000, 2)
			e.lastID++
			id := strconv.Itoa(e.lastID)
			s := SetupSwim("127.0.0.1", portList[0], portList[1], id)
			e.Swimmer = append(e.Swimmer, s)
			e.ExactMember = append(e.ExactMember, s.GetMyInfo())
			go s.Start()
		}
		return "successfully create nodes "+strconv.Itoa(num)
	case "connect":
	default:
		return "invalid cmd : " + cmdDomain
	}

	return "unknown"
}

func before2() {

	swimObjList := make([]*swim.SWIM, 0)
	for i := 0; i < 500; i++ {
		swimObj := SetupSwim("127.0.0.1", 45000+i, 55000+i, strconv.Itoa(i))
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

func SetupSwim(ip string, udpPort int, tcpPort int, id string) *swim.SWIM {
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
	swimObj := swim.New(&swimConfig, &suspicionConfig, messageEndpointConfig, tcpEndpointconfig, &swim.Member{
		ID:               swim.MemberID{ID: id},
		Addr:             net.ParseIP(ip),
		UDPPort:          uint16(udpPort),
		TCPPort:          uint16(tcpPort),
		Status:           swim.Alive,
		LastStatusChange: time.Now(),
		Incarnation:      0,
	})

	return swimObj
}

func processInOutPacketSparkle(inSparkle *widgets.Sparkline, outSparkle *widgets.Sparkline,
	sparkleLineGroup *widgets.SparklineGroup, swimmers []swim.EvaluatorMessageEndpoint) {
	inSparkle.Data = inSparkle.Data[1:]
	outSparkle.Data = inSparkle.Data[1:]

	var totalInPacket, totalOutPacket int

	for _, m := range swimmers {
		totalInPacket += m.PopInPacketCounter()
		totalOutPacket += m.PopOutPacketCounter()
	}

	avgInPacket := float64(totalInPacket) / float64(len(swimmers))
	avgOutPacket := float64(totalOutPacket) / float64(len(swimmers))

	inSparkle.Data = append(inSparkle.Data, avgInPacket)
	outSparkle.Data = append(inSparkle.Data, avgOutPacket)

	ui.Render(sparkleLineGroup)
}
type IdProbStruct struct {
	id    string
	prob  float64
	isRun bool
}
func processMemberStatus(exactMembers []swim.Member, swimmers []*swim.SWIM) string {


	dataList := make([]IdProbStruct, 0)
	for _, swimmer := range swimmers {
		memberMap := swimmer.GetMemberMap()
		prob := compareMemberList(exactMembers, memberMap.GetMembers())
		dataList = append(dataList, IdProbStruct{
			id:    swimmer.GetMyInfo().ID.ID,
			prob:  prob,
			isRun: swimmer.IsRun(),
		})
	}

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

		}
		counter++
		if counter == 12 {
			visualizeString += "\n"
			counter = 0
		}

	}
	return visualizeString
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
