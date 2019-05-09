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
	"github.com/gizak/termui/v3/widgets"
)

type currentInfo struct {
	LiveNodeNum    int
	SuspectNodeNum int
	DeadNodeNum    int
}
type selectedInfo struct {
	LiveNodeNum     int
	SuspectNodeNum  int
	DeadNodeNum     int
	DeadNodeList    []swim.MemberID
	SuspectNodeList []swim.MemberID
}

type networkLoadInfo struct {
	avgInMsg  int
	avgOutMsg int
}

type IdProbStruct struct {
	Id    string
	Prob  float64
	IsRun bool
}

type Visualizer struct {
	UpdateMemberDataChan       chan []IdProbStruct
	UpdateMemberViewCursorChan chan int
	UpdateGaugeViewChan        chan int
	UpdateCurrentInfoViewChan  chan currentInfo
	UpdateSelectedInfoViewChan chan selectedInfo
	AppendNetworkLoadChan      chan networkLoadInfo
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
}

func NewVisualizer() (*Visualizer, error) {
	// first, init termui
	err := ui.Init()
	if err != nil {
		return nil, err
	}

	vis := &Visualizer{
		UpdateMemberDataChan:       make(chan []IdProbStruct),
		UpdateMemberViewCursorChan: make(chan int),
		UpdateGaugeViewChan:        make(chan int),
		UpdateCurrentInfoViewChan:  make(chan currentInfo),
		UpdateSelectedInfoViewChan: make(chan selectedInfo),
		AppendNetworkLoadChan:      make(chan networkLoadInfo),
		UpdateTimePeriod:           make(chan int),
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
	vis.SelectedNodeInfoView.Rows = []string{"", "", "", "", "", "",}
	vis.SelectedNodeInfoView.SetRect(82, 9, 110, 26)

	ui.Render(vis.CmdLog, vis.CmdTextBox,vis.NetworkLoadSparkGroup, vis.CurNodeInfo,vis.SelectedNodeInfoView,
		vis.MemberStatusView,vis.AvgNodeSyncGauge, vis.LastDataCoverRate,vis.CustomCmdGauge)
	return vis,nil
}

func (v *Visualizer) Shutdown() {
	ui.Close()
}
