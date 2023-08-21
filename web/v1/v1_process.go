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

func registProcess(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	/*
		{"process": "ifbccard", "group_id": "4", "package": "xfp-dev"}
		{"system": {"message": "success", "code": 200}}
		{"system": {"message": "total 1 juno. process 1 registed", "code": 200}}
	*/
	params, err := parsingRequest(req)
	if err != nil {
		log.Warn("invalid parameter : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	if log.IsDebugEnabled() {
		log.Debug("regist process : %v", params)
	}

	clientAddress, _ := params["client_address"]
	if !controller.IsRemoteOperationAllowed(clientAddress) {
		log.Warn("remote operation is not allowed")
		web.ResponseError(res, req, http.StatusForbidden, "remote operation is not allowed")
		return
	}

	var process, group_id string
	var ok bool
	process, ok = params["process"]
	if !ok {
		log.Warn("not found process")
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}
	group_id, ok = params["group_id"]
	if !ok {
		log.Warn("not found group_id")
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	err = controller.RegistProcess(process, group_id)
	if err != nil {
		log.Warn("fail to regist : %s", err.Error())
		web.WriteSystemError(res, req, "fail to regist : "+err.Error())
		return
	}
	web.WriteSystemSuccess(res, req, "success")
}

func startProcess(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	/*
		{"process": "ifbccard"}
		{"package_group": "basic", "package_host": "xfp-dev", "summary": {"message": "START PROCESS : ifbccard\nFAIL TO EXECUTE\n", "package_name": "default"}}
		{'system': {'message': 'not found process', 'code': 700}}
	*/
	params, err := parsingRequest(req)
	if err != nil {
		log.Warn("invalid parameter : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	if !controller.IsRemoteOperationAllowed(req.RemoteAddr) {
		log.Warn("remote operation is not allowed")
		web.ResponseError(res, req, http.StatusForbidden, "remote operation is not allowed")
		return
	}

	_, ok := params["all"]
	all := ok
	var group string
	group, ok = params["group"]
	process, ok := params["process"]

	var b []byte
	report := controller.StartProcess(all, group, process)
	b, err = json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func stopProcess(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
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

	if !controller.IsRemoteOperationAllowed(req.RemoteAddr) {
		log.Warn("remote operation is not allowed")
		web.ResponseError(res, req, http.StatusForbidden, "remote operation is not allowed")
		return
	}

	_, ok := params["all"]
	all := ok
	var group string
	group, ok = params["group"]
	process, ok := params["process"]

	var b []byte
	report := controller.StopProcess(all, group, process)
	b, err = json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func unregistProcess(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	/*
		{"process": "ifbccard", "package": "xfp-dev"}
		{"system": {"message": "total 1 juno. 1 unregisted", "code": 200}}
	*/
	params, err := parsingRequest(req)
	if err != nil {
		log.Warn("invalid parameter : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	if log.IsDebugEnabled() {
		log.Debug("unregist process : %v", params)
	}

	clientAddress, _ := params["client_address"]
	if !controller.IsRemoteOperationAllowed(clientAddress) {
		log.Warn("remote operation is not allowed")
		web.ResponseError(res, req, http.StatusForbidden, "remote operation is not allowed")
		return
	}

	process, ok := params["process"]
	if !ok {
		log.Warn("not found process")
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	err = controller.UnregistProcess(process)
	if err != nil {
		log.Warn("fail to unregist : %s", err.Error())
		web.WriteSystemSuccess(res, req, "fail to unregist : "+err.Error())
		return
	}
	web.WriteSystemSuccess(res, req, "success")
}

func clearIcProcess(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	params, err := parsingRequest(req)
	if err != nil {
		log.Warn("invalid parameter : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	_, ok := params["all"]
	all := ok
	var group string
	group, ok = params["group"]
	process, ok := params["process"]

	var b []byte
	report := controller.ClearIcProcess(all, group, process)
	b, err = json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}

func deploymentHistoryProcess(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	params, err := parsingRequest(req)
	if err != nil {
		log.Warn("invalid parameter : %s", err.Error())
		web.ResponseError(res, req, http.StatusBadRequest, err.Error())
		return
	}

	_, ok := params["all"]
	all := ok
	var group string
	group, ok = params["group"]
	process, ok := params["process"]

	var b []byte
	report := controller.DeploymentHistory(all, group, process)
	b, err = json.Marshal(report)
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}
	web.ResponseSuccess(res, req, string(b))
}
