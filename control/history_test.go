package control

import (
	"context"
	"testing"

	"github.com/fatima-go/fatima-opm/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHistoryRoleAndValidation(t *testing.T) {
	var roles, queried []string
	s := &Server{
		Authorize: func(_ context.Context, role string) error { roles = append(roles, role); return nil },
		History: func(q *api.HistoryQuery) (*api.HistoryList, error) {
			queried = append(queried, q.Process)
			return &api.HistoryList{}, nil
		},
	}
	a := &historyAPI{s: s}
	for _, process := range []string{"", "worker"} {
		if _, err := a.List(context.Background(), &api.HistoryQuery{Process: process}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.List(context.Background(), &api.HistoryQuery{Process: "../worker"}); status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	if len(queried) != 2 || roles[0] != "MONITOR" {
		t.Fatal(queried, roles)
	}
}
