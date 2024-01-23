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
	"bufio"
	"bytes"
	"fmt"
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/lib"
	"github.com/fatima-go/fatima-core/monitor"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/web"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

const (
	AlarmCategoryMonitor = "monitor"
)

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
	output := p.readMeaningfulOutputMessage(next.Name)
	if len(output) > 0 {
		msg = fmt.Sprintf("%s\n```%s```", msg, output)
	}
	p.fatimaRuntime.GetSystemNotifyHandler().SendAlarmWithCategory(alarmLvl, monitor.ActionUnknown, msg, AlarmCategoryMonitor)

	if next.Status != domain.PROC_STATUS_ALIVE {
		go p.restartProc(next)
	}
}

func (p *processMonitor) readMeaningfulOutputMessage(proc string) string {
	procDir := filepath.Join(p.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(),
		builder.FatimaFolderApp,
		proc,
		builder.FatimaFolderProc)
	pidFile := filepath.Join(procDir, fmt.Sprintf("%s.pid", proc))
	b, err := os.ReadFile(pidFile)
	if err != nil {
		log.Warn("fail to read pid file %s : %s", pidFile, err.Error())
		return ""
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		log.Warn("fail to convert pid number %s : %s", string(b), err.Error())
		return ""
	}

	outputFile := filepath.Join(procDir, fmt.Sprintf("%s.%d.output", proc, pid))
	b, err = os.ReadFile(outputFile)
	if err != nil {
		log.Warn("fail to read output file %s : %s", outputFile, err.Error())
		return ""
	}

	output := findPanicPoint(b)
	if len(output) > 0 {
		// 마지막 패닉 포인트를 찾았다면 해당 내용을 리턴
		return output
	}

	if len(b) < 2048 {
		// 패닉 포인트를 찾진 못했으나 전체 내용이 2k 이하라면 그대로 리턴
		return string(b)
	}

	// 얖에서 30라인정도 빼서 그 내용을 리턴
	return readFirstNLines(b, 30)
}

func findPanicPoint(output []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	panicLines := make([]string, 0)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "panic: ") {
			panicLines = make([]string, 0) // reset
			panicLines = append(panicLines, line)
			continue
		}

		if len(panicLines) > 0 {
			panicLines = append(panicLines, line)
		}
	}

	if len(panicLines) == 0 {
		return ""
	}

	var buf bytes.Buffer
	for _, line := range panicLines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return buf.String()
}

func readFirstNLines(output []byte, lineCount int) string {
	scanner := bufio.NewScanner(bytes.NewReader(output))

	var buf bytes.Buffer
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line)
		if count < lineCount {
			buf.WriteByte('\n')
		}
	}

	return buf.String()
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
		p.fatimaRuntime.GetSystemNotifyHandler().SendAlarmWithCategory(monitor.AlamLevelMajor, monitor.ActionUnknown, msg, AlarmCategoryMonitor)
		return
	}

	pkgProc := p.yamlConfig.GetProcByName(target.Name)
	if pkgProc == nil {
		log.Warn("not found pkg process %s", target.Name)
		return
	}

	time.Sleep(time.Second * 3)
	msg := fmt.Sprintf("프로세스 [%s] 를 재시작합니다", target.Name)
	p.fatimaRuntime.GetSystemNotifyHandler().SendAlarmWithCategory(monitor.AlarmLevelWarn, monitor.ActionUnknown, msg, AlarmCategoryMonitor)
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

	log.Debug("%s : current=%d, deadline=%d", proc, lib.CurrentTimeMillis(), deadline)
	if deadline == 0 || lib.CurrentTimeMillis() <= deadline {
		return true
	}

	delete(p.internalJobs, proc)
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
