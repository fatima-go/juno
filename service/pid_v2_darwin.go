package service

import "golang.org/x/sys/unix"

// Juno's existing SIGCHLD reaper can collect a ps child before Cmd.Wait does.
// Query the kernel directly so v2 never mistakes that race for process exit.
func runningV2(_ string, pid int) bool {
	if pid < 2 {
		return false
	}
	p, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || p == nil {
		return false
	}
	const zombie = 5 // SZOMB from Darwin sys/proc.h.
	return p.Proc.P_pid == int32(pid) && p.Proc.P_stat != 0 && p.Proc.P_stat != zombie
}
