package service

import (
	"context"
	"fmt"
	fatima "github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/ipc"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/artifact"
	"github.com/fatima-go/fatima-core/opm/operations"
	"github.com/fatima-go/juno/service/goaway"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func (s *DomainService) processCatalogV2(q *api.ProcessQuery) (result *api.ProcessCatalog, err error) {
	defer func() {
		if v := recover(); v != nil {
			result = nil
			err = fmt.Errorf("cannot read process status: %v", v)
		}
	}()
	loc := time.Local
	if q.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(q.Timezone)
		if err != nil {
			return nil, err
		}
	}
	r := s.processReportV2(loc)
	out := &api.ProcessCatalog{PackageId: r.Host + ":" + r.Summary.Name, Group: r.Group, Platform: r.Platform.Os + "_" + r.Platform.Architecture, HaStatus: int32(r.HAStatus), PsStatus: int32(r.PSStatus)}
	cfg := builder.NewYamlFatimaPackageConfig(s.fatimaRuntime.GetEnv())
	out.ObservedAt = time.Now().Unix()
	for _, p := range r.ProcInfo {
		item := cfg.GetProcByName(p.Name)
		isOPM := item != nil && item.GetGid() == 1
		out.Processes = append(out.Processes, &api.ProcessEntry{Name: p.Name, Pid: p.Pid, State: p.Status, Group: p.Group, Cpu: p.CpuUtil, Memory: p.Memory, Fd: p.FDCount, Threads: p.Thread, StartedAt: p.StartTime, Ic: p.ICount, Index: int32(p.Index), Opm: isOPM})
	}
	return out, nil
}

// Only the new operating/deployment paths participate; legacy contracts remain unchanged.
func (s *DomainService) lockProcessesV2(names []string) (func(), error) {
	root := filepath.Join(s.fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder(), "operation-locks")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	ordered := append([]string(nil), names...)
	sort.Strings(ordered)
	var files []*os.File
	unlock := func() {
		for _, f := range files {
			syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			f.Close()
		}
	}
	for _, name := range ordered {
		name = strings.ToLower(name)
		if !artifact.SafeName.MatchString(name) {
			unlock()
			return nil, fmt.Errorf("invalid process name")
		}
		f, err := os.OpenFile(filepath.Join(root, name+".lock"), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			unlock()
			return nil, err
		}
		if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			f.Close()
			unlock()
			return nil, fmt.Errorf("another operation is controlling %s", name)
		}
		files = append(files, f)
	}
	return unlock, nil
}
func (s *DomainService) selectedProcessesV2(names []string) ([]fatima.FatimaPkgProc, error) {
	cfg := builder.NewYamlFatimaPackageConfig(s.fatimaRuntime.GetEnv())
	var out []fatima.FatimaPkgProc
	for _, name := range names {
		p := cfg.GetProcByName(name)
		if p == nil {
			return nil, fmt.Errorf("process %s is not registered", name)
		}
		out = append(out, p)
	}
	return out, nil
}
func (s *DomainService) stopProcessesV2(ctx context.Context, names []string, emit operations.Emit) error {
	procs, err := s.selectedProcessesV2(names)
	if err != nil {
		return err
	}
	for _, p := range procs {
		if strings.EqualFold(p.GetName(), "jupiter") || strings.EqualFold(p.GetName(), "juno") {
			return fmt.Errorf("%s cannot be stopped by remote process control", p.GetName())
		}
	}
	unlock, err := s.lockProcessesV2(names)
	if err != nil {
		return err
	}
	defer unlock()
	sort.SliceStable(procs, func(i, j int) bool { return procs[i].GetWeight() < procs[j].GetWeight() })
	for i, p := range procs {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = emit(p.GetName()+"/stop", "RUNNING", "Stopping selected process", int64(i), int64(len(procs))); err != nil {
			return err
		}
		if err = s.stopObservedV2(ctx, p, emit); err != nil {
			return fmt.Errorf("%s: %w", p.GetName(), err)
		}
		if err = emit(p.GetName()+"/stop", "SUCCEEDED", "Process stopped", int64(i+1), int64(len(procs))); err != nil {
			return err
		}
	}
	return nil
}
func (s *DomainService) stopObservedV2(ctx context.Context, p fatima.FatimaPkgProc, emit operations.Emit) error {
	env := s.fatimaRuntime.GetEnv()
	name := p.GetName()
	pid := GetPid(env, p)
	GetProcessMonitor().ProcessStop(name)
	if pid < 2 || !runningV2(name, pid) {
		return emit(name+"/shutdown", "SUCCEEDED", "Already stopped", 0, 0)
	}
	report := func(stage, state, msg string, current, total int64) error {
		return emit(name+"/"+stage, state, msg, current, total)
	}
	if ipc.IsFatimaIPCAvailable(name) {
		if err := goaway.ExecuteObserved(ctx, name, func(message string, elapsed int64) error { return report("goaway", "WAITING", message, elapsed, 31) }); err != nil {
			return err
		}
	} else if isFatimaOrientProcess(env, p) {
		if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
			return err
		}
		for i := int64(0); i < 31; i++ {
			if err := report("goaway", "WAITING", "Legacy SIGUSR1; completion acknowledgement is unavailable", i, 31); err != nil {
				return err
			}
			if err := waitControl(ctx, time.Second); err != nil {
				return err
			}
		}
	}
	script := filepath.Join(env.GetFolderGuide().GetFatimaHome(), builder.FatimaFolderApp, name, shellGoaway)
	if _, err := os.Stat(script); err == nil {
		if err = report("goaway", "RUNNING", "Running goaway.sh", 0, 0); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, "/bin/sh", script)
		cmd.Dir = filepath.Dir(script)
		if err = cmd.Run(); err != nil {
			return err
		}
	}
	if err := report("shutdown", "RUNNING", fmt.Sprintf("Sending SIGTERM to PID %d", pid), 0, 120); err != nil {
		return err
	}
	if err := KillProgram(name, pid); err != nil && err != syscall.ESRCH {
		return err
	}
	for i := int64(0); i < 120; i++ {
		if !runningV2(name, pid) {
			return report("shutdown", "SUCCEEDED", fmt.Sprintf("PID %d exited", pid), i, 120)
		}
		if err := report("shutdown", "WAITING", fmt.Sprintf("Waiting for PID %d to exit", pid), i, 120); err != nil {
			return err
		}
		if err := waitControl(ctx, time.Second); err != nil {
			return err
		}
	}
	return fmt.Errorf("process did not stop within 120 seconds")
}
func waitControl(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
