package service

func runningV2(name string, pid int) bool {
	return pid > 1 && inspector.CheckProcessRunningByPid(name, pid)
}
