package service

import (
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/juno/domain"
)

// The legacy Darwin collector appends to one slice from several goroutines.
// Keep that HTTP path unchanged; v2 uses fixed slots and bounded concurrency.
func collectStatusV2(items []builder.ProcessItem, inspect func(builder.ProcessItem) *domain.ProcessInfo) []*domain.ProcessInfo {
	rows := make([]*domain.ProcessInfo, len(items))
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for index, item := range items {
		slots <- struct{}{}
		wg.Add(1)
		go func(index int, item builder.ProcessItem) {
			defer wg.Done()
			defer func() { <-slots }()
			row := inspect(item)
			row.Index = index
			rows[index] = row
		}(index, item)
	}
	wg.Wait()
	return rows
}

func (s *DomainService) processReportV2(loc *time.Location) domain.PackageReport {
	env := s.fatimaRuntime.GetEnv()
	cfg := builder.NewYamlFatimaPackageConfig(env)
	cfg.OrderByGroup()
	list := newProcessList(toGroupMap(cfg.Groups))
	rows := collectStatusV2(cfg.Processes, func(item builder.ProcessItem) *domain.ProcessInfo {
		if runtime.GOOS != "darwin" {
			return registeredProcessV2(item, list.group[item.Gid], GetProcessMonitor().GetProcess(item.Name, loc))
		}
		row := domain.NewProcessInfo()
		row.Name, row.Group = item.Name, list.group[item.Gid]
		pid := GetPid(env, item)
		if runningV2(item.Name, pid) {
			row.Status = domain.PROC_STATUS_ALIVE
			row.Pid = strconv.Itoa(pid)
		}
		return row
	})
	if runtime.GOOS == "darwin" {
		inspector.MeasureProcessStatus(rows, loc)
	}
	pack := s.fatimaRuntime.GetPackaging()
	report := domain.NewPackageReport()
	report.Group, report.Host = pack.GetGroup(), pack.GetHost()
	report.HAStatus = int(s.fatimaRuntime.GetSystemStatus().GetHAStatus())
	report.PSStatus = int(s.fatimaRuntime.GetSystemStatus().GetPSStatus())
	report.Summary = domain.PackageSummary{Name: pack.GetName(), Total: len(rows)}
	for _, row := range rows {
		report.ProcInfo = append(report.ProcInfo, *row)
		if row.Status == domain.PROC_STATUS_ALIVE {
			report.Summary.Alive++
		} else {
			report.Summary.Dead++
		}
	}
	return report
}

// The registry is authoritative for identity and membership. A newly registered
// process may not have a monitor sample yet; it must still be selectable for
// start/stop without inventing a DEAD or ALIVE observation.
func registeredProcessV2(item builder.ProcessItem, group string, cached domain.ProcessInfo) *domain.ProcessInfo {
	if cached.Name == "" {
		cached = *domain.NewProcessInfo()
		cached.Status = "UNKNOWN"
	}
	cached.Name, cached.Group = item.Name, group
	return &cached
}
