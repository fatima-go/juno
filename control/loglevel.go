package control

import (
	"context"
	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/artifact"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"slices"
	"strings"
)

// LogLevels are the only levels rolog may apply.
var LogLevels = []string{"error", "warn", "info", "debug", "trace"}

type logLevelAPI struct {
	api.UnimplementedLogLevelControlServer
	s *Server
}

func (a *logLevelAPI) List(ctx context.Context, _ *api.Empty) (*api.LogLevelCatalog, error) {
	if err := a.s.Authorize(ctx, "MONITOR"); err != nil {
		return nil, err
	}
	return a.s.LogLevels()
}
func (a *logLevelAPI) Set(ctx context.Context, q *api.LogLevelRequest) (*api.LogLevelEntry, error) {
	if err := a.s.Authorize(ctx, "OPERATOR"); err != nil {
		return nil, err
	}
	level := strings.ToLower(strings.TrimSpace(q.Level))
	if !slices.Contains(LogLevels, level) {
		return nil, status.Error(codes.InvalidArgument, "level must be one of error, warn, info, debug, trace")
	}
	if !artifact.SafeName.MatchString(q.Process) {
		return nil, status.Error(codes.InvalidArgument, "invalid process name")
	}
	return a.s.SetLogLevel(q.Process, level)
}
