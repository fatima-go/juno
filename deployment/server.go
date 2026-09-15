// Package deployment implements new operations without changing legacy handlers.
package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/artifact"
	"github.com/fatima-go/fatima-core/opm/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Result struct {
	RevisionPath     string
	PreviousRevision string
}
type Emit func(stage, state, message string, current, total int64) error
type Executor func(context.Context, *api.OperationSpec, string, Emit) (Result, error)
type Authorize func(context.Context, *api.ValidateRequest) error
type record struct {
	Spec      *api.OperationSpec
	Operation *api.Operation
}
type database struct{ Records map[string]*record }

type Server struct {
	api.UnimplementedPackageDeploymentServer
	Store               *store.Store
	PackageID, Platform string
	Authorize           Authorize
	Execute             Executor
	CheckRevision       func(*api.Operation) error
	Now                 func() time.Time
	ctx                 context.Context
	cancel              context.CancelFunc
	lock                *os.File
	mu                  sync.Mutex
	closing             bool
	wg                  sync.WaitGroup
}

func New(root, packageID, platform string, auth Authorize, execute Executor) (*Server, error) {
	st, e := store.Open(root)
	if e != nil {
		return nil, e
	}
	lock, e := os.OpenFile(filepath.Join(root, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		lock.Close()
		return nil, fmt.Errorf("another Juno owns deployment storage: %w", e)
	}
	s := &Server{Store: st, PackageID: packageID, Platform: platform, Authorize: auth, Execute: execute, Now: time.Now, lock: lock}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	var db database
	e = st.Update("operations", &db, func() error {
		for _, r := range db.Records {
			if r.Operation.State == "RUNNING" {
				r.Operation.State = "INTERRUPTED"
				r.Operation.Error = "Juno restarted during execution; inspect process before retry"
				r.Operation.FinishedAt = s.Now().Unix()
				appendEvent(r.Operation, "recovery", "INTERRUPTED", r.Operation.Error, 0, 0, s.Now())
			}
		}
		return nil
	})
	if e != nil {
		s.Close()
		return nil, e
	}
	if e = os.MkdirAll(filepath.Join(root, "staged"), 0700); e != nil {
		s.Close()
		return nil, e
	}
	return s, nil
}
func (s *Server) Capabilities() *api.Capabilities {
	return &api.Capabilities{Server: "juno", ApiVersion: 2, Features: []string{"deployment", "progress", "resume", "deployment_cancel"}, PackageId: s.PackageID, Platform: s.Platform}
}
func (s *Server) Register(g *grpc.Server) { api.RegisterPackageDeploymentServer(g, s) }
func (s *Server) Close() {
	s.mu.Lock()
	s.closing = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	if s.lock != nil {
		_ = syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
		_ = s.lock.Close()
		s.lock = nil
	}
}
func (s *Server) path(id string) string { return filepath.Join(s.Store.Dir, "staged", id+".far") }
func (s *Server) auth(ctx context.Context, id, hash string) error {
	return s.Authorize(ctx, &api.ValidateRequest{Role: "OPERATOR", OperationId: id, Sha256: hash, PackageId: s.PackageID})
}
func (s *Server) get(id string) (*record, error) {
	var db database
	if e := s.Store.Update("operations", &db, nil); e != nil {
		return nil, status.Error(codes.Internal, e.Error())
	}
	r := db.Records[id]
	if r == nil {
		return nil, status.Error(codes.NotFound, "operation not found")
	}
	return r, nil
}
func (s *Server) Stage(stream grpc.ClientStreamingServer[api.StageChunk, api.Operation]) error {
	first, e := stream.Recv()
	if e != nil {
		return status.Error(codes.InvalidArgument, "operation specification required")
	}
	spec := first.Spec
	if spec == nil || len(first.Data) != 0 || !artifact.SafeName.MatchString(spec.Id) || !artifact.SafeName.MatchString(spec.ArtifactId) || !artifact.SafeName.MatchString(spec.Process) || spec.Size <= 0 || spec.Size > artifact.MaxSize || len(spec.Sha256) != 64 || spec.PackageId != s.PackageID {
		return status.Error(codes.InvalidArgument, "invalid operation specification")
	}
	if e = s.auth(stream.Context(), spec.Id, spec.Sha256); e != nil {
		return e
	}
	if r, e := s.get(spec.Id); e == nil {
		if !proto.Equal(spec, r.Spec) {
			return status.Error(codes.AlreadyExists, "operation ID conflict")
		}
		return stream.SendAndClose(r.Operation)
	} else if status.Code(e) != codes.NotFound {
		return e
	}
	if spec.ExpiresAt <= s.Now().Unix() {
		return status.Error(codes.FailedPrecondition, "artifact expired")
	}
	f, e := os.CreateTemp(filepath.Join(s.Store.Dir, "staged"), ".upload-")
	if e != nil {
		return status.Error(codes.Internal, e.Error())
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	var n int64
	for {
		chunk, e := stream.Recv()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if chunk.Spec != nil || len(chunk.Data) == 0 || n+int64(len(chunk.Data)) > spec.Size {
			return status.Error(codes.InvalidArgument, "invalid stage chunk")
		}
		if spec.ExpiresAt <= s.Now().Unix() {
			return status.Error(codes.FailedPrecondition, "artifact expired before staging completed")
		}
		written, e := io.MultiWriter(f, h).Write(chunk.Data)
		if e != nil {
			return status.Error(codes.Internal, e.Error())
		}
		n += int64(written)
	}
	if n != spec.Size || hex.EncodeToString(h.Sum(nil)) != spec.Sha256 {
		return status.Error(codes.DataLoss, "FAR size or SHA-256 mismatch")
	}
	if e = f.Sync(); e != nil {
		return status.Error(codes.Internal, e.Error())
	}
	if e = f.Close(); e != nil {
		return status.Error(codes.Internal, e.Error())
	}
	info, e := artifact.Inspect(f.Name())
	if e != nil || info.Process != spec.Process {
		return status.Errorf(codes.InvalidArgument, "invalid FAR: %v", e)
	}
	if len(info.Platforms) > 0 {
		found := false
		for _, p := range info.Platforms {
			if p == s.Platform {
				found = true
			}
		}
		if !found {
			return status.Error(codes.FailedPrecondition, "FAR does not contain this platform")
		}
	}
	op := &api.Operation{Id: spec.Id, ArtifactId: spec.ArtifactId, Sha256: spec.Sha256, Process: spec.Process, PackageId: s.PackageID, State: "STAGED"}
	appendEvent(op, "stage", "SUCCEEDED", "FAR received and SHA-256 verified", n, n, s.Now())
	var db database
	e = s.Store.Update("operations", &db, func() error {
		if db.Records == nil {
			db.Records = map[string]*record{}
		}
		if r := db.Records[spec.Id]; r != nil {
			if !proto.Equal(spec, r.Spec) {
				return status.Error(codes.AlreadyExists, "operation ID conflict")
			}
			op = r.Operation
			return nil
		}
		if spec.ExpiresAt <= s.Now().Unix() {
			return status.Error(codes.FailedPrecondition, "artifact expired before staging completed")
		}
		if e := os.Rename(f.Name(), s.path(spec.Id)); e != nil {
			return e
		}
		db.Records[spec.Id] = &record{Spec: spec, Operation: op}
		return nil
	})
	if e != nil {
		return status.Error(codes.Internal, e.Error())
	}
	return stream.SendAndClose(op)
}
func (s *Server) Start(ctx context.Context, q *api.OperationQuery) (*api.Operation, error) {
	if e := s.auth(ctx, q.Id, ""); e != nil {
		return nil, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return nil, status.Error(codes.Unavailable, "Juno stopping")
	}
	var db database
	var r *record
	launch := false
	e := s.Store.Update("operations", &db, func() error {
		r = db.Records[q.Id]
		if r == nil {
			return status.Error(codes.NotFound, "operation not found")
		}
		if r.Operation.State != "STAGED" {
			return nil
		}
		for id, other := range db.Records {
			if id != q.Id && other.Spec.Process == r.Spec.Process && other.Operation.State == "RUNNING" {
				return status.Errorf(codes.Aborted, "process already owned by operation %s", id)
			}
		}
		r.Operation.State = "RUNNING"
		r.Operation.StartedAt = s.Now().Unix()
		appendEvent(r.Operation, "execute", "RUNNING", "Juno accepted durable operation", 0, 0, s.Now())
		launch = true
		return nil
	})
	if e != nil {
		return nil, e
	}
	response := proto.Clone(r.Operation).(*api.Operation)
	if launch {
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.run(r.Spec) }()
	}
	return response, nil
}
func (s *Server) Get(ctx context.Context, q *api.OperationQuery) (*api.Operation, error) {
	if e := s.auth(ctx, q.Id, ""); e != nil {
		return nil, e
	}
	r, e := s.get(q.Id)
	if e != nil {
		return nil, e
	}
	if r.Operation.State == "SUCCEEDED" && s.CheckRevision != nil {
		if e = s.CheckRevision(r.Operation); e != nil {
			r.Operation.State = "DRIFTED"
			r.Operation.Error = e.Error()
		}
	}
	return r.Operation, nil
}

func (s *Server) Watch(q *api.OperationQuery, stream grpc.ServerStreamingServer[api.Operation]) error {
	if e := s.auth(stream.Context(), q.Id, ""); e != nil {
		return e
	}
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	var last *api.Operation
	for {
		r, e := s.get(q.Id)
		if e != nil {
			return e
		}
		if !proto.Equal(last, r.Operation) {
			if e = stream.Send(r.Operation); e != nil {
				return e
			}
			last = r.Operation
		}
		if r.Operation.State != "RUNNING" {
			return nil
		}
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-s.ctx.Done():
			return status.Error(codes.Unavailable, "Juno stopping; reconcile the same operation ID")
		case <-tick.C:
		}
	}
}
func appendEvent(op *api.Operation, stage, state, message string, current, total int64, now time.Time) {
	seq := uint64(1)
	if len(op.Events) > 0 {
		seq = op.Events[len(op.Events)-1].Sequence + 1
	}
	// Waiting events are updated in place. Keep step transitions, bounded even
	// for a long wait; snapshots carry a monotonically increasing sequence.
	event := &api.Event{Sequence: seq, At: now.UnixMilli(), Stage: stage, State: state, Message: message, Current: current, Total: total}
	if n := len(op.Events); n > 0 && state == "WAITING" && op.Events[n-1].Stage == stage && op.Events[n-1].State == state {
		op.Events[n-1] = event
	} else {
		op.Events = append(op.Events, event)
	}
	if len(op.Events) > 256 {
		op.Events = op.Events[len(op.Events)-256:]
	}
}
func (s *Server) run(spec *api.OperationSpec) {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Minute)
	defer cancel()
	emit := func(stage, state, message string, current, total int64) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		var db database
		return s.Store.Update("operations", &db, func() error {
			r := db.Records[spec.Id]
			if r == nil {
				return fmt.Errorf("operation missing")
			}
			appendEvent(r.Operation, stage, state, message, current, total, s.Now())
			return nil
		})
	}
	var result Result
	var err error
	// Unexpected executor panics must leave an inspectable result, not a live
	// reservation or a crash that could trigger legacy automatic restarts.
	func() {
		defer func() {
			if v := recover(); v != nil {
				err = fmt.Errorf("executor panic: %v", v)
			}
		}()
		result, err = s.Execute(ctx, spec, s.path(spec.Id), emit)
	}()
	var db database
	saveErr := s.Store.Update("operations", &db, func() error {
		r := db.Records[spec.Id]
		r.Operation.RevisionPath = result.RevisionPath
		r.Operation.PreviousRevision = result.PreviousRevision
		r.Operation.FinishedAt = s.Now().Unix()
		state := "SUCCEEDED"
		message := "deployment complete"
		if err != nil {
			state = "FAILED"
			message = err.Error()
			r.Operation.Error = message
		}
		if s.ctx.Err() != nil {
			state = "INTERRUPTED"
		}
		r.Operation.State = state
		appendEvent(r.Operation, "result", state, message, 0, 0, s.Now())
		return nil
	})
	if saveErr == nil {
		_ = os.Remove(s.path(spec.Id))
	}
}
