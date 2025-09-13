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
	"os"
	"strings"

	"github.com/fatima-go/fatima-core"
)

const (
	PropWebServerAddress     = "webserver.address"
	PropWebServerPort        = "webserver.port"
	PropGatewayServerAddress = "gateway.address"
	PropGatewayServerPort    = "gateway.port"
	ValueGatewayDefaultPort  = "9190"
	ValueTokenValidationUrl  = "token/v1"
	ValueJunoRegisterUrl     = "juno/regist/v1"
	ValueJunoUnregisterUrl   = "juno/unregist/v1"
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
	v, ok := service.fatimaRuntime.GetConfig().GetValue(PropGatewayServerAddress)
	if ok {
		addr := v
		v, ok = service.fatimaRuntime.GetConfig().GetValue(PropGatewayServerPort)
		if !ok {
			v = ValueGatewayDefaultPort
		}
		return fmt.Sprintf("http://%s:%s/%s", addr, v, suffix)
	}

	uri := os.Getenv(fatima.ENV_FATIMA_JUPITER_URI)
	if len(uri) == 0 {
		idx := strings.Index(service.ListenAddress, ":")
		return fmt.Sprintf("http://%s:%s/%s", service.ListenAddress[:idx], ValueGatewayDefaultPort, suffix)
	}

	if strings.HasSuffix(uri, "/") {
		return fmt.Sprintf("%s%s", uri, suffix)
	}
	return fmt.Sprintf("%s/%s", uri, suffix)
}
