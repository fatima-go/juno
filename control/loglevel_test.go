package control

import (
	"context"
	"testing"

	"github.com/fatima-go/fatima-core/opm/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLogLevelRolesAndValidation(t *testing.T) {
	var roles, applied []string
	s := &Server{
		Authorize: func(_ context.Context, role string) error { roles = append(roles, role); return nil },
		LogLevels: func() (*api.LogLevelCatalog, error) { return &api.LogLevelCatalog{}, nil },
		SetLogLevel: func(p, l string) (*api.LogLevelEntry, error) {
			applied = append(applied, p+"="+l)
			return &api.LogLevelEntry{Process: p, Level: l}, nil
		},
	}
	a := &logLevelAPI{s: s}
	if _, err := a.List(context.Background(), &api.Empty{}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Set(context.Background(), &api.LogLevelRequest{Process: "worker", Level: " DEBUG "}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []*api.LogLevelRequest{{Process: "worker", Level: "fatal"}, {Process: "worker"}, {Process: "../worker", Level: "info"}} {
		if _, err := a.Set(context.Background(), q); status.Code(err) != codes.InvalidArgument {
			t.Fatal(q, err)
		}
	}
	if len(applied) != 1 || applied[0] != "worker=debug" || roles[0] != "MONITOR" || roles[1] != "OPERATOR" {
		t.Fatal(applied, roles)
	}
}
