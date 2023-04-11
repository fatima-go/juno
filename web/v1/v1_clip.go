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
// @author 1100282
// @date 2017. 5. 7. PM 5:24
//

package v1

import (
	"encoding/json"
	"net/http"
	"throosea.com/juno/web"
)

type Clipboard struct {
	Content string `json:"content"`
}

func clip(controller web.JunoWebServiceController, res http.ResponseWriter, req *http.Request) {
	clipboard := Clipboard{}
	clipboard.Content = controller.GetClipboard()
	b, _ := json.Marshal(clipboard)
	web.ResponseSuccess(res, req, string(b))
}
