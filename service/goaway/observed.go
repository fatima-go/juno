package goaway

import (
	"context"
	"fmt"
	. "github.com/fatima-go/fatima-core/v2/ipc"
	"time"
)

// ExecuteObserved is deliberately separate from the legacy timeout behavior.
// A failed goaway acknowledgement is not reported as successful completion.
func ExecuteObserved(ctx context.Context, proc string, progress func(string, int64) error) error {
	started := time.Now()
	client, e := NewFatimaIPCClientSession(proc)
	if e != nil {
		return e
	}
	defer client.Disconnect()
	if e = client.SendCommand(NewMessageGoaway()); e != nil {
		return e
	}
	if e = progress("waiting for goaway start", 0); e != nil {
		return e
	}
	if e = receiveObserved(ctx, client, CommandGoawayStart, time.Second, progress); e != nil {
		return e
	}
	if e = progress("goaway started; waiting for completion", 0); e != nil {
		return e
	}
	if e = receiveObserved(ctx, client, CommandGoawayDone, goawayTimeoutDuration, progress); e != nil {
		return e
	}
	return progress("goaway completed", int64(time.Since(started)/time.Second))
}
func receiveObserved(ctx context.Context, client FatimaIPCClientSession, command string, timeout time.Duration, progress func(string, int64) error) error {
	result := make(chan error, 1)
	go func() {
		m, e := client.ReadCommand()
		if e == nil && !m.Is(command) {
			e = fmt.Errorf("unexpected goaway response: %s", m)
		}
		result <- e
	}()
	started := time.Now()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case e := <-result:
			return e
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return fmt.Errorf("timeout waiting for %s", command)
		case <-ticker.C:
			if e := progress("waiting for "+command, int64(time.Since(started)/time.Second)); e != nil {
				return e
			}
		}
	}
}
