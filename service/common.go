/*
 * Copyright 2023 github.com/fatima-go
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
 *
 * @project fatima-core
 * @author jin
 * @date 23. 4. 14. 오후 5:20
 */

package service

import (
	"github.com/fatima-go/fatima-core"
	log "github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/infra"
	"net"
	"strconv"
	"strings"
)

var inspector infra.SystemInspector

var remoteOperationAllowed = true
var localIpAddress = "0.0.0.0"

func PrepareService(fatimaRuntime fatima.FatimaRuntime) {
	inspector = infra.NewSystemInspector(fatimaRuntime)
	newProcessMonitor(fatimaRuntime)
	allow, err := fatimaRuntime.GetConfig().GetBool(remoteOperationAllow)
	if err == nil {
		remoteOperationAllowed = allow
	}

	localIpAddress = getDefaultIpAddress()
	v, ok := fatimaRuntime.GetConfig().GetValue(domain.PropWebServerAddress)
	if ok {
		localIpAddress = v
	}

	log.Warn("remoteOperationAllow=%s", remoteOperationAllowed)
	log.Warn("localIpAddress=%s", localIpAddress)
}

const (
	// rostop, roproc, rostart 등의 프로세스 제어 명령어의 외부 허용 여부
	remoteOperationAllow = "remote.operation.allow"
)

// IsRemoteOperationAllowed(clientIp string) bool

func (service *DomainService) IsRemoteOperationAllowed(clientIp string) bool {
	if log.IsTraceEnabled() {
		log.Trace("IsRemoteOperationAllowed. remoteOperationAllowed=%s, clientIp=[%s], localIpAddress=[%s]",
			remoteOperationAllowed, clientIp, localIpAddress)
	}

	if remoteOperationAllowed {
		return true
	}

	ipaddress := getIpaddressPart(clientIp)

	if len(ipaddress) == 0 {
		return false
	}

	return strings.Compare(ipaddress, localIpAddress) == 0
}

// getIpaddressPart get ip address part from address (maybe IP:port)
func getIpaddressPart(addr string) string {
	if len(addr) < 8 {
		return "" // 0.0.0.0
	}

	ip := addr
	sepIndex := strings.Index(addr, ":") // split port
	if sepIndex > 1 {
		ip = addr[:sepIndex]
	}
	return ip
}

// getDefaultIpAddress find local ipv4 address
func getDefaultIpAddress() string {
	// func Interfaces() ([]Interface, error)
	inf, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	var min = 100
	ordered := make(map[int]string)
	for _, v := range inf {
		if !(v.Flags&net.FlagBroadcast == net.FlagBroadcast) {
			continue
		}
		if !strings.HasPrefix(v.Name, "eth") && !strings.HasPrefix(v.Name, "en") {
			continue
		}
		addrs, _ := v.Addrs()
		if len(addrs) < 1 {
			continue
		}
		var order int
		if strings.HasPrefix(v.Name, "eth") {
			order, _ = strconv.Atoi(v.Name[3:])
		} else {
			order, _ = strconv.Atoi(v.Name[2:])
		}

		for _, addr := range addrs {
			// check the address type and if it is not a loopback the display it
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ordered[order] = ipnet.IP.String()
					if order <= min {
						min = order
					}
					break
				}
			}
		}
	}

	if len(ordered) < 1 {
		return "127.0.0.1"
	}

	return ordered[min]
}
