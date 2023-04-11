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
// @date 2017. 3. 8. AM 10:30
//

package engine

import (
	"errors"
	"fmt"
	"os"
)

const (
	PropWebServerAddress     = "webserver.address"
	PropWebServerPort        = "webserver.port"
	PropGatewayServerAddress = "gateway.address"
	PropGatewayServerPort    = "gateway.port"
	ValueGatewayDefaultPort  = "9190"
	ValueJunoRegistUrl       = "juno/regist/v1"
	ValueJunoUnregistUrl     = "juno/unregist/v1"
)

func ensureDirectory(path string, forceCreate bool) error {
	if stat, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if forceCreate {
				return os.MkdirAll(path, 0755)
			}
		} else if !stat.IsDir() {
			return errors.New(fmt.Sprintf("%s path exist as file", path))
		}
	}

	return nil
}
