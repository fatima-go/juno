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
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/web"
	"mime"
	"mime/multipart"
	"net/http"
)

func deployPackage(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	log.Info("start deploy package")
	defer func() {
		log.Info("deploy finished")
	}()

	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		log.Warn("fail to build json response : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}

	mr := multipart.NewReader(req.Body, params["boundary"])
	_, err = controller.DeployPackage(mr)
	if err != nil {
		log.Warn("fail to deploy : %s", err.Error())
		web.ResponseError(res, req, http.StatusInternalServerError, err.Error())
		return
	}

	web.ResponseSuccess(res, req, "")
}
