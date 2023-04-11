/*
 * Copyright (c) 2018 throosea.com.
 * All right reserved.
 *
 * This software is the confidential and proprietary information of throosea.com.
 * You shall not disclose such Confidential Information and
 * shall use it only in accordance with the terms of the license agreement
 * you entered into with throosea.com.
 */

package v1

import (
	"encoding/json"
	"net/http"
	"throosea.com/juno/web"
	"throosea.com/log"
)

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
