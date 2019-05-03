package main

import (
	"fmt"
	"github.com/DE-labtory/iLogger"
	"github.com/DE-labtory/swim"
	"net"
	"time"
)

func beforeTester() {

	swim1 := SetupSwim("127.0.0.1", 45000, "1")
	swim2 := SetupSwim("127.0.0.1", 45001, "2")
	swim3 := SetupSwim("127.0.0.1", 45002, "3")
	swim4 := SetupSwim("127.0.0.1", 45003, "4")
	swim5 := SetupSwim("127.0.0.1", 45004, "5")
	swim6 := SetupSwim("127.0.0.1", 45005, "6")

	go swim1.Start()
	go swim2.Start()
	go swim3.Start()
	go swim4.Start()
	go swim5.Start()
	go swim6.Start()
	time.Sleep(300 * time.Millisecond)

	err := swim1.Join([]string{"127.0.0.1:55005"})
	if err != nil {
		print(err)
		return
	}

	err = swim2.Join([]string{"127.0.0.1:55005"})
	if err != nil {
		print(err)
		return
	}

	err = swim3.Join([]string{"127.0.0.1:55005"})
	if err != nil {
		print(err)
		return
	}

	err = swim4.Join([]string{"127.0.0.1:55005"})
	if err != nil {
		print(err)
		return
	}

	err = swim5.Join([]string{"127.0.0.1:55005"})
	if err != nil {
		print(err)
		return
	}

	err = swim6.Join([]string{"127.0.0.1:55005"})
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
func main() {
	beforeTester()
	//swimObjList := make([]*swim.SWIM,0)
	//for i := 0; i < 1000; i++ {
	//	swimObj := SetupSwim("127.0.0.1", 30000+i, strconv.Itoa(i))
	//	go swimObj.Start()
	//	swimObjList = append(swimObjList, swimObj)
	//}
	//time.Sleep(3*time.Second)
	//
	//for idx,sObj := range swimObjList{
	//	if idx == 0{
	//		continue
	//	}
	//	sObj.Join([]string{"127.0.0.1:30000"})
	//}
	//
	//counter := 0
	//time.Sleep(3000 * time.Millisecond)
	//
	//swimObjList[0].ShutDown()
	//for {
	//	if counter == 100 {
	//		break
	//	}
	//	s2mm := swimObjList[2].GetMemberMap()
	//	infoStr := ""
	//	for _, a := range s2mm.GetMembers() {
	//		infoStr = infoStr + "IP : " + a.Address() + ",status :" + a.Status.String() + "||"
	//	}
	//	iLogger.Info(nil, infoStr)
	//	time.Sleep(1000 * time.Millisecond)
	//	counter++
	//}
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
		EncryptionEnabled:       false,
		SendTimeout:             1000 * time.Millisecond,
		CallbackCollectInterval: time.Minute,
	}

	tcpEndpointconfig := swim.TCPMessageEndpointConfig{
		EncryptionEnabled: false,
		TCPTimeout:        2 * time.Second,
		IP:                ip,
		Port:              port + 10000,
		MyId:              swim.MemberID{ID: id},
	}
	swimObj := swim.New(&swimConfig, &suspicionConfig, messageEndpointConfig, tcpEndpointconfig, &swim.Member{
		ID:               swim.MemberID{ID: id},
		Addr:             net.ParseIP(ip),
		Port:             uint16(port),
		Status:           swim.Alive,
		LastStatusChange: time.Now(),
		Incarnation:      0,
	})

	return swimObj
}
