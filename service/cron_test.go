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
 * @date 25. 4. 23. 오후 3:05
 *
 */

package service

import (
	"fmt"
	"testing"

	"github.com/fatima-go/juno/domain"
	"github.com/stretchr/testify/assert"
)

func TestRebuildHourlyBatches(t *testing.T) {
	list := make([]domain.ProcessBatch, 0)
	batch := domain.ProcessBatch{}
	batch.ProcessName = "first"
	batch.JobList = make([]domain.BatchJob, 0)
	batch.JobList = appendJob(batch.JobList, "N1", "0 10 0 * * SUN,MON,WED,FRI")
	batch.JobList = appendJob(batch.JobList, "N2", "0 0 2 1 * *")
	batch.JobList = appendJob(batch.JobList, "N3", "@hourly")
	batch.JobList = appendJob(batch.JobList, "N4", "@annually")
	batch.JobList = appendJob(batch.JobList, "N5", "0 */10 * * * *")
	batch.JobList = appendJob(batch.JobList, "N6", "0 10 0 * * SUN,MON,WED,FRI")
	batch.JobList = appendJob(batch.JobList, "N7", "0 * * * * *")
	batch.JobList = appendJob(batch.JobList, "midnight", "@midnight")
	batch.JobList = appendJob(batch.JobList, "every1h30m", "@every 1h30m")
	list = append(list, batch)
	batch.ProcessName = "second"
	batch.JobList = make([]domain.BatchJob, 0)
	batch.JobList = appendJob(batch.JobList, "S1", "0 10 0 * * SUN,MON,WED,FRI")
	batch.JobList = appendJob(batch.JobList, "S2", "0 0 2 1 * *")
	list = append(list, batch)
	doc := rebuildHourlyBatches(list)
	for _, hourlyBatch := range doc.List {
		for _, p := range hourlyBatch.ProcessList {
			for _, j := range p.JobList {
				fmt.Printf("[%d] %s::%s::%s\n", hourlyBatch.Hour, p.ProcessName, j.Name, j.Spec)
			}
		}
	}
	assert.Nil(t, include(doc, 0, "first", "N1"))
	assert.NotNil(t, include(doc, 1, "first", "N1"))
	assert.Nil(t, include(doc, 2, "first", "N2"))
	assert.NotNil(t, include(doc, 1, "first", "N2"))
	for i := 0; i < 24; i++ {
		assert.Nil(t, include(doc, i, "first", "N3"))
	}
	assert.Nil(t, include(doc, 0, "first", "N4"))
	for i := 0; i < 24; i++ {
		assert.Nil(t, include(doc, i, "first", "N5"))
	}
	assert.Nil(t, include(doc, 0, "first", "N6"))
	for i := 0; i < 24; i++ {
		assert.Nil(t, include(doc, i, "first", "N7"))
	}
	assert.Nil(t, include(doc, 0, "first", "midnight"))
	assert.NotNil(t, include(doc, 0, "first", "every1h30m"))
	assert.NotNil(t, include(doc, 1, "first", "every1h30m"))
	assert.Nil(t, include(doc, 0, "second", "S1"))
	assert.NotNil(t, include(doc, 1, "second", "S1"))
	assert.Nil(t, include(doc, 2, "second", "S2"))
	assert.NotNil(t, include(doc, 1, "second", "S2"))
}

func appendJob(list []domain.BatchJob, name, spec string) []domain.BatchJob {
	if list == nil {
		list = make([]domain.BatchJob, 0)
	}
	list = append(list, domain.BatchJob{Valid: true, Spec: spec, Name: name})
	return list
}

func include(doc domain.BatchList, hour int, processName, jobName string) error {
	hourlyBatch := doc.FindHourlyBatch(hour)
	if !hourlyBatch.Valid {
		return fmt.Errorf("not found %d hourly batch for %s", hour, processName)
	}

	processBatch := hourlyBatch.FindProcessBatch(processName)
	if !processBatch.Valid {
		return fmt.Errorf("not found %d process batch for %s", hour, processName)
	}

	job := processBatch.FindJob(jobName)
	if !job.Valid {
		return fmt.Errorf("not found %s job for %s:%d", jobName, processName, hour)
	}

	return nil
}
