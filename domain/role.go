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
	RoleMonitor = iota
	RoleOperator
	RoleUnknown
)

type Role int

func (r Role) String() string {
	switch r {
	case RoleMonitor:
		return "MONITOR"
	case RoleOperator:
		return "OPERATOR"
	}
	return "UNKNOWN"
}

func (r Role) Acceptable(another Role) bool {
	if r == RoleUnknown || another == RoleUnknown {
		return false
	}

	switch r {
	case RoleOperator:
		return true
	case RoleMonitor:
		return another == r
	}
	return false
}

func ToRole(value string) Role {
	switch strings.ToUpper(value) {
	case "OPERATOR":
		return RoleOperator
	case "MONITOR":
		return RoleMonitor
	}
	return RoleUnknown
}

func ToRoleString(role Role) string {
	switch role {
	case RoleMonitor:
		return "MONITOR"
	case RoleOperator:
		return "OPERATOR"
	}
	return "UNKNOWN"
}
