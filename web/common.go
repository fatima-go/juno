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

package web

import (
	"encoding/json"
	"fmt"
	"github.com/fatima-go/fatima-log"
	"net/http"
	"time"
)

const (
	HeaderAccessControlAllowOrigin   = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods  = "Access-Control-Allow-Methods"
	HeaderAccessControlMaxAge        = "Access-Control-Max-Age"
	HeaderAccessControlAllowHeaders  = "Access-Control-Allow-Headers"
	HeaderAccessControlExposeHeaders = "Access-Control-Expose-Headers"
	HeaderFatimaAuthToken            = "Fatima-Auth-Token"
	HeaderContentType                = "Content-Type"
	HeaderCharset                    = "Charset"
	HeaderUserAgent                  = "User-Agent"
	HeaderFatimaTimezone             = "Fatima-Timezone"
	HeaderFatimaResTime              = "Fatima-Response-Time"
	HeaderFatimaTokenRole            = "Fatima-Token-Role"

	HeaderValueUserAgent      = "fatima-application-juno"
	HeaderValueCharset        = "UTF-8"
	HeaderValueContentType    = "application/json; charset=utf-8"
	HeaderValueFatimaTimezone = "Asia/Seoul"

	TIME_YYYYMMDDHHMMSS = "2006-01-02 15:04:05"
)

type ServerError struct {
	Message string `json:"message"`
}

type SystemResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func GetFatimaAuthToken(req *http.Request) string {
	if token, ok := req.Header[HeaderFatimaAuthToken]; ok {
		return token[0]
	}

	return ""
}

func GetFatimaClientTimezone(req *http.Request) *time.Location {
	if tz, ok := req.Header[HeaderFatimaTimezone]; ok {
		if loc, err := time.LoadLocation(tz[0]); err == nil {
			return loc
		}
	}

	// default
	return time.UTC
}

func writeResponseHeader(res http.ResponseWriter, req *http.Request, httpStatusCode int) {
	res.Header().Set(HeaderAccessControlAllowOrigin, "*")
	res.Header().Set(HeaderAccessControlAllowHeaders, AccessControlAllowHeaderList)
	res.Header().Set(HeaderAccessControlExposeHeaders, AccessControlExposeHeaderList)
	res.Header().Set(HeaderContentType, HeaderValueContentType)
	res.Header().Set(HeaderCharset, HeaderValueCharset)
	res.Header().Set(HeaderUserAgent, HeaderValueUserAgent)
	if tz, ok := req.Header[HeaderFatimaTimezone]; ok {
		if loc, err := time.LoadLocation(tz[0]); err == nil {
			res.Header().Set(HeaderFatimaTimezone, tz[0])
			res.Header().Set(HeaderFatimaResTime, time.Now().In(loc).Format(TIME_YYYYMMDDHHMMSS))
		}
	}
	res.WriteHeader(httpStatusCode)
}

func WriteSystemSuccess(res http.ResponseWriter, req *http.Request, message string) {
	// {'system': {'message': 'not found process : benefita1p', 'code': 700}}
	response := make(map[string]SystemResponse)
	response["system"] = SystemResponse{Code: 200, Message: message}
	b, err := json.Marshal(response)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(res, req, string(b))
}

func WriteSystemError(res http.ResponseWriter, req *http.Request, message string) {
	// {'system': {'message': 'not found process : benefita1p', 'code': 700}}
	response := make(map[string]SystemResponse)
	response["system"] = SystemResponse{Code: 700, Message: message}
	b, err := json.Marshal(response)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	ResponseSuccess(res, req, string(b))
}

func ResponseSuccess(res http.ResponseWriter, req *http.Request, message string) {
	writeResponseHeader(res, req, http.StatusOK)
	if len(message) > 0 {
		fmt.Fprintln(res, message)
	}
}

func ResponseError(res http.ResponseWriter, req *http.Request, httpStatusCode int, message string) {
	writeResponseHeader(res, req, httpStatusCode)

	if len(message) > 0 {
		errorResponse := ServerError{message}
		outgoingJSON, err := json.Marshal(errorResponse)
		if err != nil {
			log.Warn("fail to make response", err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(res, string(outgoingJSON))
	}
}
