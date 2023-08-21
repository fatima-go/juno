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

package domain

const (
	PropWebServerAddress     = "webserver.address"
	PropWebServerPort        = "webserver.port"
	PropGatewayServerAddress = "gateway.address"
	PropGatewayServerPort    = "gateway.port"
	ValueGatewayDefaultPort  = "9190"
	ValueJunoRegistUrl       = "juno/regist/v1"
	ValueJunoUnregistUrl     = "juno/unregist/v1"
)

const (
	HEADER_FATIMA_AUTH_TOKEN = "fatima-auth-token"

	PROC_STATUS_ALIVE = "ALIVE"
	PROC_STATUS_DEAD  = "DEAD"

	FOLDER_PACKAGE = "package"
	FOLDER_CFM     = "cfm"
	FOLDER_HA      = "ha"
	FILE_HA        = "system.ha"
	FILE_PS        = "system.ps"
	FILE_LOG_LEVEL = "loglevels"

	FOLDER_APP_REVISION = "revision"

	PACKAGE_DEPLOY_FAR = "far"
	PACKAGE_DEPLOY_GAR = "gar"
)
