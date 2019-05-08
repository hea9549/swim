package main

import (
	"testing"
	"github.com/DE-labtory/swim"
	"time"

	"github.com/gizak/termui/v3/widgets"
	"sync"
)

func TestProcessCommand(t *testing.T) {
	ev := &Evaluator{
		ExactMember:    make([]swim.Member, 0),
		Swimmer:        make(map[string]*swim.SWIM),
		MsgEndpoint:    make(map[string]*swim.EvaluatorMessageEndpoint),
		lastID:         0,
		lastCheckPort:  0,
		cmdLog:         nil,
		renderLock:     sync.Mutex{},
		dataAccessLock: sync.Mutex{},
		nodeInfoCur:    0,
	}
	ev.processCommand("create 10")
	time.Sleep(time.Second)
	ev.processCommand("connect all 1")
	//c := 0
	//for {
	//	time.Sleep(100 * time.Millisecond)
	//	c++
	//	if c == 100 {
	//		return
	//	}
	//}
	str,avgProb := processMemberStatus(ev.ExactMember, ev.Swimmer)
	print(str,avgProb)
}

func TestInsparkle(t *testing.T) {
	sparkData := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	sl1 := widgets.NewSparkline()
	sl1.Title = "In"
	sl1.Data = sparkData

	sl2 := widgets.NewSparkline()
	sl2.Title = "Out"
	sl2.Data = sparkData[5:]

	slg1 := widgets.NewSparklineGroup(sl1, sl2)
	slg1.Title = "Avg network load"
	slg1.SetRect(62, 0, 82, 8)
	//processInOutPacketSparkle(sl1,sl2,slg1)
}
