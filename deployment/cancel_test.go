package deployment

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func cancelSpec() *api.OperationSpec {
	return &api.OperationSpec{Id: "op", ArtifactId: "far", Process: "worker", PackageId: "p:default", Sha256: strings.Repeat("a", 64), Size: 1}
}
func TestCancelBeforeStageSurvivesRestartAndLateRequests(t *testing.T) {
	root := t.TempDir()
	auth := func(context.Context, *api.ValidateRequest) error { return nil }
	execute := func(context.Context, *api.OperationSpec, string, Emit) (Result, error) {
		t.Error("cancelled operation executed")
		return Result{}, nil
	}
	s, err := New(root, "p:default", "linux_amd64", auth, execute)
	if err != nil {
		t.Fatal(err)
	}
	if op, err := s.Cancel(context.Background(), cancelSpec()); err != nil || op.State != "CANCELLED" {
		t.Fatal(op, err)
	}
	s.Close()
	s, err = New(root, "p:default", "linux_amd64", auth, execute)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	g := grpc.NewServer()
	s.Register(g)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve(l)
	defer g.Stop()
	c, err := transport.Dial(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	client := api.NewPackageDeploymentClient(c)
	stream, err := client.Stage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = stream.Send(&api.StageChunk{Spec: cancelSpec()}); err != nil {
		t.Fatal(err)
	}
	if op, err := stream.CloseAndRecv(); err != nil || op.State != "CANCELLED" {
		t.Fatal(op, err)
	}
	if op, err := client.Start(context.Background(), &api.OperationQuery{Id: "op"}); err != nil || op.State != "CANCELLED" {
		t.Fatal(op, err)
	}
	if op, err := client.Cancel(context.Background(), cancelSpec()); err != nil || op.State != "CANCELLED" {
		t.Fatal(op, err)
	}
	s.Authorize = func(context.Context, *api.ValidateRequest) error {
		return status.Error(codes.PermissionDenied, "wrong scope")
	}
	if _, err := client.Cancel(context.Background(), cancelSpec()); status.Code(err) != codes.PermissionDenied {
		t.Fatal(err)
	}
}
func TestStartCancelRaceHasOneDurableWinner(t *testing.T) {
	for i := 0; i < 25; i++ {
		var count atomic.Int32
		s, err := New(t.TempDir(), "p:default", "linux_amd64", func(context.Context, *api.ValidateRequest) error { return nil }, func(ctx context.Context, _ *api.OperationSpec, _ string, _ Emit) (Result, error) {
			count.Add(1)
			<-ctx.Done()
			return Result{}, ctx.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
		spec := cancelSpec()
		var db database
		if err = s.Store.Update("operations", &db, func() error {
			db.Records = map[string]*record{"op": {Spec: spec, Operation: &api.Operation{Id: "op", State: "STAGED"}}}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, e := s.Start(context.Background(), &api.OperationQuery{Id: "op"}); e != nil {
				t.Error(e)
			}
		}()
		go func() {
			defer wg.Done()
			if _, e := s.Cancel(context.Background(), spec); e != nil {
				t.Error(e)
			}
		}()
		wg.Wait()
		s.Close()
		r, e := s.get("op")
		if e != nil {
			t.Fatal(e)
		}
		if r.Operation.State == "CANCELLED" && count.Load() != 0 {
			t.Fatal("executed after cancel won")
		}
		if r.Operation.State != "CANCELLED" && count.Load() != 1 {
			t.Fatal("start winner did not execute exactly once")
		}
	}
}
