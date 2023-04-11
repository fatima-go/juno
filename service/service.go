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
// @date 2017. 3. 5. PM 5:24
//

package service

import (
	"fmt"
	"os"
	"strings"
	"throosea.com/fatima"
)

const (
	PROP_WEB_SERVER_ADDRESS     = "webserver.address"
	PROP_WEB_SERVER_PORT        = "webserver.port"
	PROP_GATEWAY_SERVER_ADDRESS = "gateway.address"
	PROP_GATEWAY_SERVER_PORT    = "gateway.port"
	VALUE_GATEWAY_DEFAULT_PORT  = "9190"
	VALUE_TOKEN_VALIDATION_URL  = "token/v1"
	VALUE_JUNO_REGIST_URL       = "juno/regist/v1"
	VALUE_JUNO_UNREGIST_URL     = "juno/unregist/v1"
)

type DomainService struct {
	fatimaRuntime fatima.FatimaRuntime
	ListenAddress string
	UrlSeed       string
	//ValidateToken(token string, role domain.Role) error
}

func NewDomainService(fatimaRuntime fatima.FatimaRuntime) *DomainService {
	service := DomainService{fatimaRuntime: fatimaRuntime}

	return &service
}

func (service *DomainService) getGatewayAddress(suffix string) string {
	v, ok := service.fatimaRuntime.GetConfig().GetValue(PROP_GATEWAY_SERVER_ADDRESS)
	if ok {
		addr := v
		v, ok = service.fatimaRuntime.GetConfig().GetValue(PROP_GATEWAY_SERVER_PORT)
		if !ok {
			v = VALUE_GATEWAY_DEFAULT_PORT
		}
		return fmt.Sprintf("http://%s:%s/%s", addr, v, suffix)
	}

	uri := os.Getenv(fatima.ENV_FATIMA_JUPITER_URI)
	if len(uri) == 0 {
		idx := strings.Index(service.ListenAddress, ":")
		return fmt.Sprintf("http://%s:%s/%s", service.ListenAddress[:idx], VALUE_GATEWAY_DEFAULT_PORT, suffix)
	}

	if strings.HasSuffix(uri, "/") {
		return fmt.Sprintf("%s%s", uri, suffix)
	}
	return fmt.Sprintf("%s/%s", uri, suffix)
}
