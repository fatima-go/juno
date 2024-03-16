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
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/lib"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/web"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
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
	cyBarrier := lib.NewCyclicBarrier(size, nil)
	for _, v := range target {
		t := v
		cyBarrier.Dispatch(func() {
			buffer.WriteString(startProcess(service.fatimaRuntime.GetEnv(), t))
		})
	}
	cyBarrier.Wait()
	buffer.WriteByte('\n')
	summary["message"] = buffer.String()
	report["summary"] = summary
	return report
}

func startProcess(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) string {
	return startProcessWithActionCategory(env, proc, "")
}

func startProcessWithActionCategory(env fatima.FatimaEnv, proc fatima.FatimaPkgProc, actionCategory string) string {
	if proc == nil {
		return fmt.Sprintf("UNREGISTED PROCESS")
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
		return buffer.String()
	}

	var (
		childPid int
		err      error
	)
	if len(actionCategory) == 0 {
		childPid, err = ExecuteProgram(env, proc)
	} else {
		childPid, err = ExecuteProgramWithActionCategory(env, proc, actionCategory)
	}

	if err != nil {
		buffer.WriteString(fmt.Sprintf("FAIL TO EXECUTE : %s", err.Error()))
	} else {
		buffer.WriteString(fmt.Sprintf("SUCCESS : pid=%d", childPid))
		GetProcessMonitor().ResetICount(proc.GetName())
	}

	return buffer.String()
}

func (service *DomainService) StopProcess(all bool, group string, proc string) map[string]interface{} {
	report := make(map[string]interface{})

	log.Info("StopProcess. all=[%b], group=[%s], proc=[%s]", all, group, proc)

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
			buffer.WriteString(stopProcess(service.fatimaRuntime.GetEnv(), t))
		})
	}
	cyBarrier.Wait()

	summary["message"] = buffer.String()
	report["summary"] = summary
	return report
}

func stopProcess(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) string {
	if proc == nil {
		return fmt.Sprintf("UNREGISTED PROCESS")
	}

	log.Warn("TRY TO STOP PROCESS : %s", proc.GetName())

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("\nSTOP PROCESS : %s\n", proc.GetName()))

	comp := strings.ToLower(proc.GetName())
	if comp == "jupiter" || comp == "juno" {
		log.Warn("%s is not permitted for killing", proc.GetName())
		buffer.WriteString(fmt.Sprintf("%s is not permitted for killing", proc.GetName()))
		return buffer.String()
	}

	pid := GetPid(env, proc)
	if pid < 1 || !inspector.CheckProcessRunningByPid(proc.GetName(), pid) {
		buffer.WriteString("NOT RUNNING\n")
	} else {
		executeGoaway(env, proc, pid)
		err := KillProgram(proc.GetName(), pid)
		if err != nil {
			buffer.WriteString(fmt.Sprintf("FAIL TO KILL %s[%d] : %s", proc.GetName(), pid, err.Error()))
		} else {
			buffer.WriteString(fmt.Sprintf("KILLED %d\n", pid))
		}
	}

	return buffer.String()
}

// execute "goaway.sh"
func executeGoaway(env fatima.FatimaEnv, proc fatima.FatimaPkgProc, pid int) {
	sendGoawaySignal(proc, pid)

	if proc == nil {
		return
	}

	path := filepath.Join(env.GetFolderGuide().GetFatimaHome(), "app", proc.GetName(), shellGoaway)
	if _, err := os.Stat(path); err != nil {
		return
	}

	GetProcessMonitor().ProcessStop(proc.GetName())
	log.Info("executing %s for: %s", shellGoaway, path)
	cmd := exec.Command("/bin/sh", "-c", path)
	err := cmd.Run()
	if err != nil {
		log.Error("fail to exec goaway[%s] : %s", path, err.Error())
	}

	log.Info("goaway finished")
	return
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
