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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/ipc"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"github.com/robfig/cron"
)

const (
	valueCronsDir = "crons"
)

func (service *DomainService) SummaryCronList() map[string]interface{} {
	report := make(map[string]interface{})

	log.Info("SummaryCronList")

	commandList := make([]domain.ProcessBatch, 0)
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())
	go removeUnusedCronsFiles(service.GetCronsDir(), yamlConfig)
	for _, p := range yamlConfig.Processes {
		if p.Gid == 1 {
			continue // 1 : OPM
		}
		file := filepath.Join(service.GetCronsDir(), buildCronJsonFilename(p.Name))
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		//cronJob := make(map[string]interface{})
		processBatch := domain.ProcessBatch{}
		err = json.Unmarshal(b, &processBatch)
		if err != nil {
			log.Warn("%s invalid json", p.Name)
			continue
		}
		//cronJob[p.ProcessName] = m
		commandList = append(commandList, processBatch)
	}

	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]interface{})
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()
	summary["batches"] = rebuildHourlyBatches(commandList)
	report["summary"] = summary

	return report
}

func rebuildHourlyBatches(list []domain.ProcessBatch) domain.BatchList {
	doc := domain.BatchList{}

	for _, item := range list {
		for _, job := range item.JobList {
			schedule, err := cron.Parse(job.Spec)
			if err != nil {
				log.Warn("%s invalid spec : %s", job.Name, err.Error())
				continue
			}

			specSchedule, ok := schedule.(*cron.SpecSchedule)
			if !ok {
				log.Warn("%s not SpecSchedule type", job.Name)
				continue
			}

			scheduled := dispatchTimes(job, specSchedule)
			for _, next := range scheduled {
				hourlyBatch := doc.FindHourlyBatch(next.Hour())
				if !hourlyBatch.Valid {
					hourlyBatch.Hour = next.Hour()
					hourlyBatch = hourlyBatch.ReflectProcessBatch(domain.NewProcessBatch(item.ProcessName, job))
					doc = doc.ReflectHourlyBatch(hourlyBatch)
					continue
				}
				processBatch := hourlyBatch.FindProcessBatch(item.ProcessName)
				if !processBatch.Valid {
					hourlyBatch = hourlyBatch.ReflectProcessBatch(domain.NewProcessBatch(item.ProcessName, job))
					doc = doc.ReflectHourlyBatch(hourlyBatch)
					continue
				}
				hourlyBatch = hourlyBatch.ReflectProcessBatch(processBatch.ReflectJob(job))
				doc = doc.ReflectHourlyBatch(hourlyBatch)
			}
		}
	}

	sort.Sort(doc.List)
	return doc
}

func dispatchTimes(job domain.BatchJob, schedule *cron.SpecSchedule) []time.Time {
	scheduled := make([]time.Time, 0)
	midnight := time.Date(time.Now().Year(),
		time.Now().Month(),
		time.Now().Day(),
		0,
		0,
		0,
		0,
		time.Local).Add(-time.Second)

	t := midnight
	today := false
	for {
		next := schedule.Next(t)
		if today && !isSameDay(t, next) {
			break
		}

		scheduled = append(scheduled, next)
		//log.Info("[%s::%s] next H=%s", job.Name, job.Spec, next)
		//log.Info("[%s::%s] diff(%d-%d) = %d", job.ProcessName, job.Spec, next.Unix(), t.Unix(), next.Unix()-t.Unix())
		if today && next.Unix()-t.Unix() <= 61 {
			//log.Info("[%s::%s] diff(%d-%d) = %d", job.Name, job.Spec, next.Unix(), t.Unix(), next.Unix()-t.Unix())
			t = time.Date(time.Now().Year(),
				time.Now().Month(),
				time.Now().Day(),
				next.Hour()+1,
				0,
				0,
				0,
				time.Local).Add(-time.Second)
			today = true
			continue
		}
		today = true
		t = next
	}

	return scheduled
}

func isSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

func (service *DomainService) ListCronCommand() map[string]interface{} {
	report := make(map[string]interface{})

	log.Info("ListCronCommand")

	commandList := make([]interface{}, 0)
	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())
	go removeUnusedCronsFiles(service.GetCronsDir(), yamlConfig)
	for _, p := range yamlConfig.Processes {
		if p.Gid == 1 {
			continue // 1 : OPM
		}
		file := filepath.Join(service.GetCronsDir(), buildCronJsonFilename(p.Name))
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		//cronJob := make(map[string]interface{})
		m := make(map[string]interface{})
		err = json.Unmarshal(b, &m)
		if err != nil {
			log.Warn("%s invalid json", p.Name)
			continue
		}
		//cronJob[p.ProcessName] = m
		commandList = append(commandList, m)
	}

	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]interface{})
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()
	summary["commands"] = commandList
	report["summary"] = summary
	return report
}

func (service *DomainService) GetCronsDir() string {
	return filepath.Join(service.fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder(), valueCronsDir)
}

func buildCronJsonFilename(prefix string) string {
	return prefix + ".json"
}

func removeUnusedCronsFiles(dir string, yamlConfig *builder.YamlFatimaPackageConfig) {
	set := make(map[string]struct{})
	for _, p := range yamlConfig.Processes {
		set[buildCronJsonFilename(p.Name)] = struct{}{}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		log.Warn("fail to read crons dir : %s", err.Error())
		return
	}

	for _, f := range files {
		_, ok := set[f.Name()]
		if !ok {
			log.Info("del : %s", f.Name())
			_ = os.Remove(filepath.Join(dir, f.Name()))
		}
	}
}

func (service *DomainService) RerunCronCommand(proc string, command string, sample string) map[string]interface{} {
	log.Info("rerun cron. proc=[%s], job=[%s], args=[%s]", proc, command, sample)

	message := fmt.Sprintf("successfully call rerun. proc=[%s], job=[%s], args=[%s]", proc, command, sample)
	err := requestRerunCronWithIPC(proc, command, sample)
	if err != nil {
		log.Debug("fail to call ipc. message=%s", err.Error())
		message = requestRerunCronWithFile(proc, command, sample, service.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome())
	}

	report := make(map[string]interface{})
	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]interface{})
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()
	summary["message"] = message
	report["summary"] = summary
	return report
}

func requestRerunCronWithIPC(proc string, jobName string, sample string) error {
	if !ipc.IsFatimaIPCAvailable(proc) {
		return fmt.Errorf("ipc not available")
	}

	ipcSession, err := ipc.NewFatimaIPCClientSession(proc)
	if err != nil {
		return fmt.Errorf("fail to create ipc session : %s", err.Error())
	}
	defer ipcSession.Disconnect()

	err = ipcSession.SendCommand(ipc.NewMessageCronExecute(jobName, sample))
	if err != nil {
		return fmt.Errorf("fail to send cron execute : %s", err.Error())
	}

	return nil
}

func requestRerunCronWithFile(proc string, command string, sample string, fatimaHomeDir string) string {
	file := filepath.Join(fatimaHomeDir,
		"data",
		proc,
		"cron.rerun")

	var message string
	content := command
	if len(sample) > 0 {
		content = command + " " + sample
		message = fmt.Sprintf("successfully call rerun. proc=%s, job=%s, args=%s", proc, command, sample)
	} else {
		message = fmt.Sprintf("successfully call rerun. proc=%s, job=%s", proc, command)
	}
	err := os.WriteFile(file, []byte(content), 0644)
	if err != nil {
		message = fmt.Sprintf("fail to call job %s for proc %s. err=%s", command, proc, err.Error())
	}
	return message
}
