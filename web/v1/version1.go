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

package v1

import (
	"encoding/json"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/web"
	"io"
	"net/http"
)

type HandlerFunc func(web.JunoWebServiceController, http.ResponseWriter, *http.Request)

func NewWebService(domainService web.JunoWebServiceController) web.WebServiceHandler {
	service := new(Version1Handler)
	service.controller = domainService
	return service
}

type Version1Handler struct {
	controller web.JunoWebServiceController
}

func (version1 *Version1Handler) GetVersion() string {
	return "v1"
}

func (version1 *Version1Handler) HandlePackage(method string, res http.ResponseWriter, req *http.Request) {
	switch method {
	case "dis":
		version1.secureHandle(domain.ROLE_MONITOR, res, req, displayPackage)
	case "proc":
		version1.secureHandle(domain.ROLE_MONITOR, res, req, displayProcess)
	case "health":
		healthCheck(version1.controller, res, req)
	default:
		web.ResponseError(res, req, http.StatusNotFound, "")
		return
	}
}

func (version1 *Version1Handler) HandleLogLevel(method string, res http.ResponseWriter, req *http.Request) {
	switch method {
	case "dis":
		version1.secureHandle(domain.ROLE_MONITOR, res, req, displayLogLevels)
	case "chg":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, changeLogLevel)
	default:
		web.ResponseError(res, req, http.StatusNotFound, "")
		return
	}
}

func (version1 *Version1Handler) HandleProcess(method string, res http.ResponseWriter, req *http.Request) {
	switch method {
	case "stop":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, stopProcess)
	case "start":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, startProcess)
	case "regist":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, registProcess)
	case "unregist":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, unregistProcess)
	case "clric":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, clearIcProcess)
	case "history":
		version1.secureHandle(domain.ROLE_MONITOR, res, req, deploymentHistoryProcess)
	default:
		web.ResponseError(res, req, http.StatusNotFound, "")
		return
	}
}

func (version1 *Version1Handler) HandleCron(method string, res http.ResponseWriter, req *http.Request) {
	switch method {
	case "summary":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, summaryCronList)
	case "list":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, displayCronCommands)
	case "rerun":
		version1.secureHandle(domain.ROLE_OPERATOR, res, req, rerunCronCommand)
	default:
		web.ResponseError(res, req, http.StatusNotFound, "")
		return
	}
}

func (version1 *Version1Handler) HandleDeploy(res http.ResponseWriter, req *http.Request) {
	version1.secureHandle(domain.ROLE_OPERATOR, res, req, deployPackage)
}

func (version1 *Version1Handler) secureHandle(userRole domain.Role, res http.ResponseWriter, req *http.Request, businessHandler HandlerFunc) {
	token := req.Header.Get(domain.HEADER_FATIMA_AUTH_TOKEN)
	if len(token) < 1 {
		log.Warn("Unauthorized : not found fatima token")
		web.ResponseError(res, req, http.StatusUnauthorized, "invalid access")
		return
	}

	err := version1.controller.ValidateToken(token, userRole)
	if err != nil {
		log.Warn("authorization fail :: %s :: %s", err.Error(), token)
		web.ResponseError(res, req, http.StatusUnauthorized, "invalid access")
		return
	}

	businessHandler(version1.controller, res, req)
}

func (version1 *Version1Handler) HandleClip(res http.ResponseWriter, req *http.Request) {
	version1.secureHandle(domain.ROLE_MONITOR, res, req, clip)
}

func parsingRequest(req *http.Request) (map[string]string, error) {
	params := make(map[string]string)

	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return params, nil
	}

	err = json.Unmarshal(b, &params)
	if err != nil {
		return nil, err
	}

	return params, nil
}
