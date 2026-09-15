package deployment

import (
	"context"
	"os"

	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-opm/artifact"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Cancel seals an ID even if Stage has not arrived. The same durable transaction
// as Start decides which won; an already running operation is never killed.
func (s *Server) Cancel(ctx context.Context, spec *api.OperationSpec) (*api.Operation, error) {
	if spec == nil || !artifact.SafeName.MatchString(spec.Id) || !artifact.SafeName.MatchString(spec.Process) || !artifact.SafeName.MatchString(spec.ArtifactId) || len(spec.Sha256) != 64 || spec.Size <= 0 || spec.PackageId != s.PackageID {
		return nil, status.Error(codes.InvalidArgument, "invalid cancellation specification")
	}
	if err := s.auth(ctx, spec.Id, spec.Sha256); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return nil, status.Error(codes.Unavailable, "Juno stopping")
	}
	var db database
	var op *api.Operation
	err := s.Store.Update("operations", &db, func() error {
		if db.Records == nil {
			db.Records = map[string]*record{}
		}
		r := db.Records[spec.Id]
		if r == nil {
			r = &record{Spec: proto.Clone(spec).(*api.OperationSpec), Operation: &api.Operation{Id: spec.Id, ArtifactId: spec.ArtifactId, Sha256: spec.Sha256, Process: spec.Process, PackageId: spec.PackageId, State: "STAGED"}}
			db.Records[spec.Id] = r
		}
		if !proto.Equal(r.Spec, spec) {
			return status.Error(codes.AlreadyExists, "operation ID conflict")
		}
		op = r.Operation
		if op.State == "STAGED" {
			op.State = "CANCELLED"
			op.FinishedAt = s.Now().Unix()
			appendEvent(op, "result", "CANCELLED", "Cancelled before execution", 0, 0, s.Now())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if op.State == "CANCELLED" {
		_ = os.Remove(s.path(spec.Id))
	}
	return proto.Clone(op).(*api.Operation), nil
}
