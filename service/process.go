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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/builder/platform"
	"github.com/fatima-go/fatima-core/lib"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/service/goaway"
	"github.com/fatima-go/juno/web"
)

func (service *DomainService) StartProcess(all bool, group string, proc string) map[string]interface{} {
	report := make(map[string]interface{})

	log.Info("StartProcess. all=[%b], group=[%s], proc=[%s]", all, group, proc)

	target := make([]fatima.FatimaPkgProc, 0)
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())
	if all {
		target = yamlConfig.GetAllProc(true)
	} else if len(group) > 0 {
		if strings.ToLower(group) == "opm" {
			report["system"] = web.SystemResponse{Code: 700, Message: "OPM group not permitted"}
			return report
		}
		target = yamlConfig.GetProcByGroup(group)
	} else {
		target = append(target, yamlConfig.GetProcByName(proc))
	}

	size := len(target)
	if size == 0 {
		report["system"] = web.SystemResponse{Code: 700, Message: "not found process"}
		return report
	}

	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]string)
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()

	var buffer bytes.Buffer
	output := startProcessWithWeightGroup(service.fatimaRuntime, target, processExecuteAsync)
	buffer.WriteString(output)
	buffer.WriteByte('\n')
	summary["message"] = buffer.String()
	report["summary"] = summary
	return report
}

func startProcess(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) (int, string) {
	if proc == nil {
		return 0, fmt.Sprintf("UNREGISTED PROCESS")
	}

	/*
		START PROCESS : ifbccard\nFAIL TO EXECUTE\n
		START PROCESS : fcmapp
		SUCCESS : pid=65361
		START PROCESS : fcmapp
		ALEADY RUNNING 65361

	*/

	log.Warn("TRY TO START PROCESS : %s", proc.GetName())

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("\nSTART PROCESS : %s\n", proc.GetName()))

	pid := GetPid(env, proc)

	if pid > 0 && inspector.CheckProcessRunningByPid(proc.GetName(), pid) {
		buffer.WriteString(fmt.Sprintf("ALEADY RUNNING : %d", pid))
		return 0, buffer.String()
	}

	childPid, err := ExecuteProgram(env, proc)
	if err != nil {
		buffer.WriteString(fmt.Sprintf("FAIL TO EXECUTE : %s", err.Error()))
	} else {
		buffer.WriteString(fmt.Sprintf("SUCCESS : pid=%d", childPid))
		GetProcessMonitor().ResetICount(proc.GetName())
	}

	return childPid, buffer.String()
}

func (service *DomainService) StopProcess(all bool, group string, proc string) map[string]interface{} {
	report := make(map[string]interface{})

	log.Info("StopProcess. all=[%t], group=[%s], proc=[%s]", all, group, proc)

	target := make([]fatima.FatimaPkgProc, 0)
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())
	if all {
		target = yamlConfig.GetAllProc(true)
	} else if len(group) > 0 {
		if strings.ToLower(group) == "opm" {
			report["system"] = web.SystemResponse{Code: 700, Message: "OPM group not permitted"}
			return report
		}
		target = yamlConfig.GetProcByGroup(group)
	} else {
		target = append(target, yamlConfig.GetProcByName(proc))
	}

	size := len(target)
	if size == 0 {
		report["system"] = web.SystemResponse{Code: 700, Message: "not found process"}
		return report
	}

	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]string)
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()

	var buffer bytes.Buffer
	output := stopProcessWithWeightGroup(service.fatimaRuntime, target, processTerminateAsync)
	buffer.WriteString(output)
	buffer.WriteByte('\n')
	summary["message"] = buffer.String()
	report["summary"] = summary
	return report
}

func stopProcess(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) (int, string) {
	if proc == nil {
		return 0, fmt.Sprintf("UNREGISTED PROCESS")
	}

	log.Warn("TRY TO STOP PROCESS : %s", proc.GetName())

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("\nSTOP PROCESS : %s\n", proc.GetName()))

	comp := strings.ToLower(proc.GetName())
	if comp == "jupiter" || comp == "juno" {
		log.Warn("%s is not permitted for killing", proc.GetName())
		buffer.WriteString(fmt.Sprintf("%s is not permitted for killing", proc.GetName()))
		return 0, buffer.String()
	}

	pid := GetPid(env, proc)
	if pid < 1 || !inspector.CheckProcessRunningByPid(proc.GetName(), pid) {
		log.Info("%s[%d] is not running", proc.GetName(), pid)
		buffer.WriteString("NOT RUNNING\n")
		return 0, buffer.String()
	}

	executeGoaway(env, proc, pid)
	err := KillProgram(proc.GetName(), pid)
	if err != nil {
		buffer.WriteString(fmt.Sprintf("FAIL TO KILL %s[%d] : %s", proc.GetName(), pid, err.Error()))
	} else {
		buffer.WriteString(fmt.Sprintf("KILLED %d\n", pid))
	}

	return pid, buffer.String()
}

// execute "goaway.sh"
func executeGoaway(env fatima.FatimaEnv, proc fatima.FatimaPkgProc, pid int) {
	if proc == nil {
		return
	}

	err := goaway.ExecuteGoawayByIPC(proc.GetName())
	if err != nil {
		log.Warn("skip goaway by IPC : %s", err.Error())
		if isFatimaOrientProcess(env, proc) {
			// 기존 방식대로 SIGUSR1 을 전송
			sendGoawaySignal(proc, pid)
		}
	}

	path := filepath.Join(env.GetFolderGuide().GetFatimaHome(), "app", proc.GetName(), shellGoaway)
	if _, err := os.Stat(path); err != nil {
		return
	}

	GetProcessMonitor().ProcessStop(proc.GetName())
	log.Info("executing %s for: %s", shellGoaway, path)
	cmd := exec.Command("/bin/sh", "-c", path)
	err = cmd.Run()
	if err != nil {
		log.Error("fail to exec goaway[%s] : %s", path, err.Error())
	}

	log.Info("goaway finished")
	return
}

// isFatimaOrientProcess fatima 프레임워크로 제작된 프로세스인지 여부를 판단한다.
// 별도의 실행 sh 파일이 없고 별도의 bin 패스가 명시되어 있지 않으면 fatima 프레임워크로 제작된 프로세스로 판단한다.
func isFatimaOrientProcess(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) bool {
	if hasExecutingShell(env, proc) {
		return false
	}
	if len(proc.GetPath()) > 0 {
		return false
	}
	return true
}

func sendGoawaySignal(proc fatima.FatimaPkgProc, pid int) {
	err := syscall.Kill(pid, syscall.SIGUSR1)
	if err != nil {
		log.Warn("goaway to pid %d : signal(SIGUSR1) fail. err=%s", pid, err.Error())
	} else {
		log.Warn("send SIGUSR1 to %s(%d)", proc.GetName(), pid)
	}
}

const (
	shellGoaway = "goaway.sh"
)

func (service *DomainService) RegistProcess(proc string, groupId string) error {
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())

	gid, err := strconv.Atoi(groupId)
	if err != nil {
		gid = yamlConfig.GetGroupId(groupId)
		if gid < 0 {
			return fmt.Errorf("invalid groupId format : %s", err.Error())
		}
	}

	if isExistProc(proc, yamlConfig) {
		return fmt.Errorf("aleady exist process %s", proc)
	}

	if !yamlConfig.IsValidGroupId(gid) {
		return fmt.Errorf("invalid group id")
	}

	p := builder.ProcessItem{}
	p.Gid = gid
	p.Name = proc
	p.Hb = false
	var logLevel log.LogLevel
	logLevel = log.LOG_INFO
	p.Loglevel = strings.ToLower(logLevel.String())

	yamlConfig.Processes = append(yamlConfig.Processes, p)
	yamlConfig.Save()

	// reflect loglevel
	m := service.readLogLevels()
	m[proc] = log.ConvertLogLevelToHexa(logLevel.String())
	service.writeLogLevels(m)

	return nil
}

func isExistProc(proc string, yamlConfig *builder.YamlFatimaPackageConfig) bool {
	comp := strings.ToLower(proc)

	for _, p := range yamlConfig.Processes {
		if comp == strings.ToLower(p.GetName()) {
			return true
		}
	}

	return false
}

func (service *DomainService) UnregistProcess(proc string) error {
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())
	comp := strings.ToLower(proc)
	found := -1

	for i, p := range yamlConfig.Processes {
		if comp == strings.ToLower(p.GetName()) {
			found = i
			break
		}
	}

	if found < 0 {
		// not found
		return fmt.Errorf("not found process %s", proc)
	}

	yamlConfig.Processes = append(yamlConfig.Processes[:found], yamlConfig.Processes[found+1:]...)
	yamlConfig.Save()

	// reflect loglevel
	m := service.readLogLevels()
	delete(m, proc)
	service.writeLogLevels(m)

	// unlink app
	unlinkApp(service.fatimaRuntime.GetEnv(), proc)
	fatimaDir := service.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome()

	// remove all logs
	appLogDir := filepath.Join(fatimaDir, builder.FatimaFolderLog, proc)
	os.RemoveAll(appLogDir)
	log.Debug("removed log dir : %s", appLogDir)

	// remove app revisions
	appRevDir := filepath.Join(fatimaDir, builder.FatimaFolderApp, domain.FOLDER_APP_REVISION, proc)
	os.RemoveAll(appRevDir)
	log.Debug("removed rev dir : %s", appRevDir)

	// remove data
	appDataDir := filepath.Join(fatimaDir, builder.FatimaFolderData, proc)
	os.RemoveAll(appDataDir)
	log.Debug("removed data dir : %s", appDataDir)

	// remove deployment history dir
	deploymentHistoryDir := buildHistorySaveDir(service.fatimaRuntime.GetEnv(), proc)
	os.RemoveAll(deploymentHistoryDir)
	log.Debug("removed deployment history dir : %s", deploymentHistoryDir)

	return nil
}

func (service *DomainService) ClearIcProcess(all bool, group string, proc string) map[string]interface{} {
	report := make(map[string]interface{})

	log.Info("ClearIcProcess. all=[%b], group=[%s], proc=[%s]", all, group, proc)

	target := make([]fatima.FatimaPkgProc, 0)
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())
	if all {
		target = yamlConfig.GetAllProc(true)
	} else if len(group) > 0 {
		if strings.ToLower(group) == "opm" {
			report["system"] = web.SystemResponse{Code: 700, Message: "OPM group not permitted"}
			return report
		}
		target = yamlConfig.GetProcByGroup(group)
	} else {
		target = append(target, yamlConfig.GetProcByName(proc))
	}

	size := len(target)
	if size == 0 {
		report["system"] = web.SystemResponse{Code: 700, Message: "not found process"}
		return report
	}

	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]string)
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()

	var buffer bytes.Buffer
	cyBarrier := lib.NewCyclicBarrier(size, nil)
	for _, v := range target {
		t := v
		cyBarrier.Dispatch(func() {
			buffer.WriteString(clearIcProcess(service.fatimaRuntime.GetEnv(), t))
		})
	}
	cyBarrier.Wait()

	summary["message"] = buffer.String()
	report["summary"] = summary
	return report
}

func (service *DomainService) DeploymentHistory(all bool, group string, proc string) map[string]interface{} {
	report := make(map[string]interface{})

	log.Info("DeploymentHistory. all=[%b], group=[%s], proc=[%s]", all, group, proc)

	target := make([]fatima.FatimaPkgProc, 0)
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())
	if all {
		target = yamlConfig.GetAllProc(true)
	} else if len(group) > 0 {
		if strings.ToLower(group) == "opm" {
			report["system"] = web.SystemResponse{Code: 700, Message: "OPM group not permitted"}
			return report
		}
		target = yamlConfig.GetProcByGroup(group)
	} else {
		target = append(target, yamlConfig.GetProcByName(proc))
	}

	size := len(target)
	if size == 0 {
		report["system"] = web.SystemResponse{Code: 700, Message: "not found process"}
		return report
	}

	processHistoryDir := buildHistorySaveDir(service.fatimaRuntime.GetEnv(), proc)
	savedFileTimeMillisList, err := readFilesInDir(processHistoryDir)
	if err != nil {
		report["system"] = web.SystemResponse{Code: 700, Message: "not found deployment history"}
		return report
	}

	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]interface{})
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()

	history := make([]map[string]interface{}, 0)
	for _, deploymentFile := range savedFileTimeMillisList {
		m, e := readFileAsMap(fmt.Sprintf("%s/%d", processHistoryDir, deploymentFile))
		if e != nil {
			log.Warn("readFileAsMap : %s", e.Error())
			continue
		}
		m["deployment_time"] = deploymentFile
		history = append(history, m)
	}

	summary["message"] = fmt.Sprintf("total %d process history found", len(savedFileTimeMillisList))
	summary["history"] = history
	report["summary"] = summary
	return report
}

// readFileAsMap read deployment file to map
func readFileAsMap(deploymentFilePath string) (map[string]interface{}, error) {
	deployment := make(map[string]interface{})
	b, err := os.ReadFile(deploymentFilePath) // just pass the file name
	if err != nil {
		return deployment, fmt.Errorf("fail to read file %s : %s", deploymentFilePath, err.Error())
	}

	err = json.Unmarshal(b, &deployment)
	if err != nil {
		return deployment, fmt.Errorf("fail to unmarshal %s : %s", deploymentFilePath, err.Error())
	}

	return deployment, nil
}

func clearIcProcess(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) string {
	if proc == nil {
		return fmt.Sprintf("UNREGISTED PROCESS")
	}

	log.Warn("CLRIC PROCESS : %s", proc.GetName())

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("\nCLRIC PROCESS : %s\n", proc.GetName()))
	GetProcessMonitor().ResetICount(proc.GetName())
	return buffer.String()
}

func (service *DomainService) GetProcessReport(loc *time.Location, proc string) domain.ProcessReport {
	log.Info("GetProcessReport. proc=[%s]", proc)

	ctx := new(ProcessReportContext)
	ctx.proc = proc
	ctx.loc = loc
	ctx.fatimaRuntime = service.fatimaRuntime
	ctx.report.Package.Group = service.fatimaRuntime.GetPackaging().GetGroup()
	ctx.report.Package.Host = service.fatimaRuntime.GetPackaging().GetHost()
	ctx.report.Package.Name = service.fatimaRuntime.GetPackaging().GetName()

	ctx.wg.Add(5)
	go loadProcessDescription(ctx)
	go loadProcessStatus(ctx)
	go loadBatchJobs(ctx, service.GetCronsDir())
	go loadDeployment(ctx)
	go loadMonitoringTail(ctx)

	ctx.wg.Wait()

	return ctx.report
}

type ProcessReportContext struct {
	fatimaRuntime fatima.FatimaRuntime
	proc          string
	wg            sync.WaitGroup
	report        domain.ProcessReport
	loc           *time.Location
}

const (
	propProgramDesc = "program.desc"
)

func loadProcessDescription(ctx *ProcessReportContext) {
	defer ctx.wg.Done()

	propFile := filepath.Join(
		ctx.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(),
		builder.FatimaFolderApp,
		ctx.proc,
		fmt.Sprintf("application.%s.properties", ctx.fatimaRuntime.GetEnv().GetProfile()))

	params, err := readProperties(propFile)
	if err != nil {
		log.Warn("fail to read properties [%s] : %s", propFile, err.Error())
		return
	}

	desc, ok := params[propProgramDesc]
	if !ok {
		return
	}

	ctx.report.Description = desc
}

func readProperties(path string) (map[string]string, error) {
	resolved := make(map[string]string)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var line string
	var idx int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line = strings.Trim(scanner.Text(), " ")
		if strings.HasPrefix(line, "#") || len(line) < 3 {
			continue
		}
		idx = strings.Index(line, "#")
		if idx > 0 {
			if line[idx-1] == ' ' {
				line = line[:idx]
			}
		}
		idx = strings.Index(line, "=")
		if idx < 1 {
			continue
		}
		resolved[line[:idx]] = line[idx+1:]
	}

	return resolved, nil
}

func loadProcessStatus(ctx *ProcessReportContext) {
	defer ctx.wg.Done()
	ctx.report.Process.Name = ctx.proc

	if runtime.GOOS == "darwin" {
		getProcessStatusInDarwin(ctx)
	} else {
		getProcessStatusInLinux(ctx)
	}
}

func getProcessStatusInLinux(ctx *ProcessReportContext) {
	proc := GetProcessMonitor().GetProcess(ctx.proc, ctx.loc)
	ctx.report.Process.Name = proc.Name
	ctx.report.Process.Pid = proc.Pid
	ctx.report.Process.StartTime = proc.StartTime
	ctx.report.Process.Status = proc.Status
}

func getProcessStatusInDarwin(ctx *ProcessReportContext) {
	yamlConfig := builder.NewYamlFatimaPackageConfig(ctx.fatimaRuntime.GetEnv())

	processList := newProcessList(toGroupMap(yamlConfig.Groups))
	yamlConfig.OrderByGroup()

	for i, p := range yamlConfig.Processes {
		index := i
		item := p
		if p.Name == ctx.proc {
			processList.wg.Add(1)
			buildBasicProcessStatusDarwin(ctx.fatimaRuntime.GetEnv(), processList, index, item)
			break
		}
	}

	inspector.MeasureProcessStatus(processList.processes, ctx.loc)

	for _, p := range processList.processes {
		if p.Name == ctx.proc {
			ctx.report.Process.Name = p.Name
			ctx.report.Process.Pid = p.Pid
			ctx.report.Process.Status = p.Status
			ctx.report.Process.StartTime = p.StartTime
			break
		}
	}
}

func loadBatchJobs(ctx *ProcessReportContext, cronDir string) {
	defer ctx.wg.Done()

	file := filepath.Join(cronDir, buildCronJsonFilename(ctx.proc))
	b, err := os.ReadFile(file)
	if err != nil {
		return
	}

	var batchJob domain.CronJob
	err = json.Unmarshal(b, &batchJob)
	if err != nil {
		log.Warn("%s invalid json : %s", ctx.proc, string(b))
		return
	}

	if len(batchJob.Jobs) == 0 {
		return
	}

	reportJob := &ctx.report.BatchJobs
	reportJob.JobList = make([]domain.BatchItem, 0)

	for _, j := range batchJob.Jobs {
		item := domain.BatchItem{}
		item.Name = j.Name
		item.Desc = j.Desc
		item.Spec = j.Spec
		item.Sample = j.Sample
		reportJob.JobList = append(reportJob.JobList, item)
	}
}

type BatchJobs struct {
	JobList []BatchItem `json:"job_list"`
}

type BatchItem struct {
	Name string `json:"name"`
	Spec string `json:"spec"`
	Desc string `json:"desc"`
}

const (
	deploymentJsonFile = "deployment.json"
)

func loadDeployment(ctx *ProcessReportContext) {
	defer ctx.wg.Done()

	deploymentFile := filepath.Join(ctx.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(),
		builder.FatimaFolderApp,
		ctx.proc,
		deploymentJsonFile)
	file, err := os.ReadFile(deploymentFile)
	if err != nil {
		log.Warn("readfile err : %s", err.Error())
		return
	}

	deployment := AppDeployment{}
	err = json.Unmarshal(file, &deployment)
	if err != nil {
		log.Warn("json unmarshal err : %s\n", err.Error())
		return
	}

	ctx.report.Deployment.BuildTime = deployment.Build.BuildTime
	ctx.report.Deployment.BuildUser = deployment.Build.BuildUser
	ctx.report.Deployment.GitBranch = deployment.Build.Git.Branch
	ctx.report.Deployment.GitCommit = deployment.Build.Git.Commit
	ctx.report.Deployment.GitCommitMessage = deployment.Build.Git.Message
}

func loadMonitoringTail(ctx *ProcessReportContext) {
	defer ctx.wg.Done()

	pid := readPidFromFile(ctx.fatimaRuntime.GetEnv(), ctx.proc)
	if pid == 0 {
		return
	}

	monitorFile := filepath.Join(ctx.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(),
		builder.FatimaFolderApp,
		ctx.proc,
		builder.FatimaFolderProc,
		"monitor",
		fmt.Sprintf("%s.%d.monitor", ctx.proc, pid))

	log.Debug("monitor file : %s", monitorFile)
	ctx.report.Monitoring.LogTail = getLastLineWithSeek(monitorFile, 100)

}

func getLastLineWithSeek(filepath string, lineCount int) string {
	if lineCount < 1 {
		return ""
	}

	fileHandle, err := os.Open(filepath)
	if err != nil {
		log.Warn("fail to open %s : %s", filepath, err.Error())
		return ""
	}
	defer fileHandle.Close()

	var cursor int64 = 0
	stat, _ := fileHandle.Stat()
	filesize := stat.Size()
	if filesize < 1024 {
		// if under 1k, just return all
		b, _ := os.ReadFile(filepath)
		return string(b)
	}

	for {
		cursor -= 1
		fileHandle.Seek(cursor, io.SeekEnd)

		char := make([]byte, 1)
		fileHandle.Read(char)

		if cursor != -1 && (char[0] == 10 || char[0] == 13) {
			// stop if we find a line
			lineCount -= 1
			if lineCount == 0 {
				break
			}
		}

		if cursor == -filesize {
			// stop and return whole if we are at the begining
			b, _ := os.ReadFile(filepath)
			return string(b)
		}
	}

	remain := -cursor
	pos := filesize - remain
	log.Debug("filesize=[%d], cursor=[%d], pos=[%d]", filesize, cursor, pos)

	buf := make([]byte, remain)
	_, err = fileHandle.ReadAt(buf, pos)
	if err != nil {
		return ""
	}
	return string(buf)
}

type AppDeployment struct {
	Process     string          `json:"process"`
	ProcessType string          `json:"process_type,omitempty"`
	Build       DeploymentBuild `json:"build,omitempty"`
}

type DeploymentBuild struct {
	Git       DeploymentBuildGit `json:"git,omitempty"`
	BuildTime string             `json:"time,omitempty"`
	BuildUser string             `json:"user,omitempty"`
}

type DeploymentBuildGit struct {
	Branch  string `json:"branch"`
	Commit  string `json:"commit"`
	Message string `json:"message,omitempty"`
}

func StartDeadProcessesWithWeightGroup(fatimaRuntime fatima.FatimaRuntime) {
	yamlConfig := builder.NewYamlFatimaPackageConfig(fatimaRuntime.GetEnv())

	targetProcList := make([]fatima.FatimaPkgProc, 0)
	for _, proc := range yamlConfig.Processes {
		if domain.IsManagedOpmProcess(proc) {
			continue // skip OPM
		}

		if !domain.IsStartingTarget(fatimaRuntime, proc.GetStartMode()) {
			log.Info("skip start process : %s", proc.GetName())
			continue
		}

		targetProcList = append(targetProcList, proc)
	}
	startProcessWithWeightGroup(fatimaRuntime, targetProcList, processExecuteSerial)
}

func startProcessWithWeightGroup(fatimaRuntime fatima.FatimaRuntime,
	targetProcList []fatima.FatimaPkgProc,
	executeFunc ProcessActionFunc) string {
	platformImpl := platform.OSPlatform{}
	procList, err := platformImpl.GetProcesses()
	if err != nil {
		return ""
	}

	weightGroups := make(map[int][]fatima.FatimaPkgProc)

	// gather target process list as weight group
	for _, p := range targetProcList {
		pid := GetPid(fatimaRuntime.GetEnv(), p)
		if pid > 0 {
			if domain.ExistInProcessListWithPid(procList, pid) {
				continue // skip alive process
			}
		}
		deadProcessList, ok := weightGroups[p.GetWeight()]
		if !ok {
			deadProcessList = make([]fatima.FatimaPkgProc, 0)
		}
		deadProcessList = append(deadProcessList, p)
		weightGroups[p.GetWeight()] = deadProcessList
	}

	// sort group with weight
	weightList := make([]int, 0)
	for weightKey, _ := range weightGroups {
		weightList = append(weightList, weightKey)
	}
	sort.Sort(ByWeightDesc(weightList))

	// launch process by weight group
	var buffer bytes.Buffer
	for _, weight := range weightList {
		weightedProcList := weightGroups[weight]
		log.Info("weight %d : [%s]", weight, extractProcessNameList(weightedProcList))
		launchedProcList, output := executeFunc(fatimaRuntime.GetEnv(), weightedProcList)
		buffer.WriteString(output)
		if weight > 0 {
			// we don't need checking weight 0 process group
			err = checkProcessAliveWithDeadline(fatimaRuntime.GetEnv(), launchedProcList, time.Second*3)
			if err != nil {
				log.Error("checkProcessAliveWithDeadline failed : %s", err.Error())
			}
		}
	}
	return buffer.String()
}

type ProcessActionFunc func(env fatima.FatimaEnv, procList []fatima.FatimaPkgProc) (ProcessBriefInfo, string)

func processExecuteSerial(env fatima.FatimaEnv, procList []fatima.FatimaPkgProc) (ProcessBriefInfo, string) {
	launchedProcList := make([]ProcessNameAndPid, 0)
	for _, proc := range procList {
		item := ProcessNameAndPid{ProcName: proc.GetName()}
		var err error
		item.Pid, err = ExecuteProgram(env, proc)
		if err != nil {
			continue
		}
		launchedProcList = append(launchedProcList, item)
	}
	return launchedProcList, ""
}

func processExecuteAsync(env fatima.FatimaEnv, procList []fatima.FatimaPkgProc) (ProcessBriefInfo, string) {
	launchedProcList := make([]ProcessNameAndPid, 0)

	size := len(procList)
	if size == 0 {
		return launchedProcList, ""
	}

	mu := sync.Mutex{}
	var buffer bytes.Buffer
	cyBarrier := lib.NewCyclicBarrier(size, nil)
	for _, v := range procList {
		t := v
		cyBarrier.Dispatch(func() {
			pid, output := startProcess(env, t)
			mu.Lock()
			buffer.WriteString(output)
			buffer.WriteByte('\n')
			if pid > 0 {
				item := ProcessNameAndPid{ProcName: t.GetName()}
				item.Pid = pid
				launchedProcList = append(launchedProcList, item)
			}
			mu.Unlock()
		})
	}
	cyBarrier.Wait()
	return launchedProcList, buffer.String()
}

func stopProcessWithWeightGroup(fatimaRuntime fatima.FatimaRuntime,
	targetProcList []fatima.FatimaPkgProc,
	executeFunc ProcessActionFunc) string {
	weightGroups := make(map[int][]fatima.FatimaPkgProc)

	// gather target process list as weight group
	for _, p := range targetProcList {
		processList, ok := weightGroups[p.GetWeight()]
		if !ok {
			processList = make([]fatima.FatimaPkgProc, 0)
		}
		processList = append(processList, p)
		weightGroups[p.GetWeight()] = processList
	}

	// sort group with weight
	weightList := make([]int, 0)
	for weightKey, _ := range weightGroups {
		weightList = append(weightList, weightKey)
	}
	sort.Sort(ByWeightAsc(weightList))

	// handle process by weight group
	var buffer bytes.Buffer
	for _, weight := range weightList {
		weightedProcList := weightGroups[weight]
		log.Info("weight %d : [%s]", weight, extractProcessNameList(weightedProcList))
		launchedProcList, output := executeFunc(fatimaRuntime.GetEnv(), weightedProcList)
		buffer.WriteString(output)
		if launchedProcList.IsAllDead() {
			continue
		}
		time.Sleep(time.Second)
	}
	return buffer.String()
}

func processTerminateAsync(env fatima.FatimaEnv, procList []fatima.FatimaPkgProc) (ProcessBriefInfo, string) {
	launchedProcList := make([]ProcessNameAndPid, 0)

	size := len(procList)
	if size == 0 {
		return launchedProcList, ""
	}

	mu := sync.Mutex{}
	var buffer bytes.Buffer
	cyBarrier := lib.NewCyclicBarrier(size, nil)
	for _, v := range procList {
		t := v
		cyBarrier.Dispatch(func() {
			pid, output := stopProcess(env, t)
			mu.Lock()
			buffer.WriteString(output)
			if pid > 0 {
				item := ProcessNameAndPid{ProcName: t.GetName()}
				item.Pid = pid
				launchedProcList = append(launchedProcList, item)
			}
			mu.Unlock()
		})
	}
	cyBarrier.Wait()
	return launchedProcList, buffer.String()
}

func extractProcessNameList(list []fatima.FatimaPkgProc) string {
	nameList := make([]string, 0)
	for _, item := range list {
		nameList = append(nameList, item.GetName())
	}
	return strings.Join(nameList, ",")
}

type ProcessNameAndPid struct {
	ProcName string
	Pid      int
}

type ProcessBriefInfo []ProcessNameAndPid

func (p ProcessBriefInfo) IsAllDead() bool {
	for _, proc := range p {
		if proc.Pid > 0 {
			return false
		}
	}
	return true
}

func getMaxStartingSeconds(env fatima.FatimaEnv, procList []ProcessNameAndPid) time.Duration {
	yamlConfig := builder.NewYamlFatimaPackageConfig(env)
	maxWaitSec := 0
	for _, proc := range procList {
		p := yamlConfig.GetProcByName(proc.ProcName)
		if p == nil {
			continue
		}
		if p.GetStartSec() > maxWaitSec {
			maxWaitSec = p.GetStartSec()
		}
	}

	return time.Duration(max(1, maxWaitSec)) * time.Second
}

// checkProcessAliveWithDeadline check process in list alive or not
func checkProcessAliveWithDeadline(env fatima.FatimaEnv, procList []ProcessNameAndPid, deadline time.Duration) error {
	if len(procList) == 0 {
		return nil
	}

	time.Sleep(getMaxStartingSeconds(env, procList)) // initial sleep

	lastFailedProcName := ""
	start := time.Now()
	for {
		if time.Since(start) > deadline {
			return fmt.Errorf("deadline exceeded. fail proc=%s", lastFailedProcName)
		}

		fine := true
		for _, procItem := range procList {
			if procItem.Pid == 0 {
				// skip. (maybe it has own start script)
				continue
			}
			log.Trace("check running %s:%d", procItem.ProcName, procItem.Pid)
			running := inspector.CheckProcessRunningByPid(procItem.ProcName, procItem.Pid)
			if !running {
				fine = false
				lastFailedProcName = procItem.ProcName
				log.Trace("not running %s:%d", procItem.ProcName, procItem.Pid)
				break
			}
		}

		if fine {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	log.Debug("checkProcessAliveWithDeadline finished : %v", procList)
	return nil
}

type ByWeightDesc []int

func (a ByWeightDesc) Len() int           { return len(a) }
func (a ByWeightDesc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByWeightDesc) Less(i, j int) bool { return a[i] > a[j] }

type ByWeightAsc []int

func (a ByWeightAsc) Len() int           { return len(a) }
func (a ByWeightAsc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByWeightAsc) Less(i, j int) bool { return a[i] < a[j] }
