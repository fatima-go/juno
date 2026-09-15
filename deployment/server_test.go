package deployment

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fatima-go/fatima-opm/api"
)

func TestStartDisconnectDuplicateAndRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "juno")
	var executions atomic.Int32
	entered := make(chan struct{})
	executor := func(ctx context.Context, spec *api.OperationSpec, path string, emit Emit) (Result, error) {
		executions.Add(1)
		close(entered)
		if e := emit("goaway", "WAITING", "draining", 1, 31); e != nil {
			return Result{}, e
		}
		<-ctx.Done()
		return Result{}, ctx.Err()
	}
	auth := func(context.Context, *api.ValidateRequest) error { return nil }
	s, e := New(dir, "p:default", "darwin_arm64", auth, executor)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = New(dir, "p:default", "darwin_arm64", auth, executor); e == nil {
		t.Fatal("two executors acquired same storage")
	}
	spec := &api.OperationSpec{Id: "op", Process: "example", PackageId: "p:default"}
	var db database
	if e = s.Store.Update("operations", &db, func() error {
		db.Records = map[string]*record{"op": {Spec: spec, Operation: &api.Operation{Id: "op", State: "STAGED"}}}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if _, e = s.Start(ctx, &api.OperationQuery{Id: "op"}); e != nil {
		t.Fatal(e)
	}
	cancel()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("execution never started")
	}
	if _, e = s.Start(context.Background(), &api.OperationQuery{Id: "op"}); e != nil {
		t.Fatal(e)
	}
	if executions.Load() != 1 {
		t.Fatal("duplicate start executed twice")
	}
	s.Close()
	recovered, e := New(dir, "p:default", "darwin_arm64", auth, func(context.Context, *api.OperationSpec, string, Emit) (Result, error) {
		return Result{}, fmt.Errorf("must not auto-run")
	})
	if e != nil {
		t.Fatal(e)
	}
	defer recovered.Close()
	op, e := recovered.Get(context.Background(), &api.OperationQuery{Id: "op"})
	if e != nil || op.State != "INTERRUPTED" {
		t.Fatalf("restart lost interrupted state: %v %v", op, e)
	}
	if _, e = recovered.Start(context.Background(), &api.OperationQuery{Id: "op"}); e != nil {
		t.Fatal(e)
	}
	if executions.Load() != 1 {
		t.Fatal("restart replayed interrupted operation")
	}
}
