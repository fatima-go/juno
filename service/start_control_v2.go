package service

import (
	"context"
	"fmt"
	"github.com/fatima-go/fatima-opm/operations"
	"sort"
	"time"
)

func (s *DomainService) startProcessesV2(ctx context.Context, names []string, emit operations.Emit) error {
	procs, err := s.selectedProcessesV2(names)
	if err != nil {
		return err
	}
	unlock, err := s.lockProcessesV2(names)
	if err != nil {
		return err
	}
	defer unlock()
	sort.SliceStable(procs, func(i, j int) bool { return procs[i].GetWeight() > procs[j].GetWeight() })
	env := s.fatimaRuntime.GetEnv()
	for i, p := range procs {
		if err = ctx.Err(); err != nil {
			return err
		}
		name := p.GetName()
		pid := GetPid(env, p)
		if pid > 1 && runningV2(name, pid) {
			if err = emit(name+"/start", "SUCCEEDED", fmt.Sprintf("Already running: PID %d", pid), int64(i+1), int64(len(procs))); err != nil {
				return err
			}
			continue
		}
		if err = emit(name+"/start", "RUNNING", "Starting process", int64(i), int64(len(procs))); err != nil {
			return err
		}
		pid, err = ExecuteProgram(env, p)
		GetProcessMonitor().ProcessStop(name)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if pid <= 1 {
			return fmt.Errorf("%s start returned no valid PID", name)
		}
		seconds := max(3, p.GetStartSec())
		for elapsed := 0; elapsed < seconds; elapsed++ {
			if err = emit(name+"/start", "WAITING", fmt.Sprintf("Checking PID %d remains alive; application readiness is not checked", pid), int64(elapsed), int64(seconds)); err != nil {
				return err
			}
			if err = waitControl(ctx, time.Second); err != nil {
				return err
			}
			if !runningV2(name, pid) {
				return fmt.Errorf("%s PID %d exited during startup", name, pid)
			}
		}
		GetProcessMonitor().ProcessStart(name)
		if err = emit(name+"/start", "SUCCEEDED", fmt.Sprintf("PID %d is running; application readiness is not checked", pid), int64(i+1), int64(len(procs))); err != nil {
			return err
		}
	}
	return nil
}
