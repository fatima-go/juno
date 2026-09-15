package control

import (
	"context"
	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/artifact"
	"github.com/fatima-go/fatima-opm/operations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

func (a *processAPI) WatchCatalog(q *api.ProcessQuery, stream grpc.ServerStreamingServer[api.ProcessCatalog]) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastAuth := time.Time{}
	for {
		if time.Since(lastAuth) >= time.Minute {
			if err := a.s.Authorize(stream.Context(), "MONITOR"); err != nil {
				return err
			}
			lastAuth = time.Now()
		}
		r, err := a.s.Processes(q)
		if err != nil {
			return err
		}
		if err = stream.Send(r); err != nil {
			return err
		}
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-a.s.Manager.Done():
			return status.Error(codes.Unavailable, "server is shutting down")
		case <-ticker.C:
		}
	}
}

type processAPI struct {
	api.UnimplementedProcessControlServer
	s *Server
}

func (a *processAPI) Catalog(ctx context.Context, q *api.ProcessQuery) (*api.ProcessCatalog, error) {
	if err := a.s.Authorize(ctx, "MONITOR"); err != nil {
		return nil, err
	}
	return a.s.Processes(q)
}
func (a *processAPI) Stop(ctx context.Context, q *api.ProcessRequest) (*api.ControlOperation, error) {
	if err := a.s.Authorize(ctx, "OPERATOR"); err != nil {
		return nil, err
	}
	if err := a.s.CheckRemoteOperation(ctx); err != nil {
		return nil, err
	}
	if err := validateProcesses(q); err != nil {
		return nil, err
	}
	return a.s.Manager.Start(q.RequestId, "rostop", q, func(ctx context.Context, emit operations.Emit) (string, string, error) {
		err := a.s.StopProcesses(ctx, q.Processes, emit)
		return "SUCCEEDED", "All selected processes are stopped", err
	})
}
func validateProcesses(q *api.ProcessRequest) error {
	if len(q.Processes) == 0 || len(q.Processes) > 1000 {
		return status.Error(codes.InvalidArgument, "select between 1 and 1000 processes")
	}
	seen := map[string]bool{}
	for _, name := range q.Processes {
		if !artifact.SafeName.MatchString(name) || seen[name] {
			return status.Error(codes.InvalidArgument, "invalid or duplicate process name")
		}
		seen[name] = true
	}
	return nil
}
func (a *processAPI) Start(ctx context.Context, q *api.ProcessRequest) (*api.ControlOperation, error) {
	if err := a.s.Authorize(ctx, "OPERATOR"); err != nil {
		return nil, err
	}
	if err := a.s.CheckRemoteOperation(ctx); err != nil {
		return nil, err
	}
	if err := validateProcesses(q); err != nil {
		return nil, err
	}
	return a.s.Manager.Start(q.RequestId, "rostart", q, func(ctx context.Context, emit operations.Emit) (string, string, error) {
		err := a.s.StartProcesses(ctx, q.Processes, emit)
		return "SUCCEEDED", "All selected processes are running. Application readiness is not checked.", err
	})
}
func (a *processAPI) Get(ctx context.Context, q *api.OperationQuery) (*api.ControlOperation, error) {
	return a.s.Get(ctx, q)
}
func (a *processAPI) Watch(q *api.OperationQuery, stream grpc.ServerStreamingServer[api.ControlOperation]) error {
	return a.s.Watch(q, stream)
}
