//go:build linux
// +build linux

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
	"fmt"
	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/lib"
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

// http://man7.org/linux/man-pages/man5/proc.5.html

var Hertz int
var NumberOfProcessor int

type SystemInspector struct {
	fatimaRuntime fatima.FatimaRuntime
}

func NewSystemInspector(fatimaRuntime fatima.FatimaRuntime) SystemInspector {
	inspector := SystemInspector{}
	inspector.fatimaRuntime = fatimaRuntime

	Hertz = 100
	out, err := lib.ExecuteCommand("getconf CLK_TCK")
	if err == nil {
		parsed, err := strconv.Atoi(strings.TrimSpace(out))
		if err == nil {
			Hertz = parsed
		}
	}

	return inspector
}

func (i SystemInspector) CheckProcessRunningByPid(procName string, pid int) bool {
	statusFile := fmt.Sprintf("/proc/%d/status", pid)

	contents, err := os.ReadFile(statusFile)
	if err != nil {
		return false
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		switch fields[0] {
		case "Name:":
			if fields[1] == "java" {
				return checkJavaPsName(procName, pid)
			}
			if procName != fields[1] {
				// invalid(another) process
				return false
			}
			return true
		}
	}

	return false
}

func checkJavaPsName(procName string, pid int) bool {
	statusFile := fmt.Sprintf("/proc/%d/cmdline", pid)
	contents, err := os.ReadFile(statusFile)
	if err != nil {
		return false
	}
	match := strings.Contains(string(contents), "psname="+procName)
	if match {
		return match
	}

	return strings.Contains(string(contents), "pscategory="+procName)
}

func (i SystemInspector) MeasureProcessStatus(list []*domain.ProcessInfo, loc *time.Location) {
	var wg sync.WaitGroup

	for i := 0; i < len(list); i++ {
		proc := list[i]
		if proc.Status != domain.PROC_STATUS_ALIVE {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			fillRSS(proc)
			countFD(proc)
			countThread(proc)
			fillStat(proc, loc)
		}()
	}

	wg.Wait()
}

func fillStat(proc *domain.ProcessInfo, loc *time.Location) {
	statusFile := fmt.Sprintf("/proc/%s/stat", proc.Pid)

	contents, err := os.ReadFile(statusFile)
	if err != nil {
		return
	}
	fields := strings.Fields(string(contents))
	fillStarttime(fields[21], proc, loc)
	fillCpuUsage(fields[13], fields[14], proc)
}

// ps -p 1 -p 11610 -wo user,pid,%cpu,%mem,vsz,rss,tty,stat,lstart,cmd
// https://stackoverflow.com/questions/16726779/how-do-i-get-the-total-cpu-usage-of-an-application-from-proc-pid-stat/16736599
func fillStarttime(factor string, proc *domain.ProcessInfo, loc *time.Location) {
	starttime, _ := strconv.Atoi(factor)
	uptime := getSystemUptime()
	if uptime == 0 {
		return
	}
	seconds := uptime - (starttime / Hertz)
	proc.StartTime = time.Now().In(loc).
		Add(time.Second * time.Duration(-seconds)).
		Format(web.TIME_YYYYMMDDHHMMSS)
}

// https://stackoverflow.com/questions/1420426/how-to-calculate-the-cpu-usage-of-a-process-by-pid-in-linux-from-c/1424556#1424556
func fillCpuUsage(factor1 string, factor2 string, proc *domain.ProcessInfo) {
	utime, _ := strconv.Atoi(factor1)
	stime, _ := strconv.Atoi(factor2)
	procTime1 := utime + stime

	user, nice, system, idle := getSystemCpuStat()
	totalSystemCpu1 := user + nice + system + idle

	// sleep
	time.Sleep(time.Millisecond * 500)

	statusFile := fmt.Sprintf("/proc/%s/stat", proc.Pid)
	contents, err := os.ReadFile(statusFile)
	if err != nil {
		return
	}
	fields := strings.Fields(string(contents))
	utime, _ = strconv.Atoi(fields[13])
	stime, _ = strconv.Atoi(fields[14])
	procTime2 := utime + stime
	if procTime2-procTime1 == 0 {
		proc.CpuUtil = "0.0"
		return
	}

	user, nice, system, idle = getSystemCpuStat()
	totalSystemCpu2 := user + nice + system + idle
	if totalSystemCpu2-totalSystemCpu1 == 0 {
		proc.CpuUtil = "0.0"
		return
	}

	//[myapp] 192, 16, 208
	//[myapp] 87683, 4306, 111505, 263282209, 263485703
	//[myapp] 193, 16, 209
	//[myapp] 87683, 4306, 111505, 263282308, 263485802
	// (number of processors) * (proc_times2 - proc_times1) * 100 / (float) (total_cpu_usage2 - total_cpu_usage1)
	var util float64
	util = float64(procTime2-procTime1) * 100 / float64(totalSystemCpu2-totalSystemCpu1)
	proc.CpuUtil = fmt.Sprintf("%.1f", util*float64(runtime.NumCPU()))
}

func getSystemUptime() int {
	contents, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(contents))
	f, err := strconv.ParseFloat(fields[0], 64)
	if err == nil {
		return int(f)
	}
	return 0
}

// return user, nice, system, idle
func getSystemCpuStat() (user int, nice int, system int, idle int) {
	contents, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	//cpu  81352 4074 104125 262143413 128057 0 1673 0 0 0
	//cpu0 38794 2022 49449 131119559 23457 0 350 0 0 0
	//cpu1 42558 2052 54676 131023853 104600 0 1323 0 0 0
	fields := strings.Fields(lines[0]) // always first line means cpu
	user, _ = strconv.Atoi(fields[1])
	nice, _ = strconv.Atoi(fields[2])
	system, _ = strconv.Atoi(fields[3])
	idle, _ = strconv.Atoi(fields[4])
	return
}

func fillRSS(proc *domain.ProcessInfo) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("PANIC", r)
			return
		}
	}()

	statusFile := fmt.Sprintf("/proc/%s/status", proc.Pid)

	contents, err := os.ReadFile(statusFile)
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && len(fields[0]) > 0 {
			switch fields[0] {
			case "VmRSS:":
				// e.g) 16592 kB
				v, _ := strconv.Atoi(fields[1])
				proc.Memory = lib.FormatBytes(v * 1024)
				return
			}
		}
	}
}

func countFD(info *domain.ProcessInfo) {
	if info.Status != domain.PROC_STATUS_ALIVE {
		return
	}

	filePath := filepath.Join(
		"/proc",
		info.Pid,
		"fd")

	files, err := os.ReadDir(filePath)
	if err == nil {
		info.FDCount = fmt.Sprintf("%d", len(files))
	}
}

func countThread(info *domain.ProcessInfo) {
	if info.Status != domain.PROC_STATUS_ALIVE {
		return
	}

	filePath := filepath.Join(
		"/proc",
		info.Pid,
		"task")

	files, err := os.ReadDir(filePath)
	if err == nil {
		info.Thread = fmt.Sprintf("%d", len(files))
	}
}
