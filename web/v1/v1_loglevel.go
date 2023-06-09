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
	"github.com/fatima-go/juno/web"
	"io"
	"net/http"
)

func displayLogLevels(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	loglevel := controller.GetLogLevels()

	b, err := json.Marshal(loglevel)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func changeLogLevel(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	b, err := io.ReadAll(req.Body)
	if err != nil {
		log.Warn("fail to read request data : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	params := make(map[string]string)
	err = json.Unmarshal(b, &params)
	if err != nil {
		log.Warn("invalid read request data : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	var proc, loglevel string
	var ok bool
	proc, ok = params["process"]
	if !ok {
		log.Warn("invalid request json : not found process")
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}
	loglevel, ok = params["loglevel"]
	if !ok {
		log.Warn("invalid request json : not found loglevel")
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	report := controller.ChangeLogLevel(proc, loglevel)
	b, err = json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}
