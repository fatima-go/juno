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
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-log"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	valueCronsDir = "crons"
)

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
		//cronJob[p.Name] = m
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

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Warn("fail to read crons dir : %s", err.Error())
		return
	}

	for _, f := range files {
		_, ok := set[f.Name()]
		if !ok {
			log.Info("del : %s", f.Name())
			os.Remove(filepath.Join(dir, f.Name()))
		}
	}
}

func (service *DomainService) RerunCronCommand(proc string, command string, sample string) map[string]interface{} {
	log.Info("rerun cron. proc=[%s], job=[%s], args=[%s]", proc, command, sample)
	file := filepath.Join(service.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(),
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

	report := make(map[string]interface{})
	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]interface{})
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()
	summary["message"] = message
	report["summary"] = summary
	return report
}
