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

package engine

import (
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/builder/platform"
	"github.com/fatima-go/fatima-core/monitor"
	"github.com/fatima-go/fatima-log"
	. "github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/service"
)

func NewSystemBase(fatimaRuntime fatima.FatimaRuntime) *SystemBase {
	server := new(SystemBase)
	server.fatimaRuntime = fatimaRuntime
	return server
}

type SystemBase struct {
	fatimaRuntime fatima.FatimaRuntime
	sigs          chan os.Signal
}

func (system *SystemBase) Initialize() bool {
	log.Info("SystemBase Initialize()")

	service.PrepareService(system.fatimaRuntime)
	system.checkLogLevelFile()

	hourlyMaintainTick := time.NewTicker(time.Hour * 1)
	go func() {
		for range hourlyMaintainTick.C {
			maintainRevision(system.fatimaRuntime.GetEnv())
		}
	}()

	if runtime.GOOS == "linux" {
		go func() {
			// start first measuring
			service.GetProcessMonitor().WatchProcesses()
		}()

		SecondTick := time.NewTicker(time.Second)
		go func() {
			for range SecondTick.C {
				service.GetProcessMonitor().WatchProcesses()
			}
		}()
	}

	system.sigs = make(chan os.Signal, 1)
	signal.Notify(system.sigs, syscall.SIGCHLD)

	go func() {
		for true {
			<-system.sigs

			var (
				status syscall.WaitStatus
				usage  syscall.Rusage
			)
			syscall.Wait4(-1, &status, syscall.WNOHANG, &usage)
		}
	}()

	return true
}

func (system *SystemBase) checkLogLevelFile() {
	cfmFolder := filepath.Join(
		system.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(),
		FOLDER_PACKAGE,
		FOLDER_CFM)
	ensureDirectory(cfmFolder, true)
	filePath := filepath.Join(cfmFolder, FILE_LOG_LEVEL)

	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			//ioutil.WriteFile(filePath, []byte(strconv.Itoa(monitor.HA_STATUS_STANDBY)), 0644)
			log.Warn("create default loglevel files...")
			system.syncLogLevels(filePath)
			return
		}
		log.Warn("fail to load loglevel files : %s", err.Error())
	}
}

func (system *SystemBase) Bootup() {
	log.Info("SystemBase Bootup()")

	go func() {
		startDeadProcesses(system.fatimaRuntime)
	}()
}

func (system *SystemBase) SystemHAStatusChanged(newHAStatus monitor.HAStatus) {
	log.Info("notified SystemHAStatusChanged : %s", newHAStatus)

	yamlConfig := builder.NewYamlFatimaPackageConfig(system.fatimaRuntime.GetEnv())

	platformImpl := platform.OSPlatform{}
	procList, err := platformImpl.GetProcesses()
	if err != nil {
		return
	}

	for _, p := range yamlConfig.Processes {
		if IsManagedOpmProcess(p) {
			continue // skip OPM
		}

		if p.GetStartMode() != fatima.StartModeByHA {
			continue
		}

		pid := service.GetPid(system.fatimaRuntime.GetEnv(), p)
		if pid > 0 {
			if ExistInProcessListWithPid(procList, pid) {
				if newHAStatus == monitor.HA_STATUS_STANDBY {
					service.KillProgram(p.GetName(), pid)
				}
			} else if newHAStatus == monitor.HA_STATUS_ACTIVE {
				service.ExecuteProgram(system.fatimaRuntime.GetEnv(), p)
			}
		} else if newHAStatus == monitor.HA_STATUS_ACTIVE {
			service.ExecuteProgram(system.fatimaRuntime.GetEnv(), p)
		}
	}
}

func (system *SystemBase) SystemPSStatusChanged(newPSStatus monitor.PSStatus) {
	log.Info("notified SystemPSStatusChanged : %s", newPSStatus)
	yamlConfig := builder.NewYamlFatimaPackageConfig(system.fatimaRuntime.GetEnv())

	osPlatform := platform.OSPlatform{}
	procList, err := osPlatform.GetProcesses()
	if err != nil {
		return
	}

	for _, p := range yamlConfig.Processes {
		if IsManagedOpmProcess(p) {
			continue // skip OPM
		}

		if p.GetStartMode() != fatima.StartModeByPS {
			continue
		}

		pid := service.GetPid(system.fatimaRuntime.GetEnv(), p)
		if pid > 0 {
			if ExistInProcessListWithPid(procList, pid) {
				if newPSStatus == monitor.PS_STATUS_SECONDARY {
					service.KillProgram(p.GetName(), pid)
				}
			} else if newPSStatus == monitor.PS_STATUS_PRIMARY {
				service.ExecuteProgram(system.fatimaRuntime.GetEnv(), p)
			}
		} else if newPSStatus == monitor.PS_STATUS_PRIMARY {
			service.ExecuteProgram(system.fatimaRuntime.GetEnv(), p)
		}
	}
}

func (system *SystemBase) Shutdown() {
	log.Info("SystemBase Shutdown()")
}

func (system *SystemBase) GetType() fatima.FatimaComponentType {
	return fatima.COMP_GENERAL
}

func (system *SystemBase) syncLogLevels(filePath string) {
	yamlConfig := builder.NewYamlFatimaPackageConfig(system.fatimaRuntime.GetEnv())

	logLevels := make(map[string]string)
	for _, p := range yamlConfig.Processes {
		logLevels[p.Name] = log.ConvertLogLevelToHexa(p.Loglevel)
	}

	b, err := json.Marshal(logLevels)
	if err != nil {
		log.Error("fail to marshal loglevels to json : %s", err.Error())
		return
	}

	err = os.WriteFile(filePath, b, 0644)
	if err != nil {
		log.Error("fail to write loglevel files : %s", err.Error())
	}

	log.Info("sync loglevels to file")
}

func maintainRevision(env fatima.FatimaEnv) {
	yamlConfig := builder.NewYamlFatimaPackageConfig(env)
	for _, p := range yamlConfig.Processes {
		revDir := filepath.Join(env.GetFolderGuide().GetFatimaHome(),
			builder.FatimaFolderApp,
			FOLDER_APP_REVISION,
			p.Name)
		removeOldRevision(revDir)
	}
}

type Dir struct {
	Name     string
	Revision int
}

type RevisionNumbers []Dir

func (a RevisionNumbers) Len() int           { return len(a) }
func (a RevisionNumbers) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a RevisionNumbers) Less(i, j int) bool { return a[j].Revision < a[i].Revision }

func removeOldRevision(path string) {
	dirs, err := filepath.Glob(path + "/*_R[0-9]*")
	if err != nil {
		return
	}

	if len(dirs) < 4 {
		return
	}

	revisions := make([]Dir, 0)
	for _, v := range dirs {
		idx := strings.LastIndex(v, "R")
		m, err := strconv.Atoi(v[idx+1:])
		if err != nil {
			continue
		}
		d := Dir{Name: v, Revision: m}
		revisions = append(revisions, d)
	}

	sort.Sort(RevisionNumbers(revisions))

	if len(revisions) < 4 {
		return
	}

	for i := 3; i < len(revisions); i++ {
		os.RemoveAll(revisions[i].Name)
	}
}

func startDeadProcesses(fatimaRuntime fatima.FatimaRuntime) {
	time.Sleep(time.Second * 3)

	//startDeadProcessesSerial(fatimaRuntime)
	service.StartDeadProcessesWithWeightGroup(fatimaRuntime)
}

func startDeadProcessesSerial(fatimaRuntime fatima.FatimaRuntime) {
	yamlConfig := builder.NewYamlFatimaPackageConfig(fatimaRuntime.GetEnv())

	platformImpl := platform.OSPlatform{}
	procList, err := platformImpl.GetProcesses()
	if err != nil {
		return
	}

	for _, p := range yamlConfig.Processes {
		if IsManagedOpmProcess(p) {
			continue // skip OPM
		}

		if !IsStartingTarget(fatimaRuntime, p.GetStartMode()) {
			log.Info("skip start process : %s", p.GetName())
			continue
		}
		pid := service.GetPid(fatimaRuntime.GetEnv(), p)
		if pid > 0 {
			if ExistInProcessListWithPid(procList, pid) {
				continue
			}
		}
		log.Trace("startDeadProcesses : %s", p.Name)
		service.ExecuteProgram(fatimaRuntime.GetEnv(), p)
	}
}
