package main

import (
	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim/evaluator"
)

func main() {
	iLogger.EnableStd(false)
	ev, err := evaluator.NewEvaluator()
	if err != nil {
		return
	}
	ev.Start()
}
