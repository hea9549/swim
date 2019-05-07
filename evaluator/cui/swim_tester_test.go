package main

import (
	"fmt"
	"github.com/DE-labtory/swim/evaluator"
	"strconv"
	"testing"
	"time"
)

func TestEvaluator_AppendLog(t *testing.T) {
	s,_ := SetupSwim("127.0.0.1",4000,4001,"1")
	s2,_ := SetupSwim("127.0.0.1",4001,4002,"2")
	go s.Start()
	go s2.Start()
	time.Sleep(time.Second)
	err := s.Join([]string{"127.0.0.1:4002"})
	time.Sleep(time.Second)
	s1m := s.GetMemberMap()
	s2m := s2.GetMemberMap()
	fmt.Println(s1m.GetMembers())
	fmt.Println(s2m.GetMembers())
	if err != nil{
		fmt.Println(err.Error())
	}
	p,_ := evaluator.GetAvailablePort(4000)
	fmt.Println(strconv.Itoa(p))

}