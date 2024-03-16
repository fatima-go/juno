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
	"fmt"
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/lib"
	"github.com/fatima-go/fatima-log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	actionCategoryArgFormat = "actionCategory=%s"
)

func GetPid(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) int {
	grep := strings.Trim(proc.GetGrep(), "\n\r\t ")
	if len(grep) == 0 {
		return readPidFromFile(env, proc.GetName())
	}
	return GetPidByGrep(grep)
}

func readPidFromFile(env fatima.FatimaEnv, procName string) int {
	filePath := filepath.Join(
		env.GetFolderGuide().GetFatimaHome(),
		builder.FatimaFolderApp,
		procName,
		builder.FatimaFolderProc,
		procName+".pid")

	data, err := os.ReadFile(filePath)
	if err != nil {
		//log.Warn("fail to read proc[%s] pid file : %s", procName, err.Error())
		return 0
	}
	var pid = 0
	pid, err = strconv.Atoi(strings.Trim(string(data), "\r\n"))
	if err != nil {
		log.Warn("fail to parse proc[%s] pid value to int : %s", procName, err.Error())
		return 0
	}

	return pid
}

func GetPidByGrep(grep string) int {
	target := strings.Replace(grep, "-", "\\-", -1)
	target = strings.Replace(target, "\"", "\\\"", -1)
	command := fmt.Sprintf("ps -ef | grep \"%s\" | grep -v grep | awk '{print $2}'", target)
	out, err := lib.ExecuteShell(command)
	if err != nil {
		log.Warn("fail to execute command : %s", err.Error())
		return 0
	}

	s := strings.Trim(out, "\r\n\t ")
	if len(s) < 1 {
		return 0
	}

	pid, err := strconv.Atoi(s)
	if err != nil {
		log.Warn("invalid integer pid : %s", err.Error())
		return 0
	}
	return pid
}

func hasExecutingShell(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) bool {
	if len(proc.GetGrep()) > 0 || len(proc.GetPath()) > 0 {
		return false
	}

	filePath := filepath.Join(
		env.GetFolderGuide().GetFatimaHome(),
		builder.FatimaFolderApp,
		proc.GetName(),
		proc.GetName()+".sh")

	_, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	return true
}

func KillProgram(proc string, pid int) error {
	log.Warn("try to kill %s [%d]", proc, pid)
	GetProcessMonitor().ProcessStop(proc)
	err := syscall.Kill(pid, syscall.SIGTERM)
	if err != nil {
		log.Warn("kill %s(%d) fail. err=%s", proc, pid, err.Error())
	} else {
		log.Warn("%s(%d) was killed", proc, pid)
	}
	return err
}

func ExecuteProgram(env fatima.FatimaEnv, proc fatima.FatimaPkgProc) (int, error) {
	GetProcessMonitor().ProcessStart(proc.GetName())
	workingDir := filepath.Join(
		env.GetFolderGuide().GetFatimaHome(),
		builder.FatimaFolderApp,
		proc.GetName())

	if hasExecutingShell(env, proc) {
		log.Info("executing java fatima program : %s", proc.GetName())
		shell := filepath.Join(workingDir, proc.GetName()+".sh")
		cmd := exec.Command(shell)
		cmd.Dir = workingDir
		log.Info("Working Dir : %s", workingDir)
		err := cmd.Start()
		if err != nil {
			return 0, err
		}
		return grepJavaFatimaProgramPid(proc), nil
	} else {
		log.Info("executing native program : [%s], [%s]", proc.GetName(), proc.GetPath())
		var cmd *exec.Cmd
		if len(proc.GetPath()) > 0 {
			cmd = exec.Command(proc.GetPath())
		} else {
			executing := filepath.Join(workingDir, proc.GetName())
			log.Debug("executing : %s", executing)
			cmd = exec.Command(executing)
		}
		cmd.Dir = workingDir
		log.Info("Working Dir : %s", workingDir)
		err := cmd.Start()
		if err != nil {
			return 0, err
		}

		return cmd.Process.Pid, nil
	}
}

// ExecuteProgramWithActionCategory 는 카테고리 정보를 넘겨서 프로그램을 수행한다.
// 자바 프로세스로 예상되는 쉘 스크립트 수행은 지원하지 않는다
func ExecuteProgramWithActionCategory(env fatima.FatimaEnv, proc fatima.FatimaPkgProc, actionCategory string) (int, error) {
	if len(actionCategory) == 0 || hasExecutingShell(env, proc) {
		return ExecuteProgram(env, proc)
	}

	GetProcessMonitor().ProcessStart(proc.GetName())
	workingDir := filepath.Join(env.GetFolderGuide().GetFatimaHome(), builder.FatimaFolderApp, proc.GetName())
	procPath := proc.GetPath()
	if len(procPath) == 0 {
		procPath = filepath.Join(workingDir, proc.GetName())
	}

	cmd := exec.Command(procPath, fmt.Sprintf(actionCategoryArgFormat, actionCategory))
	cmd.Dir = workingDir
	log.Info("Working Dir : %s", workingDir)
	err := cmd.Start()
	if err != nil {
		return 0, err
	}

	log.Info("executing native program with action category, procName: [%s], procPath: [%s], actionCategory: [%s]",
		proc.GetName(), procPath, actionCategory)

	return cmd.Process.Pid, nil
}

func grepJavaFatimaProgramPid(proc fatima.FatimaPkgProc) int {
	time.Sleep(200 * time.Millisecond)

	grep := fmt.Sprintf("psname=%s", proc.GetName())
	command := fmt.Sprintf("ps -ef | grep \"%s\" | grep -v grep | awk '{print $2}'", grep)
	out, err := lib.ExecuteShell(command)
	if err != nil {
		log.Warn("fail to execute : %s", err.Error())
		return 0
	} else if len(out) < 1 {
		// not found
		return 0
	}

	var pid int
	pid, err = strconv.Atoi(strings.Trim(out, "\r\n"))
	if err != nil {
		log.Warn("fail to convert to int : %s[%s]", out, err.Error())
		return 0
	}

	return pid
}

func grepNativeProgramPid(cmd *exec.Cmd, proc fatima.FatimaPkgProc) int {
	if len(proc.GetPath()) == 0 {
		return cmd.Process.Pid
	}
	time.Sleep(200 * time.Millisecond)

	command := fmt.Sprintf("ps -ef | grep \"%s\" | grep -v grep | awk '{print $2}'", proc.GetGrep())
	out, err := lib.ExecuteShell(command)
	if err != nil {
		log.Warn("fail to execute : %s", err.Error())
		return 0
	} else if len(out) < 1 {
		// not found
		return 0
	}

	var pid int
	pid, err = strconv.Atoi(out)
	if err != nil {
		log.Warn("fail to convert to int : %s[%s]", out, err.Error())
		return 0
	}

	return pid
}
