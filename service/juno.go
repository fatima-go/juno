//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//
// @project juno
// @author DeockJin Chung (jin.freestyle@gmail.com)
// @date 2017. 3. 7. AM 11:22
//

package service

import (
	"fmt"
	"os"
	"path/filepath"
	"throosea.com/juno/domain"
	"throosea.com/juno/web"
	"throosea.com/log"
	"time"
)

const (
	CLIPBOARD_FILE = "clipboard"
)

var junoRegisted = false

func (service *DomainService) buildEndpointUrl() string {
	return fmt.Sprintf("http://%s/%s/", service.ListenAddress, service.UrlSeed)
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

	gatewayUri := service.getGatewayAddress(VALUE_JUNO_REGIST_URL)
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

	gatewayUri := service.getGatewayAddress(VALUE_JUNO_UNREGIST_URL)
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
