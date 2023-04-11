//
// Copyright (c) 2018 SK TECHX.
// All right reserved.
//
// This software is the confidential and proprietary information of SK TECHX.
// You shall not disclose such Confidential Information and
// shall use it only in accordance with the terms of the license agreement
// you entered into with SK TECHX.
//
//
// @project juno
// @author 1100282
// @date 2018. 4. 10. AM 9:58
//

package service

import (
	"fmt"
	"runtime"
	"sync"
	"throosea.com/fatima"
	"throosea.com/fatima/builder"
	"throosea.com/fatima/lib"
	"throosea.com/fatima/monitor"
	"throosea.com/juno/domain"
	"throosea.com/juno/web"
	"throosea.com/log"
	"time"
)

type ProcessMonitor interface {
	WatchProcesses()
	GetProcessList() []domain.ProcessInfo
	GetProcess(name string, loc *time.Location) domain.ProcessInfo
	ProcessStart(proc string)
	ProcessStop(proc string)
	ResetICount(proc string)
}

type processMonitor struct {
	fatimaRuntime fatima.FatimaRuntime
	loc           *time.Location
	runFlag       bool
	monMutex      sync.Mutex
	jobMutex      sync.Mutex
	procMap       map[string]domain.ProcessInfo
	internalJobs  map[string]int
	yamlConfig    *builder.YamlFatimaPackageConfig
}

var procMonitor *processMonitor

func newProcessMonitor(fatimaRuntime fatima.FatimaRuntime) ProcessMonitor {
	procMonitor = &processMonitor{}
	procMonitor.fatimaRuntime = fatimaRuntime
	procMonitor.runFlag = false
	procMonitor.loc = time.UTC
	procMonitor.procMap = make(map[string]domain.ProcessInfo)
	procMonitor.internalJobs = make(map[string]int)

	for _, procName := range domain.GetManagedOpmProcessNames() {
		deadline := lib.CurrentTimeMillis() + deadlineAfterStart
		procMonitor.internalJobs[procName] = deadline
	}

	return procMonitor
}

func GetProcessMonitor() ProcessMonitor {
	return procMonitor
}

func (p *processMonitor) WatchProcesses() {
	if p.runFlag {
		return
	}

	p.runFlag = true
	defer func() {
		p.runFlag = false
	}()

	// loc *time.Location
	p.yamlConfig = builder.NewYamlFatimaPackageConfig(p.fatimaRuntime.GetEnv())

	mutex := &sync.Mutex{}
	processList := newProcessList(toGroupMap(p.yamlConfig.Groups))
	for _, proc := range p.yamlConfig.Processes {
		processList.wg.Add(1)
		item := proc
		go func() {
			builtProc := buildBasicProcessStatus(p.fatimaRuntime.GetEnv(), processList, item)
			mutex.Lock()
			processList.processes = append(processList.processes, builtProc)
			mutex.Unlock()
			processList.wg.Done()
		}()
	}

	processList.wg.Wait()

	inspector.MeasureProcessStatus(processList.processes, p.loc)
	p.reflectProc(processList.processes)
}

func (p *processMonitor) reflectProc(processes []*domain.ProcessInfo) {
	p.monMutex.Lock()
	defer p.monMutex.Unlock()
	for k, v := range p.procMap {
		found := false
		for _, item := range processes {
			if k == item.Name {
				found = true
				if v.Status != item.Status {
					p.notifyStatusChange(v, *item)
				}
				break
			}
		}
		if !found {
			log.Info("reflect not found %s", k)
			delete(p.procMap, k)
		}
	}

	for _, item := range processes {
		prev, ok := p.procMap[item.Name]
		if !ok {
			p.procMap[item.Name] = *item
			continue
		}
		item.ICount = prev.ICount
		p.procMap[item.Name] = *item
	}
}

func (p *processMonitor) notifyStatusChange(previous, next domain.ProcessInfo) {
	if runtime.GOOS == "darwin" {
		return // DARWIN not support at this time
	}

	if p.isInternalJob(next.Name) {
		log.Info("[%s] is going internal", next.Name)
		return
	}

	log.Warn("[%s] status changed %s to %s", next.Name, previous.Status, next.Status)
	var alarmLvl monitor.AlarmLevel
	alarmLvl = monitor.AlamLevelMajor
	if next.Status == domain.PROC_STATUS_ALIVE {
		alarmLvl = monitor.AlarmLevelMinor
	}
	msg := fmt.Sprintf("프로세스 상태 감지 : [%s]의 상태가 %s로 변경 되었습니다", next.Name, next.Status)
	p.fatimaRuntime.GetSystemNotifyHandler().SendAlarmWithCategory(alarmLvl, msg, "monitor")

	if next.Status != domain.PROC_STATUS_ALIVE {
		go p.restartProc(next)
	}
}

func (p *processMonitor) restartProc(target domain.ProcessInfo) {
	p.monMutex.Lock()
	defer p.monMutex.Unlock()
	procInfo, ok := p.procMap[target.Name]
	if !ok {
		return
	}

	log.Info("%s IC = %d", target.Name, procInfo.GetICount())
	if procInfo.GetICount() >= maxRestartCount {
		log.Info("재시도 최대 회수 초과. 재시도 포기 : %s", target.Name)
		time.Sleep(time.Second * 1)
		msg := fmt.Sprintf("프로세스 재시도 최대 횟수 초과 : %s", target.Name)
		p.fatimaRuntime.GetSystemNotifyHandler().SendAlarmWithCategory(monitor.AlamLevelMajor, msg, "monitor")
		return
	}

	pkgProc := p.yamlConfig.GetProcByName(target.Name)
	if pkgProc == nil {
		log.Warn("not found pkg process %s", target.Name)
		return
	}

	time.Sleep(time.Second * 3)
	msg := fmt.Sprintf("프로세스 [%s] 를 재시작합니다", target.Name)
	p.fatimaRuntime.GetSystemNotifyHandler().SendAlarmWithCategory(monitor.AlarmLevelWarn, msg, "monitor")
	time.Sleep(time.Second * 1)
	procInfo.AddICount()
	p.procMap[target.Name] = procInfo
	ExecuteProgram(p.fatimaRuntime.GetEnv(), pkgProc)
}

const maxRestartCount = 3

func (p *processMonitor) GetProcessList() []domain.ProcessInfo {
	list := make([]domain.ProcessInfo, 0)

	p.monMutex.Lock()
	defer p.monMutex.Unlock()
	for _, v := range p.procMap {
		list = append(list, v)
	}

	return list
}

func (p *processMonitor) GetProcess(name string, loc *time.Location) domain.ProcessInfo {
	p.monMutex.Lock()
	defer p.monMutex.Unlock()
	proc := p.procMap[name]

	if proc.Status != domain.PROC_STATUS_ALIVE {
		return proc
	}

	// 2018-04-10 02:08:46
	utcStartTime, err := time.ParseInLocation(web.TIME_YYYYMMDDHHMMSS, proc.StartTime, p.loc)
	if err != nil {
		proc.StartTime = fmt.Sprintf("%s (%s)", proc.StartTime, p.loc.String())
		return proc
	}

	proc.StartTime = utcStartTime.In(loc).Format(web.TIME_YYYYMMDDHHMMSS)
	return proc
}

func (p *processMonitor) ProcessStart(proc string) {
	log.Debug("Mark Process Starting : %s", proc)
	p.jobMutex.Lock()
	defer p.jobMutex.Unlock()

	deadline := lib.CurrentTimeMillis() + deadlineAfterStart
	p.internalJobs[proc] = deadline
}

const deadlineAfterStart = 3 * 1000

func (p *processMonitor) ProcessStop(proc string) {
	log.Debug("Mark Process Stop : %s", proc)
	p.jobMutex.Lock()
	defer p.jobMutex.Unlock()
	p.internalJobs[proc] = 0
}

func (p *processMonitor) isInternalJob(proc string) bool {
	log.Debug("checking internal job : %s", proc)
	p.jobMutex.Lock()
	defer p.jobMutex.Unlock()
	deadline, ok := p.internalJobs[proc]
	if !ok {
		return false
	}

	delete(p.internalJobs, proc)

	if deadline == 0 || lib.CurrentTimeMillis() <= deadline {
		return true
	}
	return false
}

func (p *processMonitor) ResetICount(proc string) {
	log.Debug("Reset IC : %s", proc)

	p.monMutex.Lock()
	defer p.monMutex.Unlock()

	procInfo, ok := p.procMap[proc]
	if !ok {
		return
	}

	procInfo.ResetICount()
	p.procMap[proc] = procInfo
}
