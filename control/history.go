package control

import (
	"context"
	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/artifact"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type historyAPI struct {
	api.UnimplementedDeploymentHistoryServer
	s *Server
}

func (a *historyAPI) List(ctx context.Context, q *api.HistoryQuery) (*api.HistoryList, error) {
	if err := a.s.Authorize(ctx, "MONITOR"); err != nil {
		return nil, err
	}
	if q.Process != "" && !artifact.SafeName.MatchString(q.Process) {
		return nil, status.Error(codes.InvalidArgument, "invalid process name")
	}
	return a.s.History(q)
}
