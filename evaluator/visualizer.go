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
	"fmt"
	"github.com/DE-labtory/swim/cui"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"sort"
	"strconv"
	"strings"
)

type currentInfo struct {
	LiveNodeNum    int
	SuspectNodeNum int
	DeadNodeNum    int
}
type selectedInfo struct {
	ID              string
	LiveNodeNum     int
	SuspectNodeNum  int
	DeadNodeNum     int
	DeadNodeList    []string
	SuspectNodeList []string
	Incarnation     int
	AwarenessScore  int
}

type networkLoadInfo struct {
	avgInMsg  float64
	avgOutMsg float64
}

type IdProbStruct struct {
	Id    string
	Prob  float64
	IsRun bool
}
type CursorEvent int

const (
	UP   CursorEvent = 1
	DOWN CursorEvent = 2
)

type Visualizer struct {
	UpdateMemberDataChan       chan []IdProbStruct
	UpdateMemberViewCursorChan chan CursorEvent
	UpdateGaugeViewChan        chan int
	UpdateCurrentInfoViewChan  chan currentInfo
	UpdateSelectedInfoViewChan chan selectedInfo
	AppendNetworkLoadChan      chan networkLoadInfo
	AppendLogChan              chan string
	UpdateTimePeriod           chan int

	CmdLog                *widgets.List
	CmdTextBox            *cui.TextBox
	AvgInLoadSparkLine    *cui.Sparkline
	AvgOutLoadSparkLine   *cui.Sparkline
	NetworkLoadSparkGroup *cui.SparklineGroup
	CurNodeInfo           *widgets.List
	SelectedNodeInfoView  *widgets.List
	MemberStatusView      *cui.NodeStatParagraph
	AvgNodeSyncGauge      *widgets.Gauge
	LastDataCoverRate     *widgets.Gauge
	CustomCmdGauge        *widgets.Gauge

	toStop           bool
	stopCh           chan interface{}
	memberViewCursor int
}

func NewVisualizer() (*Visualizer, error) {
	// first, init termui
	err := ui.Init()
	if err != nil {
		return nil, err
	}

	vis := &Visualizer{
		UpdateMemberDataChan:       make(chan []IdProbStruct),
		UpdateMemberViewCursorChan: make(chan CursorEvent),
		UpdateGaugeViewChan:        make(chan int),
		UpdateCurrentInfoViewChan:  make(chan currentInfo),
		UpdateSelectedInfoViewChan: make(chan selectedInfo),
		AppendNetworkLoadChan:      make(chan networkLoadInfo),
		UpdateTimePeriod:           make(chan int),
		AppendLogChan:              make(chan string),
		stopCh:                     make(chan interface{}),
		CmdLog:                     widgets.NewList(),
		CmdTextBox:                 cui.NewTextBox(),
		AvgInLoadSparkLine:         cui.NewSparkline(),
		AvgOutLoadSparkLine:        cui.NewSparkline(),
		NetworkLoadSparkGroup:      nil, // init in below
		CurNodeInfo:                widgets.NewList(),
		SelectedNodeInfoView:       widgets.NewList(),
		MemberStatusView:           cui.NewNodeStatParagraph(),
		AvgNodeSyncGauge:           widgets.NewGauge(),
		LastDataCoverRate:          widgets.NewGauge(),
		CustomCmdGauge:             widgets.NewGauge(),
		toStop:                     false,
		memberViewCursor:           0,
	}

	sparkLineInitData := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,}
	vis.AvgInLoadSparkLine.Title = "In"
	vis.AvgInLoadSparkLine.Data = sparkLineInitData
	vis.AvgInLoadSparkLine.LineColor = ui.ColorRed
	vis.AvgInLoadSparkLine.MaxVal = float64(8)

	vis.AvgOutLoadSparkLine.Title = "Out"
	vis.AvgOutLoadSparkLine.Data = sparkLineInitData
	vis.AvgOutLoadSparkLine.LineColor = ui.ColorMagenta
	vis.AvgOutLoadSparkLine.MaxVal = float64(8)

	vis.NetworkLoadSparkGroup = cui.NewSparklineGroup(vis.AvgInLoadSparkLine, vis.AvgOutLoadSparkLine)
	vis.NetworkLoadSparkGroup.Title = "Avg network load"
	vis.NetworkLoadSparkGroup.TitleStyle.Fg = ui.ColorCyan
	vis.NetworkLoadSparkGroup.SetRect(62, 0, 82, 6)

	vis.CurNodeInfo.Title = "Cur Node Info"
	vis.CurNodeInfo.TitleStyle.Fg = ui.ColorCyan
	vis.CurNodeInfo.Rows = []string{"", "", "", "", ""}
	vis.CurNodeInfo.SetRect(62, 8, 82, 16)

	vis.MemberStatusView.SetRect(0, 0, 62, 15)

	infoExplain := cui.NewNodeStatParagraph()
	infoExplain.SetRect(0, 15, 62, 16)
	infoExplain.Border = false
	infoExplain.Text = "G:100 B:~95 C:~90 Y:~85 M:~80 R:80~ W:Dead"

	vis.CmdLog.Title = "command log"
	vis.CmdLog.TitleStyle.Fg = ui.ColorCyan
	vis.CmdLog.Rows = []string{"", "", "", "", ""}
	vis.CmdLog.SetRect(0, 16, 82, 23)

	vis.CmdTextBox.SetRect(0, 23, 82, 26)
	vis.CmdTextBox.Title = "command"
	vis.CmdTextBox.TitleStyle.Fg = ui.ColorCyan

	vis.LastDataCoverRate.Title = "Last Data Cover Rate"
	vis.LastDataCoverRate.Percent = 0
	vis.LastDataCoverRate.SetRect(82, 0, 110, 3)
	vis.LastDataCoverRate.BarColor = ui.ColorRed
	vis.LastDataCoverRate.BorderStyle.Fg = ui.ColorWhite
	vis.LastDataCoverRate.TitleStyle.Fg = ui.ColorCyan

	vis.AvgNodeSyncGauge.Title = "Avg Node Sync Rate"
	vis.AvgNodeSyncGauge.Percent = 10
	vis.AvgNodeSyncGauge.SetRect(82, 3, 110, 6)
	vis.AvgNodeSyncGauge.BarColor = ui.ColorRed
	vis.AvgNodeSyncGauge.BorderStyle.Fg = ui.ColorWhite
	vis.AvgNodeSyncGauge.TitleStyle.Fg = ui.ColorCyan

	vis.CustomCmdGauge.Title = "Custom Command Gauge"
	vis.CustomCmdGauge.Percent = 0
	vis.CustomCmdGauge.SetRect(82, 6, 110, 9)
	vis.CustomCmdGauge.BarColor = ui.ColorRed
	vis.CustomCmdGauge.BorderStyle.Fg = ui.ColorWhite
	vis.CustomCmdGauge.TitleStyle.Fg = ui.ColorCyan

	vis.SelectedNodeInfoView.Title = "Node View"
	vis.SelectedNodeInfoView.TitleStyle.Fg = ui.ColorCyan
	vis.SelectedNodeInfoView.Rows = []string{"", "", "", "", "", "", "",}
	vis.SelectedNodeInfoView.SetRect(82, 9, 110, 26)

	ui.Render(vis.CmdLog, vis.CmdTextBox, vis.NetworkLoadSparkGroup, vis.CurNodeInfo, vis.SelectedNodeInfoView,
		vis.MemberStatusView, vis.AvgNodeSyncGauge, vis.LastDataCoverRate, vis.CustomCmdGauge, infoExplain)
	return vis, nil
}
func (v *Visualizer) Start() {
	v.toStop = false
	for {
		if v.toStop {
			return
		}
		select {
		case d := <-v.UpdateMemberDataChan:
			viewStr, avgGuage := getMemberViewData(d, v.memberViewCursor)
			v.MemberStatusView.Text = viewStr
			v.AvgNodeSyncGauge.Percent = int(avgGuage)
			ui.Render(v.MemberStatusView, v.AvgNodeSyncGauge)
			break
		case c := <-v.UpdateMemberViewCursorChan:
			if c == UP {
				v.memberViewCursor--
			} else if c == DOWN {
				v.memberViewCursor++
			}
			break
		case <-v.UpdateGaugeViewChan:
			// not do yet
			break
		case <-v.UpdateCurrentInfoViewChan:
			break
		case d := <-v.UpdateSelectedInfoViewChan:
			v.SelectedNodeInfoView.Rows[0] = "ID : " + d.ID
			v.SelectedNodeInfoView.Rows[1] = "ALIVE NUM : " + strconv.Itoa(d.LiveNodeNum)
			v.SelectedNodeInfoView.Rows[2] = "SUSPECT NUM : " + strconv.Itoa(d.SuspectNodeNum)
			v.SelectedNodeInfoView.Rows[3] = "DEAD NUM : " + strconv.Itoa(d.DeadNodeNum)
			v.SelectedNodeInfoView.Rows[4] = "SUSPECT : " + strings.Join(d.SuspectNodeList, ",")
			v.SelectedNodeInfoView.Rows[5] = "DEAD : " + strings.Join(d.DeadNodeList, ",")
			v.SelectedNodeInfoView.Rows[6] = "INCARNATION : " + strconv.Itoa(d.Incarnation)
			ui.Render(v.SelectedNodeInfoView)
			break
		case d := <-v.AppendNetworkLoadChan:
			v.NetworkLoadSparkGroup.Sparklines[0].Data = append(v.NetworkLoadSparkGroup.Sparklines[0].Data[1:], d.avgInMsg)
			v.NetworkLoadSparkGroup.Sparklines[1].Data = append(v.NetworkLoadSparkGroup.Sparklines[1].Data[1:], d.avgOutMsg)
			ui.Render(v.NetworkLoadSparkGroup)
			break
		case <-v.UpdateTimePeriod:
			break
		case <-v.stopCh:
			ui.Close()
			return
		case log := <-v.AppendLogChan:
			v.CmdLog.Rows = append(v.CmdLog.Rows[1:], log)
			ui.Render(v.CmdLog)
			break
		}
	}

}

func getMemberViewData(dataList []IdProbStruct, cursor int) (string, float64) {
	totalProb := float64(0)

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
	aliveNum := 0
	for _, oneData := range dataList {
		aliveNum++
		switch {
		case !oneData.IsRun:
			visualizeString += fmt.Sprintf("%4sW", oneData.Id)
			aliveNum--
			break
		case oneData.Prob >= float64(100):
			visualizeString += fmt.Sprintf("%4sG", oneData.Id)
			totalProb += oneData.Prob
			break
		case oneData.Prob >= float64(95):
			visualizeString += fmt.Sprintf("%4sB", oneData.Id)
			totalProb += oneData.Prob
			break
		case oneData.Prob >= float64(90):
			visualizeString += fmt.Sprintf("%4sC", oneData.Id)
			totalProb += oneData.Prob
			break
		case oneData.Prob >= float64(85):
			visualizeString += fmt.Sprintf("%4sY", oneData.Id)
			totalProb += oneData.Prob
			break
		case oneData.Prob >= float64(80):
			visualizeString += fmt.Sprintf("%4sM", oneData.Id)
			totalProb += oneData.Prob
			break
		case oneData.Prob < float64(80):
			visualizeString += fmt.Sprintf("%4sR", oneData.Id)
			totalProb += oneData.Prob
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
	if cursor < 0 {
		cursor = 0
	}
	if cursor < len(nodeInfoData) {
		nodeInfoData = nodeInfoData[cursor:]
	}

	visualizeString = strings.Join(nodeInfoData[:], "\n")
	if aliveNum == 0 {
		aliveNum = 1
	}
	avgProb := totalProb / float64(aliveNum)
	if avgProb < 0 {
		avgProb = float64(0)
	}
	if avgProb > 100 {
		avgProb = float64(100)
	}
	return visualizeString, avgProb
}

func (v *Visualizer) HandleCmdTextBox(str string) string {
	returnStr := ""
	switch str {
	case "<Backspace>", "<C-<Backspace>>":
		v.CmdTextBox.Backspace()
		break
	case "<Left>":
		v.CmdTextBox.MoveCursorLeft()
		break
	case "<Right>":
		v.CmdTextBox.MoveCursorRight()
		break
	case "<Space>":
		v.CmdTextBox.InsertText(" ")
		break
	case "<Enter>":
		returnStr = v.CmdTextBox.GetText()
		v.CmdTextBox.ClearText()
		break
	default:
		if cui.ContainsString(cui.PRINTABLE_KEYS, str) {
			v.CmdTextBox.InsertText(str)
		}
		break

	}
	ui.Render(v.CmdTextBox)
	return returnStr
}

func (v *Visualizer) Shutdown() {
	v.stopCh <- struct{}{}
	v.toStop = true

}
