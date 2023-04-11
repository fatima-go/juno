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
// @date 2017. 3. 5. PM 6:01
//

package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

var netTransport = &http.Transport{
	Dial: (&net.Dialer{
		Timeout: 1 * time.Second,
	}).Dial,
	TLSHandshakeTimeout: 1 * time.Second,
}

var netClient = &http.Client{
	Timeout:   time.Second * 1,
	Transport: netTransport,
}

type HttpClient struct {
	token   string
	headers map[string]string
}

func NewHttpClient(token string) HttpClient {
	client := HttpClient{}
	client.token = token
	client.headers = make(map[string]string)
	return client
}

func (hc HttpClient) AddHeader(key string, value string) {
	hc.headers[key] = value
}

func (hc HttpClient) Post(url string, jsonData interface{}) ([]byte, error) {
	var body io.Reader
	body = nil
	if jsonData != nil {
		outgoingJSON, err := json.Marshal(jsonData)
		if err != nil {
			return nil, err
		}

		body = bytes.NewReader(outgoingJSON)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest("POST", url, body)

	req.Header.Add(HeaderUserAgent, HeaderValueUserAgent)
	req.Header.Add(HeaderCharset, HeaderValueCharset)
	req.Header.Add(HeaderContentType, HeaderValueContentType)
	req.Header.Add(HeaderFatimaTimezone, HeaderValueFatimaTimezone)
	if len(hc.token) > 0 {
		req.Header.Add(HeaderFatimaAuthToken, hc.token)
	}

	for k, v := range hc.headers {
		req.Header.Add(k, v)
	}

	//req.Header.Add(HeaderFatimaTokenRole, domain.ToRoleString(domain.ROLE_OPERATOR))

	var resp *http.Response
	resp, err = netClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var b []byte
	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}
