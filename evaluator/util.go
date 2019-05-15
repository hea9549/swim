package evaluator

import (
	"github.com/DE-labtory/swim"
	"net"
	"strconv"
	"time"
)

func GetAvailablePort(startPort int) int {
	portNumber := startPort
	for {
		strPortNumber := strconv.Itoa(portNumber)
		lis, Err := net.Listen("tcp", "127.0.0.1:"+strPortNumber)

		if Err == nil {
			lis.Close()
			return portNumber
		}

		portNumber++

	}
}
func GetAvailablePortList(startPort int, num int) []int {
	portNumber := startPort
	portList := make([]int, 0)
	for {
		strPortNumber := strconv.Itoa(portNumber)
		lis, Err := net.Listen("tcp", "127.0.0.1:"+strPortNumber)

		if Err == nil {
			portList = append(portList, portNumber)
			lis.Close()
			if len(portList) == num {
				return portList
			}
		}

		portNumber++

	}
}

type SwimmerConfig struct {
	ID                 string
	IP                 string
	UDPPort            int
	TCPPort            int
	ProbePeriodMillSec int // 800
	AckTimeoutMillSec  int // 200
	IndProbeNum        int // 3
	MaxLocalCount      int // 5
	MaxNsaCount        int // 5
	SuspicionMaxNum    int // 3
	SuspicionMinParam  int // 5
	SuspicionMaxParam  int // 1
	TCPTimeoutMillSec  int // 2000
}

func GetDefaultSwimmerConfig(id string,ip string, tcpPort int, udpPort int) SwimmerConfig {
	return SwimmerConfig{
		ID:                 id,
		IP:                 ip,
		UDPPort:            udpPort,
		TCPPort:            tcpPort,
		ProbePeriodMillSec: 700,
		AckTimeoutMillSec:  200,
		IndProbeNum:        1,
		MaxLocalCount:      5,
		MaxNsaCount:        5,
		SuspicionMaxNum:    2,
		SuspicionMinParam:  5,
		SuspicionMaxParam:  1,
		TCPTimeoutMillSec:  2000,
	}
}

func SetupSwim(config SwimmerConfig) (*swim.SWIM, *swim.EvaluatorMessageEndpoint) {
	swimConfig := swim.Config{
		MaxlocalCount: config.MaxLocalCount,
		MaxNsaCounter: config.MaxNsaCount,
		T:             config.ProbePeriodMillSec,
		AckTimeOut:    config.AckTimeoutMillSec,
		K:             config.IndProbeNum,
		BindAddress:   config.IP,
		BindPort:      config.UDPPort,
	}
	suspicionConfig := swim.SuspicionConfig{
		K:        config.SuspicionMaxNum,
		MinParam: config.SuspicionMinParam,
		MaxParam: config.SuspicionMaxParam,
	}
	messageEndpointConfig := swim.MessageEndpointConfig{
		EncryptionEnabled:       false,
		SendTimeout:             time.Duration(config.AckTimeoutMillSec) * time.Millisecond,
		CallbackCollectInterval: time.Minute,
	}

	tcpEndpointconfig := swim.TCPMessageEndpointConfig{
		EncryptionEnabled: false,
		TCPTimeout:        time.Duration(config.TCPTimeoutMillSec)*time.Millisecond,
		IP:                config.IP,
		Port:              config.TCPPort,
		MyId:              swim.MemberID{ID: config.ID},
	}
	swimObj, msgEndpoint := swim.NewSwimForEvaluate(&swimConfig, &suspicionConfig, messageEndpointConfig, tcpEndpointconfig, &swim.Member{
		ID:               swim.MemberID{ID: config.ID},
		Addr:             net.ParseIP(config.IP),
		UDPPort:          uint16(config.UDPPort),
		TCPPort:          uint16(config.TCPPort),
		Status:           swim.Alive,
		LastStatusChange: time.Now(),
		Incarnation:      0,
	})

	return swimObj, msgEndpoint
}
