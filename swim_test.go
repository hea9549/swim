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
