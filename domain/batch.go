/*
 * Copyright 2025 github.com/fatima-go
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
 * @author dave_01
 * @date 25. 4. 23. 오후 1:21
 *
 */

package domain

type ByHourAsc []HourlyBatch

func (a ByHourAsc) Len() int           { return len(a) }
func (a ByHourAsc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByHourAsc) Less(i, j int) bool { return a[i].Hour < a[j].Hour }

type BatchList struct {
	List ByHourAsc `json:"batches"`
}

func (b BatchList) ReflectHourlyBatch(batch HourlyBatch) BatchList {
	list := make([]HourlyBatch, 0)
	switched := false
	for _, h := range b.List {
		if h.Hour != batch.Hour {
			list = append(list, h)
		} else {
			list = append(list, batch)
			switched = true
		}
	}
	if !switched {
		list = append(list, batch)
	}
	b.List = list
	return b
}

func (b BatchList) FindHourlyBatch(hour int) HourlyBatch {
	for _, batch := range b.List {
		if batch.Hour == hour {
			return batch
		}
	}
	return HourlyBatch{Valid: false}
}

type HourlyBatch struct {
	Valid       bool           `json:"-"`
	Hour        int            `json:"hour"`
	ProcessList []ProcessBatch `json:"processes"`
}

func (h HourlyBatch) ReflectProcessBatch(batch ProcessBatch) HourlyBatch {
	if h.ProcessList == nil {
		h.ProcessList = make([]ProcessBatch, 0)
		h.ProcessList = append(h.ProcessList, batch)
		h.Valid = true
	}

	list := make([]ProcessBatch, 0)
	switched := false
	for _, p := range h.ProcessList {
		if p.ProcessName == batch.ProcessName {
			list = append(list, batch)
			switched = true
			continue
		}
		list = append(list, p)
	}
	if !switched {
		list = append(list, batch)
	}
	h.ProcessList = list
	return h
}

func (h HourlyBatch) FindProcessBatch(processName string) ProcessBatch {
	for _, batch := range h.ProcessList {
		if batch.ProcessName == processName {
			return batch
		}
	}
	return ProcessBatch{Valid: false}
}

func NewProcessBatch(processName string, job BatchJob) ProcessBatch {
	processBatch := ProcessBatch{Valid: true}
	processBatch.ProcessName = processName
	processBatch.JobList = make([]BatchJob, 0)
	processBatch.JobList = append(processBatch.JobList, job)
	return processBatch
}

type ProcessBatch struct {
	Valid       bool       `json:"-"`
	ProcessName string     `json:"process"`
	JobList     []BatchJob `json:"jobs"`
}

func (p ProcessBatch) FindJob(jobName string) BatchJob {
	for _, job := range p.JobList {
		if job.Name == jobName {
			return job
		}
	}
	return BatchJob{Valid: false}
}

func (p ProcessBatch) ReflectJob(job BatchJob) ProcessBatch {
	if p.JobList == nil {
		p.JobList = make([]BatchJob, 0)
	}

	for _, item := range p.JobList {
		if item.Name == job.Name {
			return p // exist
		}
	}

	p.Valid = true
	job.Valid = true
	p.JobList = append(p.JobList, job)
	return p
}

type BatchJob struct {
	Valid       bool   `json:"-"`
	Spec        string `json:"spec"`
	Name        string `json:"name"`
	Description string `json:"desc"`
}

func (b BatchJob) GetHour() int {
	return 0
}

/*
{
  "process" : "searchd",
  "jobs" : [
    {
      "spec" : "0 0 3 * * *",
      "sample" : "수행이 필요할 경우 manual",
      "name" : "support.dictionary",
      "desc" : "사전 파일 적용 (스케쥴링 미수행)"
    },
    {
      "spec" : "0 0 4 * * *",
      "sample" : "전체 색인일 경우 manual, 특정 시점 기준 변경된 대상을 보정할 경우 yyyyMMddHHmmss",
      "name" : "fullindex.track",
      "desc" : "곡 풀색인 (스케쥴링 미수행)"
    },
    {
      "spec" : "0 0 4 * * *",
      "sample" : "전체 색인일 경우 manual, 특정 시점 기준 변경된 대상을 보정할 경우 yyyyMMddHHmmss",
      "name" : "fullindex.album",
      "desc" : "앨범 풀색인 (스케쥴링 미수행)"
    }
  ]
}
*/
