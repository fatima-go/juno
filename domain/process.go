/*
 * Copyright (c) 2018 throosea.com.
 * All right reserved.
 *
 * This software is the confidential and proprietary information of throosea.com.
 * You shall not disclose such Confidential Information and
 * shall use it only in accordance with the terms of the license agreement
 * you entered into with throosea.com.
 */

package domain

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
