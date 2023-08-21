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

package web

import (
	"github.com/fatima-go/juno/domain"
	"mime/multipart"
	"time"
)

type JunoWebServiceController interface {
	IsRemoteOperationAllowed(clientIp string) bool
	ValidateToken(token string, role domain.Role) error
	GetPackageReport(loc *time.Location) domain.PackageReport
	GetPackageReportForHealthCheck() map[string]string
	GetLogLevels() domain.LogLevels
	ChangeLogLevel(proc string, loglevel string) map[string]interface{}
	RegistProcess(proc string, groupId string) error
	UnregistProcess(proc string) error
	GetClipboard() string
	StopProcess(all bool, group string, proc string) map[string]interface{}
	StartProcess(all bool, group string, proc string) map[string]interface{}
	ListCronCommand() map[string]interface{}
	RerunCronCommand(proc string, command string, sample string) map[string]interface{}
	DeployPackage(mr *multipart.Reader) (string, error)
	ClearIcProcess(all bool, group string, proc string) map[string]interface{}
	DeploymentHistory(all bool, group string, proc string) map[string]interface{}
	GetProcessReport(loc *time.Location, proc string) domain.ProcessReport
}
