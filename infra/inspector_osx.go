//go:build darwin
// +build darwin

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

package infra

import (
	"bufio"
	"fmt"
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/lib"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/web"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SystemInspector struct {
	fatimaRuntime fatima.FatimaRuntime
}

func NewSystemInspector(fatimaRuntime fatima.FatimaRuntime) SystemInspector {
	inspector := SystemInspector{}
	inspector.fatimaRuntime = fatimaRuntime
	return inspector
}

func (i SystemInspector) CheckProcessRunningByPid(procName string, pid int) bool {
	command := fmt.Sprintf("ps -ef | grep %d | grep -v grep | awk '{print $2}'", pid)
	out, err := lib.ExecuteShell(command)
	if err != nil {
		log.Warn("fail to execute command : %s", err.Error())
		return false
	}

	trimmed := strings.Trim(out, "\r\n\t ")
	found := false
	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	for scanner.Scan() {
		foundPid, err := strconv.Atoi(scanner.Text())
		if err != nil {
			continue
		}
		if foundPid == pid {
			found = true
			break
		}
	}

	return found
}

func (i SystemInspector) MeasureProcessStatus(list []*domain.ProcessInfo, loc *time.Location) {
	cmd := fmt.Sprintf("ps -v -o etime -u %d",
		i.fatimaRuntime.GetEnv().GetSystemProc().GetUid())
	out, err := lib.ExecuteCommand(cmd)
	if err != nil {
		log.Warn("fail to cmd run : %s", err.Error())
	}

	analyzeProcess(list, out, loc)

	for i := 0; i < len(list); i++ {
		info := list[i]
		countFD(info)
	}
}

func countFD(info *domain.ProcessInfo) {
	if info.Status != domain.ProcStatusAlive {
		return
	}

	cmd := fmt.Sprintf("lsof -p %s", info.Pid)
	out, err := lib.ExecuteCommand(cmd)
	if err != nil {
		return
	}

	fdCount := 0
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			continue
		}
		fdCount = fdCount + 1
	}
	info.FDCount = strconv.Itoa(fdCount - 1)
}

func analyzeProcess(list []*domain.ProcessInfo, ps string, loc *time.Location) {
	scanner := bufio.NewScanner(strings.NewReader(ps))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			continue
		}
		trimmed := strings.Trim(line, " ")
		s := regexp.MustCompile("\\s+").Split(trimmed, -1)
		if len(s) < 14 {
			continue
		}
		mappingProcessInfo(s, list, loc)
	}
}

func mappingProcessInfo(parts []string, list []*domain.ProcessInfo, loc *time.Location) {
	/*
		SKTX1100282MN03:jupiter 1100282$ ps -v -o etime -u 501
		  PID STAT      TIME  SL  RE PAGEIN      VSZ    RSS   LIM     TSIZ  %CPU %MEM COMMAND              ELAPSED
		96274 S    146:33.07   0   0      0 11998344 1431196     -        0   0.0  8.5 /Applications/Go 07-00:59:49
		13769 S    119:18.98   0   0      0  7111836 1161400     -        0   0.5  6.9 /Applications/Sl 09-07:24:46
	*/
	for i := 0; i < len(list); i++ {
		info := list[i]
		if info.Pid == parts[0] {
			v, _ := strconv.Atoi(parts[7])
			info.Memory = lib.FormatBytes(v * 1024)
			info.CpuUtil = parts[10]
			info.StartTime = convertElapsedTime(parts[len(parts)-1], loc)
			return
		}
	}
}

func convertElapsedTime(v string, loc *time.Location) string {
	// e.g)
	//    03:29:51
	//       39:12
	// 01-08:01:45
	var day, hour, minute, seconds int
	i := strings.Index(v, "-")
	if i > 0 {
		day, _ = strconv.Atoi(v[:i])
		day = -day
		v = v[i+1:]
		//time = time.substring(idx+1);
	}
	i = strings.Index(v, ":")
	sub := v[i+1:]
	i2 := strings.Index(sub, ":")
	if i2 < 0 {
		// mm:ss
		minute, _ = strconv.Atoi(v[:2])
		seconds, _ = strconv.Atoi(v[3:])
	} else {
		// hh:mm:ss
		hour, _ = strconv.Atoi(v[:2])
		minute, _ = strconv.Atoi(v[3:5])
		seconds, _ = strconv.Atoi(v[6:])
	}

	return time.Now().In(loc).
		AddDate(0, 0, day).
		Add(time.Duration(-hour) * time.Hour).
		Add(time.Duration(-minute) * time.Minute).
		Add(time.Duration(-seconds) * time.Second).
		Format(web.TIME_YYYYMMDDHHMMSS)
}
