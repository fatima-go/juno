/*
 * Copyright 2025 github.com/fatima-go
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
 * @author dave_01
 * @date 25. 7. 14. 오후 4:14
 *
 */

package domain

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPackageInfoSort(t *testing.T) {
	p := PackageReport{}
	p.Group = "group"
	p.Host = "host"
	p.ProcInfo = make([]ProcessInfo, 0)
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: ProcessGroupOpm, Name: "jupiter"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: ProcessGroupOpm, Name: "juno"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: ProcessGroupOpm, Name: "saturn"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "svc", Name: "mmatesvc"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "svc", Name: "encryptd"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "svc", Name: "kakaod"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "svc", Name: "recentalbum"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "svc", Name: "notificationd"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "svc", Name: "batcmtlikenoti"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "svc", Name: "cabinetreco"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "engine", Name: "dbro"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "engine", Name: "imaged"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "engine", Name: "targetinvtd"})
	p.ProcInfo = append(p.ProcInfo, ProcessInfo{Group: "engine", Name: "bathomeplist"})

	//for i, proc := range p.ProcInfo {
	//	fmt.Printf("%02d :: %s : %s\n", i, proc.Group, proc.Name)
	//}

	assert.True(t, len(p.ProcInfo) == 14)
	p = p.Sort(SortTypeName, OrderAsc)

	assert.True(t, p.ProcInfo[0].Name == "jupiter")
	assert.True(t, p.ProcInfo[2].Name == "saturn")
	assert.True(t, p.ProcInfo[3].Name == "batcmtlikenoti")
	assert.True(t, p.ProcInfo[9].Name == "recentalbum")
	assert.True(t, p.ProcInfo[10].Name == "bathomeplist")
	assert.True(t, p.ProcInfo[13].Name == "targetinvtd")

	p = p.Sort(SortTypeName, OrderDesc)

	assert.True(t, p.ProcInfo[0].Name == "jupiter")
	assert.True(t, p.ProcInfo[2].Name == "saturn")
	assert.True(t, p.ProcInfo[3].Name == "recentalbum")
	assert.True(t, p.ProcInfo[9].Name == "batcmtlikenoti")
	assert.True(t, p.ProcInfo[10].Name == "targetinvtd")
	assert.True(t, p.ProcInfo[13].Name == "bathomeplist")
}
