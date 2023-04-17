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
	ROLE_MONITOR = iota
	ROLE_OPERATOR
	ROLE_UNKNOWN
)

type Role int

func (r Role) String() string {
	switch r {
	case ROLE_MONITOR:
		return "MONITOR"
	case ROLE_OPERATOR:
		return "OPERATOR"
	}
	return "UNKNOWN"
}

func (r Role) Acceptable(another Role) bool {
	if r == ROLE_UNKNOWN || another == ROLE_UNKNOWN {
		return false
	}

	switch r {
	case ROLE_OPERATOR:
		return true
	case ROLE_MONITOR:
		return another == r
	}
	return false
}

func ToRole(value string) Role {
	switch strings.ToUpper(value) {
	case "OPERATOR":
		return ROLE_OPERATOR
	case "MONITOR":
		return ROLE_MONITOR
	}
	return ROLE_UNKNOWN
}

func ToRoleString(role Role) string {
	switch role {
	case ROLE_MONITOR:
		return "MONITOR"
	case ROLE_OPERATOR:
		return "OPERATOR"
	}
	return "UNKNOWN"
}
