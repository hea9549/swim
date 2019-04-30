package main

import (
	"fmt"
	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim"
	"net"
	"time"
)

func main() {

	swim1 := SetupSwim("127.0.0.1", 5000, "1")
	swim2 := SetupSwim("127.0.0.1", 5001, "2")
	swim3 := SetupSwim("127.0.0.1", 5002, "3")

	go swim1.Start()
	go swim2.Start()
	go swim3.Start()
	time.Sleep(300 * time.Millisecond)

	err := swim1.Join([]string{"127.0.0.1:5001"})
	if err != nil {
		print(err)
		return
	}

	err = swim3.Join([]string{"127.0.0.1:5001"})
	if err != nil {
		print(err)
		return
	}
	s1m := swim1.GetMemberMap()
	s2m := swim2.GetMemberMap()
	s3m := swim3.GetMemberMap()

	fmt.Println(s1m)
	fmt.Println(s2m)
	fmt.Println(s3m)
	time.Sleep(3000 * time.Millisecond)
	swim1.ShutDown()
	iLogger.Warn(nil, "[Tester] SWIM1 is shutted down")

	counter := 0
	time.Sleep(300 * time.Millisecond)

	for {
		if counter == 100 {
			break
		}
		s2mm := swim2.GetMemberMap()
		infoStr := ""
		for _, a := range s2mm.GetMembers() {
			infoStr = infoStr + "IP : " + a.Address() + ",status :" + a.Status.String() + "||"
		}
		iLogger.Info(nil, infoStr)
		time.Sleep(1000 * time.Millisecond)
		counter++
	}
}

func SetupSwim(ip string, port int, id string) *swim.SWIM {
	swimConfig := swim.Config{
		MaxlocalCount: 5,
		MaxNsaCounter: 5,
		T:             4000,
		AckTimeOut:    1000,
		K:             2,
		BindAddress:   ip,
		BindPort:      port,
	}
	suspicionConfig := swim.SuspicionConfig{
		K:        3,
		MinParam: 5,
		MaxParam: 1,
	}
	messageEndpointConfig := swim.MessageEndpointConfig{
		ID:                      id,
		EncryptionEnabled:       false,
		SendTimeout:             3000 * time.Millisecond,
		CallbackCollectInterval: time.Minute,
	}
	swimObj := swim.New(&swimConfig, &suspicionConfig, messageEndpointConfig, &swim.Member{
		ID:               swim.MemberID{ID: id},
		Addr:             net.ParseIP(ip),
		Port:             uint16(port),
		Status:           swim.Alive,
		LastStatusChange: time.Now(),
		Incarnation:      0,
	})

	return swimObj
}
