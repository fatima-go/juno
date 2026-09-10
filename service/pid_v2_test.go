package service

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestNativePIDObservation(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin PID observation")
	}
	cmd := exec.Command("/bin/sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	for i := 0; i < 20; i++ {
		if !runningV2("sleep", cmd.Process.Pid) {
			t.Fatal("live PID reported dead", i)
		}
	}
	cmd.Process.Kill()
	cmd.Wait()
	if runningV2("sleep", cmd.Process.Pid) {
		t.Fatal("exited PID reported alive")
	}
}
