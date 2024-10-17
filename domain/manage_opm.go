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

import (
	"github.com/fatima-go/fatima-core"
)

var managedOpmProcessSet = map[string]struct{}{"jupiter": {}, "juno": {}, "saturn": {}}

func GetManagedOpmProcessNames() []string {
	list := make([]string, 0)
	for k, _ := range managedOpmProcessSet {
		list = append(list, k)
	}
	return list
}

func IsManagedOpmProcessName(procName string) bool {
	_, ok := managedOpmProcessSet[procName]
	return ok
}

func IsManagedOpmProcess(p fatima.FatimaPkgProc) bool {
	if p.GetGid() != 1 {
		return false // 1 : OPM
	}

	_, ok := managedOpmProcessSet[p.GetName()]
	return ok
}
