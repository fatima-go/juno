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

import "strings"

const (
	SortTypeNone = "none"
	SortTypeName = "name"
)

type SortType string

func ToSortType(s string) SortType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "name":
		return SortTypeName
	}
	return SortTypeNone
}

const (
	OrderNone = "none"
	OrderAsc  = "asc"
	OrderDesc = "desc"
)

type Order string

func ToOrder(s string) Order {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "asc":
		return OrderAsc
	case "desc":
		return OrderDesc
	}
	return OrderNone
}

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

	ProcStatusAlive = "ALIVE"
	ProcStatusDead  = "DEAD"

	FolderPackage = "package"
	FolderCfm     = "cfm"
	FolderHa      = "ha"
	FileHa        = "system.ha"
	FilePs        = "system.ps"
	FileLogLevel  = "loglevels"

	FolderAppRevision = "revision"

	PackageDeployFar = "far"
	PackageDeployGar = "gar"
)

const (
	ProcessGroupOpm = "opm"
)

func IsOpmGroup(opm string) bool {
	return strings.ToLower(opm) == ProcessGroupOpm
}
