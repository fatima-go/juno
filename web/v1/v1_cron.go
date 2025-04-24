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
	"net/http"
)

func summaryCronList(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	report := controller.SummaryCronList()
	b, err := json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func displayCronCommands(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	report := controller.ListCronCommand()
	b, err := json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func rerunCronCommand(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	/*
		{"process": "ifbccard"}
		{"package_group": "basic", "package_host": "xfp-dev", "summary": {"message": "STOP PROCESS : ifbccard\nNOT RUNNING\n", "package_name": "default"}}
		{'system': {'message': 'not found process', 'code': 700}}
	*/
	params, err := parsingRequest(req)
	if err != nil {
		log.Warn("invalid parameter : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	process, ok := params["process"]
	if !ok {
		web.ResponseError(res, req, http.StatusBadRequest, "invalid parameter : not found process")
		return
	}
	command, ok := params["command"]
	if !ok {
		web.ResponseError(res, req, http.StatusBadRequest, "invalid parameter : not found command")
		return
	}

	sample, ok := params["sample"]

	report := controller.RerunCronCommand(process, command, sample)
	b, err := json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}
