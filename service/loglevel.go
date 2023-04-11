//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//
// @project juno
// @author DeockJin Chung (jin.freestyle@gmail.com)
// @date 2017. 3. 12. PM 11:14
//

package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"throosea.com/fatima/builder"
	"throosea.com/juno/domain"
	"throosea.com/log"
)

func (service *DomainService) GetLogLevels() domain.LogLevels {
	loglevel := domain.LogLevels{}
	loglevel.Group = service.fatimaRuntime.GetPackaging().GetGroup()
	loglevel.Host = service.fatimaRuntime.GetPackaging().GetHost()

	summary := domain.LogLevelSummary{}
	summary.LogLevel = make([]domain.LogLevelInfo, 0)
	summary.Name = service.fatimaRuntime.GetPackaging().GetName()

	prev := service.readLogLevels()
	for k, v := range prev {
		s, _ := log.ConvertHexaToLogLevel(v)
		summary.LogLevel = append(summary.LogLevel, domain.LogLevelInfo{Level: s.String(), Name: k})
	}

	loglevel.Summary = summary
	return loglevel
}

func (service *DomainService) readLogLevels() map[string]string {
	info := make(map[string]string)

	filePath := filepath.Join(
		service.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(),
		domain.FOLDER_PACKAGE,
		domain.FOLDER_CFM,
		domain.FILE_LOG_LEVEL)

	b, err := os.ReadFile(filePath)
	if err != nil {
		log.Warn("fail to read loglevel file : %s", err.Error())
		return info
	}

	err = json.Unmarshal(b, &info)
	if err != nil {
		log.Warn("fail to change loglevel : %s", err.Error())
		return info
	}

	return info
}

func (service *DomainService) writeLogLevels(loglevels map[string]string) {
	filePath := filepath.Join(
		service.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(),
		domain.FOLDER_PACKAGE,
		domain.FOLDER_CFM,
		domain.FILE_LOG_LEVEL)

	b, err := json.Marshal(loglevels)
	if err != nil {
		log.Error("fail to marshal loglevels to json : %s", err.Error())
		return
	}

	err = os.WriteFile(filePath, b, 0644)
	if err != nil {
		log.Error("fail to write loglevel files : %s", err.Error())
	}
}

func (service *DomainService) ChangeLogLevel(proc string, loglevel string) map[string]interface{} {
	report := make(map[string]interface{})

	report["package_group"] = service.fatimaRuntime.GetPackaging().GetGroup()
	report["package_host"] = service.fatimaRuntime.GetPackaging().GetHost()
	summary := make(map[string]string)
	summary["package_name"] = service.fatimaRuntime.GetPackaging().GetName()

	yamlConfig := builder.NewYamlFatimaPackageConfig(service.fatimaRuntime.GetEnv())
	found := false
	compare := strings.ToLower(proc)
	for _, p := range yamlConfig.Processes {
		if compare == strings.ToLower(p.Name) {
			found = true
			break
		}
	}
	if !found {
		summary["message"] = fmt.Sprintf("not found process : %s", proc)
	} else {
		level := log.ConvertLogLevelToHexa(loglevel)
		prev := service.readLogLevels()
		prev[proc] = level
		service.writeLogLevels(prev)
		summary["message"] = "1 process reflected to " + strings.ToUpper(loglevel)
	}

	report["summary"] = summary
	return report
}
