/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with p work for additional information
 * regarding copyright ownership.  The ASF licenses p file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use p file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 * @project fatima
 * @author DeockJin Chung (jin.freestyle@gmail.com)
 * @date 22. 11. 14. 오후 6:16
 */

package domain

import "throosea.com/fatima/builder"

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

func IsManagedOpmProcess(p builder.ProcessItem) bool {
	if p.Gid != 1 {
		return false // 1 : OPM
	}

	_, ok := managedOpmProcessSet[p.Name]
	return ok
}
