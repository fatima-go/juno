/*
 * Copyright 2026 github.com/fatima-go
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
 */

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestClipRoutesSplitByContentType(t *testing.T) {
	handler := &clipRouteHandlerStub{}
	webService.Regist(handler)
	defer delete(webService.versions, handler.GetVersion())

	router := mux.NewRouter().StrictSlash(true)
	webService.GenerateSubRouter(router)
	url := "/" + webService.urlSeed + "/clip/" + handler.GetVersion()

	tests := []struct {
		contentType string
		expected    string
	}{
		{contentType: "application/json;charset=UTF-8", expected: "clip"},
		{contentType: HeaderValueBinaryContentType, expected: "upload"},
		{contentType: "Application/Octet-Stream; charset=binary", expected: "upload"},
		{contentType: "application/octet-streamx", expected: ""},
	}
	for _, test := range tests {
		handler.called = ""
		req := httptest.NewRequest(http.MethodPost, url, strings.NewReader("{}"))
		req.Header.Set(HeaderContentType, test.contentType)
		res := httptest.NewRecorder()

		router.ServeHTTP(res, req)

		assert.Equal(t, test.expected, handler.called, test.contentType)
	}
}

type clipRouteHandlerStub struct {
	WebServiceHandler
	called string
}

func (handler *clipRouteHandlerStub) GetVersion() string {
	return "vtest"
}

func (handler *clipRouteHandlerStub) HandleClip(http.ResponseWriter, *http.Request) {
	handler.called = "clip"
}

func (handler *clipRouteHandlerStub) HandleClipUpload(http.ResponseWriter, *http.Request) {
	handler.called = "upload"
}
