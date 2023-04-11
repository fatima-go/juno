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
// @date 2017. 3. 5. PM 5:30
//

package web

import (
	"github.com/gorilla/mux"
	"net/http"
	"throosea.com/fatima/lib"
	"throosea.com/log"
)

type WebServiceHandler interface {
	GetVersion() string
	HandlePackage(method string, res http.ResponseWriter, req *http.Request)
	HandleLogLevel(method string, res http.ResponseWriter, req *http.Request)
	HandleProcess(method string, res http.ResponseWriter, req *http.Request)
	HandleCron(method string, res http.ResponseWriter, req *http.Request)
	HandleDeploy(res http.ResponseWriter, req *http.Request)
	HandleClip(res http.ResponseWriter, req *http.Request)
}

type WebService struct {
	versions map[string]WebServiceHandler
	urlSeed  string
}

var webService *WebService

func init() {
	webService = new(WebService)
	webService.versions = make(map[string]WebServiceHandler, 0)
	webService.urlSeed = lib.RandomAlphanumeric(8)
}

func GetWebService() *WebService {
	return webService
}

func (webservice *WebService) GetUrlSeed() string {
	return webservice.urlSeed
}

func (webservice *WebService) Regist(service WebServiceHandler) {
	webservice.versions[service.GetVersion()] = service
}

func (webservice *WebService) GenerateSubRouter(router *mux.Router) {
	log.Info("using url seed : %s", webservice.urlSeed)

	router.PathPrefix("/").
		Methods("OPTIONS").
		Handler(webservice)

	subrouter := router.PathPrefix("/"+webService.urlSeed+"/package").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()
	subrouter.HandleFunc("/{method}/{version}", webservice.Package)

	subrouter = router.PathPrefix("/"+webService.urlSeed+"/loglevel").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()
	subrouter.HandleFunc("/{method}/{version}", webservice.LogLevel)

	subrouter = router.PathPrefix("/"+webService.urlSeed+"/process").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()
	subrouter.HandleFunc("/{method}/{version}", webservice.Process)

	subrouter = router.PathPrefix("/"+webService.urlSeed+"/cron").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()
	subrouter.HandleFunc("/{method}/{version}", webservice.Cron)

	subrouter = router.PathPrefix("/"+webService.urlSeed+"/deploy").
		Methods("POST").
		HeadersRegexp("Content-Type", "multipart/*").
		Subrouter()
	subrouter.HandleFunc("/{version}", webservice.Deploy)

	subrouter = router.PathPrefix("/"+webService.urlSeed+"/clip").
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json*").
		Subrouter()

	subrouter.HandleFunc("/{version}", webservice.Clip)
}

func (handler *WebService) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	writeCORSResponse(res, req)
}

// var AccessControlAllowHeaderList = "Content-Type, Access-Control-Allow-Headers, Authorization, Fatima-Auth-Token, Fatima-Timezone"
var AccessControlAllowHeaderList = "Content-Type, Fatima-Auth-Token, Fatima-Timezone, Fatima-Response-Time"
var AccessControlExposeHeaderList = "Content-Type, Fatima-Timezone, Fatima-Response-Time"

func writeCORSResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(HeaderAccessControlAllowOrigin, "*")
	res.Header().Set(HeaderAccessControlAllowMethods, "POST, GET, OPTIONS")
	res.Header().Set(HeaderAccessControlMaxAge, "86400")
	res.Header().Set(HeaderAccessControlAllowHeaders, AccessControlAllowHeaderList)
	res.Header().Set(HeaderAccessControlExposeHeaders, AccessControlExposeHeaderList)
	//res.Header().Add("Vary", "Origin")
	//res.Header().Add("Vary", "Access-Control-Request-Method")
	//res.Header().Add("Vary", "Access-Control-Request-Headers")
	res.WriteHeader(http.StatusOK)
}

func (webservice *WebService) inspectUrl(res http.ResponseWriter, req *http.Request) (WebServiceHandler, string) {
	var method string
	var ok bool

	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return nil, ""
	}

	service := webservice.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return nil, ""
	}

	method, ok = vars["method"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "resouce path not found")
		return nil, ""
	}

	return service, method
}

func (webservice *WebService) Package(res http.ResponseWriter, req *http.Request) {
	service, method := webService.inspectUrl(res, req)
	if len(method) >= 0 {
		service.HandlePackage(method, res, req)
	}
}

func (webservice *WebService) LogLevel(res http.ResponseWriter, req *http.Request) {
	service, method := webService.inspectUrl(res, req)
	if len(method) >= 0 {
		service.HandleLogLevel(method, res, req)
	}
}

func (webservice *WebService) Process(res http.ResponseWriter, req *http.Request) {
	service, method := webService.inspectUrl(res, req)
	if len(method) >= 0 {
		service.HandleProcess(method, res, req)
	}
}

func (webservice *WebService) Cron(res http.ResponseWriter, req *http.Request) {
	service, method := webService.inspectUrl(res, req)
	if len(method) >= 0 {
		service.HandleCron(method, res, req)
	}
}

func (webservice *WebService) Deploy(res http.ResponseWriter, req *http.Request) {
	var ok bool

	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return
	}

	service := webservice.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return
	}

	service.HandleDeploy(res, req)
}

func (handler *WebService) Clip(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	version, ok := vars["version"]
	if !ok {
		ResponseError(res, req, http.StatusBadRequest, "not found version")
		return
	}

	service := handler.versions[version]
	if service == nil {
		ResponseError(res, req, http.StatusNotImplemented, "unsupported version")
		return
	}

	service.HandleClip(res, req)
}
