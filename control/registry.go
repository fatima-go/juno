package control

import (
	"context"
	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/operations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type registryAPI struct {
	api.UnimplementedProcessRegistryServer
	s *Server
}

func (a *registryAPI) authorize(ctx context.Context, id, role string) error {
	if err := a.s.Authorize(ctx, role); err != nil {
		return err
	}
	if id != a.s.PackageID {
		return status.Error(codes.FailedPrecondition, "Juno package identity changed")
	}
	return nil
}
func (a *registryAPI) Catalog(ctx context.Context, q *api.RegistryQuery) (*api.RegistryCatalog, error) {
	if err := a.authorize(ctx, q.PackageId, "MONITOR"); err != nil {
		return nil, err
	}
	return a.s.RegistryCatalog(q)
}
func (a *registryAPI) Preview(ctx context.Context, q *api.RegistryRequest) (*api.RegistryPlan, error) {
	if err := a.authorize(ctx, q.PackageId, "MONITOR"); err != nil {
		return nil, err
	}
	return a.s.RegistryPreview(q)
}
func (a *registryAPI) Apply(ctx context.Context, q *api.RegistryRequest) (*api.ControlOperation, error) {
	if err := a.authorize(ctx, q.PackageId, "OPERATOR"); err != nil {
		return nil, err
	}
	if err := a.s.CheckRemoteOperation(ctx); err != nil {
		return nil, err
	}
	if q.ExpectedRevision == "" {
		return nil, status.Error(codes.InvalidArgument, "preview revision is required")
	}
	return a.s.Manager.Start(q.RequestId, "roproc", q, func(ctx context.Context, emit operations.Emit) (string, string, error) {
		err := a.s.RegistryApply(ctx, q, emit)
		return "SUCCEEDED", "Process " + q.Action + " completed: " + q.Process, err
	})
}
func (a *registryAPI) Get(ctx context.Context, q *api.RegistryOperationQuery) (*api.ControlOperation, error) {
	if err := a.authorize(ctx, q.PackageId, "MONITOR"); err != nil {
		return nil, err
	}
	return a.s.Manager.Get(q.Id)
}
func (a *registryAPI) Watch(q *api.RegistryOperationQuery, stream grpc.ServerStreamingServer[api.ControlOperation]) error {
	if err := a.authorize(stream.Context(), q.PackageId, "MONITOR"); err != nil {
		return err
	}
	return a.s.Manager.Watch(stream.Context(), q.Id, stream.Send)
}
