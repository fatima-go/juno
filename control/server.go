// Package control implements additive operating APIs without invoking legacy HTTP handlers.
package control

import (
	"context"
	"fmt"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/operations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	api.UnimplementedCronControlServer
	Manager         *operations.Manager
	PackageID       string
	Authorize       func(context.Context, string) error
	ListJobs        func() (*api.CronCatalog, error)
	RunJob          func(context.Context, *api.CronRequest) (string, error)
	Processes       func(*api.ProcessQuery) (*api.ProcessCatalog, error)
	StopProcesses   func(context.Context, []string, operations.Emit) error
	StartProcesses  func(context.Context, []string, operations.Emit) error
	RegistryCatalog func(*api.RegistryQuery) (*api.RegistryCatalog, error)
	RegistryPreview func(*api.RegistryRequest) (*api.RegistryPlan, error)
	RegistryApply   func(context.Context, *api.RegistryRequest, operations.Emit) error
	LogLevels       func() (*api.LogLevelCatalog, error)
	SetLogLevel     func(process, level string) (*api.LogLevelEntry, error)
}

func New(root, packageID string, auth func(context.Context, string) error) (*Server, error) {
	m, err := operations.New(root, packageID)
	if err != nil {
		return nil, err
	}
	return &Server{Manager: m, PackageID: packageID, Authorize: auth}, nil
}
func (s *Server) Register(g *grpc.Server) {
	api.RegisterCronControlServer(g, s)
	api.RegisterProcessControlServer(g, &processAPI{s: s})
	api.RegisterProcessRegistryServer(g, &registryAPI{s: s})
	api.RegisterLogLevelControlServer(g, &logLevelAPI{s: s})
}
func (s *Server) Features() []string {
	return []string{"rocron", "rostop", "rostart", "rodis", "roproc", "rolog"}
}
func (s *Server) Close() { s.Manager.Close() }
func (s *Server) List(ctx context.Context, _ *api.Empty) (*api.CronCatalog, error) {
	if err := s.Authorize(ctx, "MONITOR"); err != nil {
		return nil, err
	}
	return s.ListJobs()
}
func (s *Server) Rerun(ctx context.Context, q *api.CronRequest) (*api.ControlOperation, error) {
	if err := s.Authorize(ctx, "OPERATOR"); err != nil {
		return nil, err
	}
	if q.Process == "" || q.Job == "" || len(q.Arguments) > 8192 {
		return nil, status.Error(codes.InvalidArgument, "process and job required; arguments limited to 8192 bytes")
	}
	return s.Manager.Start(q.RequestId, "rocron", q, func(ctx context.Context, emit operations.Emit) (string, string, error) {
		catalog, err := s.ListJobs()
		if err != nil {
			return "", "", err
		}
		found := false
		for _, j := range catalog.Jobs {
			if j.Process == q.Process && j.Name == q.Job {
				found = true
				break
			}
		}
		if !found {
			return "", "", fmt.Errorf("cron job is not registered for this process")
		}
		message, err := s.RunJob(ctx, q)
		return "REQUESTED", message, err
	})
}
func (s *Server) Get(ctx context.Context, q *api.OperationQuery) (*api.ControlOperation, error) {
	if err := s.Authorize(ctx, "MONITOR"); err != nil {
		return nil, err
	}
	return s.Manager.Get(q.Id)
}
func (s *Server) Watch(q *api.OperationQuery, stream grpc.ServerStreamingServer[api.ControlOperation]) error {
	if err := s.Authorize(stream.Context(), "MONITOR"); err != nil {
		return err
	}
	return s.Manager.Watch(stream.Context(), q.Id, stream.Send)
}
