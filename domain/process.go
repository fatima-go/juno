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

package domain

import "github.com/fatima-go/fatima-core"

type ProcessReport struct {
	Package     BriefPackage `json:"package"`
	Process     Process      `json:"process"`
	Description string       `json:"description"`
	Deployment  Deployment   `json:"deployment"`
	Monitoring  Monitoring   `json:"monitoring"`
	BatchJobs   BatchJobs    `json:"batch_jobs"`
}

type BriefPackage struct {
	Group string `json:"group"`
	Host  string `json:"host"`
	Name  string `json:"name"`
}

type Process struct {
	Name      string `json:"name"`
	Pid       string `json:"pid"`
	Status    string `json:"status"`
	StartTime string `json:"start_time"`
}

type Deployment struct {
	BuildTime        string `json:"build_time"`
	BuildUser        string `json:"build_user"`
	GitBranch        string `json:"git_branch"`
	GitCommit        string `json:"git_commit"`
	GitCommitMessage string `json:"git_commit_message"`
}

type Monitoring struct {
	LogTail string `json:"log_tail"`
}

type BatchJobs struct {
	JobList []BatchItem `json:"job_list"`
}

type BatchItem struct {
	Name   string `json:"name"`
	Spec   string `json:"spec"`
	Desc   string `json:"desc"`
	Sample string `json:"sample"`
}

type CronJob struct {
	Process string     `json:"process"`
	Jobs    []CronItem `json:"jobs"`
}

type CronItem struct {
	Name   string `json:"name"`
	Desc   string `json:"desc,omitempty"`
	Spec   string `json:"spec,omitempty"`
	Sample string `json:"sample,omitempty"`
}

func IsStartingTarget(fatimaRuntime fatima.FatimaRuntime, startMode fatima.ProcessStartMode) bool {
	switch startMode {
	case fatima.StartModeByJuno:
		return true
	case fatima.StartModeAlone:
		return false
	case fatima.StartModeByHA:
		return fatimaRuntime.GetSystemStatus().IsActive()
	case fatima.StartModeByPS:
		return fatimaRuntime.GetSystemStatus().IsPrimary()
	}

	return false
}

func ExistInProcessListWithPid(list []fatima.Process, pid int) bool {
	for _, p := range list {
		if p.Pid() == pid {
			return true
		}
	}
	return false
}
