package evaluator

import (
	"strconv"
	"net"
)

func GetAvailablePort(startPort int) (int, error) {
	portNumber := startPort
	for {
		strPortNumber := strconv.Itoa(portNumber)
		lis, Err := net.Listen("tcp", "127.0.0.1:"+strPortNumber)

		if Err == nil {
			lis.Close()
			return portNumber, nil
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
