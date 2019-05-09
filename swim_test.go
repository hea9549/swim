/*
 * Copyright 2018 De-labtory
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package swim

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

func TestIndPing(t *testing.T) {
	n1, _ := SetupSwim("127.0.0.1", 3000, 3000, "1")
	n2, _ := SetupSwim("127.0.0.1", 3001, 3001, "2")
	n3, _ := SetupSwim("127.0.0.1", 3002, 3002, "3")
	n4, _ := SetupSwim("127.0.0.1", 3003, 3003, "4")

	go n1.Start()
	go n2.Start()
	go n3.Start()
	go n4.Start()

	err := n2.Join([]string{"127.0.0.1:3000"})
	assert.NoError(t, err)
	err = n3.Join([]string{"127.0.0.1:3000"})
	assert.NoError(t, err)
	err = n4.Join([]string{"127.0.0.1:3000"})
	assert.NoError(t, err)

	time.Sleep(10*time.Second)
	n4.ShutDown()

	time.Sleep(10*time.Second)
	fmt.Println(n1.GetMemberMap().GetMembers())
	fmt.Println(n2.GetMemberMap().GetMembers())
	fmt.Println(n3.GetMemberMap().GetMembers())
	fmt.Println(n4.GetMemberMap().GetMembers())
}

func SetupSwim(ip string, udpPort int, tcpPort int, id string) (*SWIM, *EvaluatorMessageEndpoint) {
	swimConfig := Config{
		MaxlocalCount: 5,
		MaxNsaCounter: 5,
		T:             800,
		AckTimeOut:    100,
		K:             2,
		BindAddress:   ip,
		BindPort:      udpPort,
	}
	suspicionConfig := SuspicionConfig{
		K:        3,
		MinParam: 5,
		MaxParam: 1,
	}
	messageEndpointConfig := MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             100 * time.Millisecond,
		CallbackCollectInterval: time.Minute,
	}

	tcpEndpointconfig := TCPMessageEndpointConfig{
		EncryptionEnabled: false,
		TCPTimeout:        20 * time.Second,
		IP:                ip,
		Port:              tcpPort,
		MyId:              MemberID{ID: id},
	}
	swimObj, msgEndpoint := NewSwimForEvaluate(&swimConfig, &suspicionConfig, messageEndpointConfig, tcpEndpointconfig, &Member{
		ID:               MemberID{ID: id},
		Addr:             net.ParseIP(ip),
		UDPPort:          uint16(udpPort),
		TCPPort:          uint16(tcpPort),
		Status:           Alive,
		LastStatusChange: time.Now(),
		Incarnation:      0,
	})

	return swimObj, msgEndpoint
}

//
//import (
//	"testing"
//	"strconv"
//	"net"
//	"github.com/stretchr/testify/assert"
//)
//
//func TestSWIM_Join(t *testing.T) {
//	port, Err := GetAvailablePort()
//
//	assert.NoError(t, Err)
//
//	s1 := New(&Config{
//		K:             2,
//		T:             4000,
//		AckTimeOut:    1000,
//		MaxlocalCount: 1,
//		MaxNsaCounter: 8,
//		BindAddress:   "127.0.0.1",
//		BindPort:      port,
//	},
//		&SuspicionConfig{
//			K:   3,
//			min: ,
//			max: 0,
//		},
//		MessageEndpointConfig{
//			CallbackCollectInterval: 1000,
//		},
//		&Member{},
//	)
//
//}
//
//func GetAvailablePort() (int, error) {
//	portNumber := 5000
//	for {
//		strPortNumber := strconv.Itoa(portNumber)
//		lis, Err := net.Listen("tcp", "127.0.0.1:"+strPortNumber)
//
//		if Err == nil {
//			lis.Close()
//			return portNumber, nil
//		}
//
//		portNumber++
//
//	}
//}
