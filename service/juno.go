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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/web"
)

const (
	CLIPBOARD_FILE = "clipboard"
)

var junoRegisted = false

// advertisedEndpoint keeps the first computed endpoint so unregistration uses
// the same address as registration even if the default IP changes meanwhile.
var advertisedEndpoint string

func (service *DomainService) buildEndpointUrl() string {
	if advertisedEndpoint == "" {
		advertisedEndpoint = fmt.Sprintf("http://%s/%s/", advertiseAddress(service.ListenAddress, getDefaultIpAddress()), service.UrlSeed)
	}
	return advertisedEndpoint
}

// advertiseAddress replaces a wildcard listen host (empty, 0.0.0.0, ::) with
// defaultIp because jupiter and clients cannot connect to a wildcard address.
func advertiseAddress(listen, defaultIp string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return listen
	}
	if host == "" || net.ParseIP(host).IsUnspecified() {
		host = defaultIp
	}
	return net.JoinHostPort(host, port)
}

func (service *DomainService) RegistJuno() {
	time.Sleep(1 * time.Second)

	log.Info("packaging : %s/%s/%s",
		service.fatimaRuntime.GetPackaging().GetName(),
		service.fatimaRuntime.GetPackaging().GetHost(),
		service.fatimaRuntime.GetPackaging().GetGroup())
	reg := domain.NewJunoRegistration()
	reg.GroupName = service.fatimaRuntime.GetPackaging().GetGroup()
	reg.PackageHost = service.fatimaRuntime.GetPackaging().GetHost()
	reg.PackageName = service.fatimaRuntime.GetPackaging().GetName()
	reg.Endpoint = service.buildEndpointUrl()

	gatewayUri := service.getGatewayAddress(ValueJunoRegisterUrl)
	log.Info("try to regist. gateway=%s, endpoint=%s", gatewayUri, reg.Endpoint)

	httpClient := web.NewHttpClient("")
	b, err := httpClient.Post(gatewayUri, reg)
	if err != nil {
		log.Warn("fail to regist juno : %s", err.Error())
		return
	}
	junoRegisted = true
	log.Info("response from jupiter : %s", string(b))
}

func (service *DomainService) UnregistJuno() {
	if !junoRegisted {
		return
	}

	params := make(map[string]string)
	params["endpoint"] = service.buildEndpointUrl()

	gatewayUri := service.getGatewayAddress(ValueJunoUnregisterUrl)
	log.Info("try to unregist. gateway=%s, endpoint=%s", gatewayUri, params["endpoint"])

	httpClient := web.NewHttpClient("")
	b, err := httpClient.Post(gatewayUri, params)
	if err != nil {
		log.Warn("fail to unregist juno : %s", err.Error())
		return
	}

	log.Info("response from jupiter : %s", string(b))
}

func (service *DomainService) GetClipboard() string {
	file := filepath.Join(service.fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder(), CLIPBOARD_FILE)
	dataBytes, err := os.ReadFile(file)
	if err != nil {
		return ""
	}

	return string(dataBytes)
}
