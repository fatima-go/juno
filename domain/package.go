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
	"fmt"
	log "github.com/fatima-go/fatima-log"
	"sort"
	"strconv"
	"strings"
)

type PackagePoint struct {
	Host string
	Name string
}

func (pp PackagePoint) IsEmpty() bool {
	return pp.Host == ""
}

func NewPackagePoint(pack string) PackagePoint {
	data := PackagePoint{Name: "default"}
	if len(pack) < 1 {
		return data
	}

	i := strings.Index(pack, ":")
	if i < 0 {
		data.Host = pack
		return data
	}
	data.Host = pack[:i]
	data.Name = pack[i+1 : len(pack)]
	return data
}

func NewPackageReport() PackageReport {
	p := PackageReport{}
	p.Platform = NewPlatformInfo()
	return p
}

type PackageReport struct {
	Group    string         `json:"package_group"`
	Host     string         `json:"package_host"`
	PSStatus int            `json:"system_ps_status"`
	HAStatus int            `json:"system_status"`
	Summary  PackageSummary `json:"summary"`
	ProcInfo []ProcessInfo  `json:"process_list"`
	Platform PlatformInfo   `json:"platform"`
}

func (p PackageReport) Sort(sortType SortType, order Order) PackageReport {
	switch sortType {
	case SortTypeName:
		return p.sortWithName(order)
	}

	return p
}

func (p PackageReport) sortWithName(order Order) PackageReport {
	if order == OrderNone {
		return p
	}

	orderedProcInfo := make([]ProcessInfo, 0)
	for _, groupProcInfo := range p.splitProcessWithGroup() {
		if !IsOpmGroup(groupProcInfo.GroupName) {
			if order == OrderAsc {
				log.Info("%s ascending", groupProcInfo.GroupName)
				sort.Sort(ByProcessNameAsc(groupProcInfo.Entries))
			} else if order == OrderDesc {
				log.Info("%s descending", groupProcInfo.GroupName)
				sort.Sort(ByProcessNameDesc(groupProcInfo.Entries))
			}
		}
		orderedProcInfo = append(orderedProcInfo, groupProcInfo.Entries...)
	}
	p.ProcInfo = orderedProcInfo
	return p
}

func (p PackageReport) splitProcessWithGroup() []*GroupProcessInfo {
	groupList := make([]*GroupProcessInfo, 0)

	for _, proc := range p.ProcInfo {
		info := findGroupInList(groupList, proc.Group)
		if info == nil {
			info = &GroupProcessInfo{}
			info.GroupName = proc.Group
			groupList = append(groupList, info)
		}
		info.addEntry(proc)
	}

	return groupList
}

func findGroupInList(list []*GroupProcessInfo, groupName string) *GroupProcessInfo {
	for _, record := range list {
		if record.GroupName == groupName {
			return record
		}
	}
	return nil
}

type GroupProcessInfo struct {
	GroupName string
	Entries   []ProcessInfo
}

func (g *GroupProcessInfo) addEntry(entry ProcessInfo) {
	if g.Entries == nil {
		g.Entries = make([]ProcessInfo, 0)
	}
	g.Entries = append(g.Entries, entry)
}

type ByProcessNameAsc []ProcessInfo

func (n ByProcessNameAsc) Len() int      { return len(n) }
func (n ByProcessNameAsc) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n ByProcessNameAsc) Less(i, j int) bool {
	return n[i].Name < n[j].Name
}

type ByProcessNameDesc []ProcessInfo

func (n ByProcessNameDesc) Len() int      { return len(n) }
func (n ByProcessNameDesc) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n ByProcessNameDesc) Less(i, j int) bool {
	return n[i].Name > n[j].Name
}

type PackageSummary struct {
	Alive int    `json:"alive"`
	Dead  int    `json:"dead"`
	Name  string `json:"package_name"`
	Total int    `json:"total"`
}

type ProcessInfo struct {
	CpuUtil   string `json:"cpu"`
	FDCount   string `json:"fd"`
	Thread    string `json:"thread"`
	Group     string `json:"group"`
	ICount    string `json:"ic"`
	Index     int    `json:"index"`
	Memory    string `json:"mem"`
	Name      string `json:"name"`
	Pid       string `json:"pid"`
	QCount    string `json:"qcount"`
	QKey      string `json:"qkey"`
	StartTime string `json:"start_time"`
	Status    string `json:"status"`
}

func NewProcessInfo() *ProcessInfo {
	info := ProcessInfo{}
	info.Status = ProcStatusDead
	info.CpuUtil = "-"
	info.FDCount = "-"
	info.Thread = "-"
	info.ICount = "0"
	info.Memory = "-"
	info.Pid = "-"
	info.QCount = "-"
	info.QKey = "-"
	info.StartTime = "-"
	return &info
}

func (p *ProcessInfo) GetICount() int {
	ic, err := strconv.Atoi(p.ICount)
	if err != nil {
		return 0
	}

	return ic
}

func (p *ProcessInfo) AddICount() {
	prev := p.GetICount()
	p.ICount = fmt.Sprintf("%d", prev+1)
}

func (p *ProcessInfo) ResetICount() {
	p.ICount = "0"
}
