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

package service

import (
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/juno/domain"
	"runtime"
	"strconv"
	"sync"
	"time"
)

func (service *DomainService) GetPackageReport(loc *time.Location, sortType domain.SortType, order domain.Order) domain.PackageReport {
	return service.buildPackageReport(loc).Sort(sortType, order)
}

func (service *DomainService) buildPackageReport(loc *time.Location) domain.PackageReport {
	if runtime.GOOS == "darwin" {
		return service.getPackageReportDarwin(loc)
	}
	return service.getPackageReportLinux(loc)
}

func (service *DomainService) getPackageReportLinux(loc *time.Location) domain.PackageReport {
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())

	processList := newProcessList(toGroupMap(yamlConfig.Groups))
	yamlConfig.OrderByGroup()

	for i, p := range yamlConfig.Processes {
		proc := GetProcessMonitor().GetProcess(p.Name, loc)
		proc.Index = i
		processList.processes = append(processList.processes, &proc)
	}

	report := domain.NewPackageReport()
	report.Group = service.fatimaRuntime.GetPackaging().GetGroup()
	report.Host = service.fatimaRuntime.GetPackaging().GetHost()

	report.HAStatus = int(service.fatimaRuntime.GetSystemStatus().GetHAStatus())
	report.PSStatus = int(service.fatimaRuntime.GetSystemStatus().GetPSStatus())
	report.Summary = domain.PackageSummary{}
	report.Summary.Name = service.fatimaRuntime.GetPackaging().GetName()
	report.Summary.Total = len(processList.processes)
	report.ProcInfo = make([]domain.ProcessInfo, 0)
	for _, v := range processList.processes {
		report.ProcInfo = append(report.ProcInfo, *v)
		if v.Status == domain.ProcStatusAlive {
			report.Summary.Alive = report.Summary.Alive + 1
		} else {
			report.Summary.Dead = report.Summary.Dead + 1
		}
	}

	return report
}

func (service *DomainService) getPackageReportDarwin(loc *time.Location) domain.PackageReport {
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())

	processList := newProcessList(toGroupMap(yamlConfig.Groups))
	yamlConfig.OrderByGroup()

	for i, p := range yamlConfig.Processes {
		processList.wg.Add(1)
		index := i
		item := p
		go func() {
			buildBasicProcessStatusDarwin(service.fatimaRuntime.GetEnv(), processList, index, item)
		}()
	}

	processList.wg.Wait()

	inspector.MeasureProcessStatus(processList.processes, loc)

	report := domain.NewPackageReport()
	report.Group = service.fatimaRuntime.GetPackaging().GetGroup()
	report.Host = service.fatimaRuntime.GetPackaging().GetHost()

	report.HAStatus = int(service.fatimaRuntime.GetSystemStatus().GetHAStatus())
	report.PSStatus = int(service.fatimaRuntime.GetSystemStatus().GetPSStatus())
	report.Summary = domain.PackageSummary{}
	report.Summary.Name = service.fatimaRuntime.GetPackaging().GetName()
	report.Summary.Total = len(processList.processes)
	report.ProcInfo = make([]domain.ProcessInfo, 0)
	for _, v := range processList.processes {
		report.ProcInfo = append(report.ProcInfo, *v)
		if v.Status == domain.ProcStatusAlive {
			report.Summary.Alive = report.Summary.Alive + 1
		} else {
			report.Summary.Dead = report.Summary.Dead + 1
		}
	}

	return report
}

type ProcessList struct {
	processes []*domain.ProcessInfo
	wg        sync.WaitGroup
	group     map[int]string
}

func newProcessList(group map[int]string) *ProcessList {
	pList := ProcessList{}
	pList.processes = make([]*domain.ProcessInfo, 0)
	pList.group = group
	return &pList
}

func buildBasicProcessStatus(env fatima.FatimaEnv, pList *ProcessList, item builder.ProcessItem) *domain.ProcessInfo {
	proc := domain.NewProcessInfo()
	proc.Name = item.Name
	proc.Group = pList.group[item.Gid]
	if len(item.Grep) == 0 {
		pid := GetPid(env, item)
		if pid != 0 {
			if inspector.CheckProcessRunningByPid(proc.Name, pid) {
				proc.Status = domain.ProcStatusAlive
				proc.Pid = strconv.Itoa(pid)
			}
		}
	} else {
		pid := GetPidByGrep(item.Grep)
		if pid > 0 {
			proc.Status = domain.ProcStatusAlive
			proc.Pid = strconv.Itoa(pid)
		}
	}

	return proc
}

func buildBasicProcessStatusDarwin(env fatima.FatimaEnv, pList *ProcessList, index int, item builder.ProcessItem) {
	defer pList.wg.Done()
	proc := domain.NewProcessInfo()
	proc.Name = item.Name
	proc.Index = index
	proc.Group = pList.group[item.Gid]
	if len(item.Grep) == 0 {
		pid := GetPid(env, item)
		if pid != 0 {
			if inspector.CheckProcessRunningByPid(proc.Name, pid) {
				proc.Status = domain.ProcStatusAlive
				proc.Pid = strconv.Itoa(pid)
			}
		}
	} else {
		pid := GetPidByGrep(item.Grep)
		if pid > 0 {
			proc.Status = domain.ProcStatusAlive
			proc.Pid = strconv.Itoa(pid)
		}
	}

	pList.processes = append(pList.processes, proc)
}

func (service *DomainService) GetPackageReportForHealthCheck() map[string]string {
	report := make(map[string]string)
	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	report["package_name"] = service.fatimaRuntime.GetPackaging().GetName()
	return report
}

func toGroupMap(groupItems []builder.GroupItem) map[int]string {
	groups := make(map[int]string)

	for _, v := range groupItems {
		groups[v.Id] = v.Name
	}

	return groups
}
