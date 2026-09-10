package service

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/juno/domain"
)

func TestStatusCollectorKeepsEveryProcess(t *testing.T) {
	items := make([]builder.ProcessItem, 257)
	for i := range items {
		items[i].Name = fmt.Sprintf("worker%d", i)
	}
	var active atomic.Int32
	rows := collectStatusV2(items, func(item builder.ProcessItem) *domain.ProcessInfo {
		if active.Add(1) > 8 {
			t.Error("unbounded status collection")
		}
		defer active.Add(-1)
		time.Sleep(time.Millisecond)
		return &domain.ProcessInfo{Name: item.Name, Status: domain.PROC_STATUS_ALIVE}
	})
	if len(rows) != len(items) {
		t.Fatal("missing rows")
	}
	for i, row := range rows {
		if row == nil || row.Name != items[i].Name || row.Index != i {
			t.Fatal("lost or reordered process", i, row)
		}
	}
}

func TestStatusIncludesRegistrationBeforeFirstMonitorSample(t *testing.T) {
	item := builder.ProcessItem{Name: "newworker"}
	row := registeredProcessV2(item, "SVC", domain.ProcessInfo{})
	if row.Name != "newworker" || row.Group != "SVC" || row.Status != "UNKNOWN" || row.Pid != "-" {
		t.Fatalf("new registration lost or unobserved state invented: %+v", row)
	}
	cached := domain.ProcessInfo{Name: "newworker", Group: "old-group", Status: domain.PROC_STATUS_ALIVE, Pid: "123", CpuUtil: "1.2"}
	row = registeredProcessV2(item, "SVC", cached)
	if row.Group != "SVC" || row.Status != domain.PROC_STATUS_ALIVE || row.Pid != "123" || row.CpuUtil != "1.2" || cached.Group != "old-group" {
		t.Fatalf("registry identity or cached measurement changed: row=%+v cache=%+v", row, cached)
	}
}
