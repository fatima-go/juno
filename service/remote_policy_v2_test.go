package service

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	fatima "github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Construct the production service factories, not replacement auth callbacks.
type policyRuntime struct {
	fatima.FatimaRuntime
	root string
}

func (r policyRuntime) GetConfig() fatima.Config       { return policyConfig{} }
func (r policyRuntime) GetPackaging() fatima.Packaging { return policyPackage{} }
func (r policyRuntime) GetEnv() fatima.FatimaEnv       { return policyEnv{root: r.root} }

type policyConfig struct{ fatima.Config }

func (policyConfig) GetValue(string) (string, bool) { return "", false }

type policyPackage struct{ fatima.Packaging }

func (policyPackage) GetHost() string { return "host" }
func (policyPackage) GetName() string { return "package" }

type policyEnv struct {
	fatima.FatimaEnv
	root string
}

func (e policyEnv) GetFolderGuide() fatima.FolderGuide { return policyFolders{root: e.root} }

type policyFolders struct {
	fatima.FolderGuide
	root string
}

func (f policyFolders) GetDataFolder() string { return f.root }

type policyIdentity struct {
	api.UnimplementedIdentityServer
}

func (*policyIdentity) Validate(ctx context.Context, q *api.ValidateRequest) (*api.Session, error) {
	token := transport.Token(ctx)
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}
	if q.OperationId != "" {
		if token != "ticket" || q.PackageId != "host:package" || (q.OperationId != "op" && q.OperationId != "cancel-op") || (q.Sha256 != "" && q.Sha256 != strings.Repeat("a", 64)) {
			return nil, status.Error(codes.PermissionDenied, "invalid ticket scope")
		}
	} else if token != "operator" && !(token == "monitor" && q.Role == "MONITOR") {
		return nil, status.Error(codes.PermissionDenied, "insufficient role")
	}
	return &api.Session{}, nil
}
func policyEndpoint(t *testing.T, g *grpc.Server) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go g.Serve(l)
	t.Cleanup(g.Stop)
	return "http://" + l.Addr().String()
}

func TestV2RemotePolicyWiring(t *testing.T) {
	oldAllow, oldIP := remoteOperationAllowed, localIpAddress
	t.Cleanup(func() { remoteOperationAllowed, localIpAddress = oldAllow, oldIP })
	identity := grpc.NewServer()
	api.RegisterIdentityServer(identity, &policyIdentity{})
	t.Setenv(fatima.ENV_FATIMA_JUPITER_URI, policyEndpoint(t, identity))
	svc := &DomainService{fatimaRuntime: policyRuntime{root: t.TempDir()}}
	control, err := svc.NewControlV2()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(control.Close)
	deployment, err := svc.NewDeploymentV2()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(deployment.Close)
	// A sentinel proves allowed reads reached their handler without accessing
	// real process configuration or invoking any process mutation.
	reached := status.Error(codes.Unavailable, "handler reached")
	control.ListJobs = func() (*api.CronCatalog, error) { return nil, reached }
	control.Processes = func(*api.ProcessQuery) (*api.ProcessCatalog, error) { return nil, reached }
	control.RegistryCatalog = func(*api.RegistryQuery) (*api.RegistryCatalog, error) { return nil, reached }
	control.RegistryPreview = func(*api.RegistryRequest) (*api.RegistryPlan, error) { return nil, reached }
	control.LogLevels = func() (*api.LogLevelCatalog, error) { return nil, reached }
	control.History = func(*api.HistoryQuery) (*api.HistoryList, error) { return nil, reached }
	g := grpc.NewServer()
	control.Register(g)
	deployment.Register(g)
	c, err := transport.Dial(policyEndpoint(t, g))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	type check struct {
		request                      proto.Message
		want                         codes.Code
		restricted, operator, ticket bool
	}
	cases := map[string]check{
		"CronControl/List":            {&api.Empty{}, codes.Unavailable, false, false, false},
		"CronControl/Rerun":           {&api.CronRequest{}, codes.InvalidArgument, false, true, false},
		"CronControl/Get":             {&api.OperationQuery{Id: "missing"}, codes.NotFound, false, false, false},
		"CronControl/Watch":           {&api.OperationQuery{Id: "missing"}, codes.NotFound, false, false, false},
		"ProcessControl/Catalog":      {&api.ProcessQuery{}, codes.Unavailable, false, false, false},
		"ProcessControl/WatchCatalog": {&api.ProcessQuery{}, codes.Unavailable, false, false, false},
		"ProcessControl/Start":        {&api.ProcessRequest{}, codes.InvalidArgument, true, true, false},
		"ProcessControl/Stop":         {&api.ProcessRequest{}, codes.InvalidArgument, true, true, false},
		"ProcessControl/Get":          {&api.OperationQuery{Id: "missing"}, codes.NotFound, false, false, false},
		"ProcessControl/Watch":        {&api.OperationQuery{Id: "missing"}, codes.NotFound, false, false, false},
		"ProcessRegistry/Catalog":     {&api.RegistryQuery{PackageId: "host:package"}, codes.Unavailable, false, false, false},
		"ProcessRegistry/Preview":     {&api.RegistryRequest{PackageId: "host:package"}, codes.Unavailable, false, false, false},
		"ProcessRegistry/Apply":       {&api.RegistryRequest{PackageId: "host:package"}, codes.InvalidArgument, true, true, false},
		"ProcessRegistry/Get":         {&api.RegistryOperationQuery{PackageId: "host:package", Id: "missing"}, codes.NotFound, false, false, false},
		"ProcessRegistry/Watch":       {&api.RegistryOperationQuery{PackageId: "host:package", Id: "missing"}, codes.NotFound, false, false, false},
		"LogLevelControl/List":        {&api.Empty{}, codes.Unavailable, false, false, false},
		"LogLevelControl/Set":         {&api.LogLevelRequest{}, codes.InvalidArgument, false, true, false},
		"DeploymentHistory/List":      {&api.HistoryQuery{}, codes.Unavailable, false, false, false},
		"PackageDeployment/Cancel":    {&api.OperationSpec{Id: "cancel-op", ArtifactId: "artifact", Process: "process", PackageId: "host:package", Size: 1, Sha256: strings.Repeat("a", 64)}, codes.OK, false, true, true},
		"PackageDeployment/Stage":     {&api.StageChunk{Spec: &api.OperationSpec{Id: "op", ArtifactId: "artifact", Process: "process", PackageId: "host:package", Size: 1, Sha256: strings.Repeat("a", 64)}}, codes.FailedPrecondition, false, true, true},
		"PackageDeployment/Start":     {&api.OperationQuery{Id: "op"}, codes.NotFound, false, true, true},
		"PackageDeployment/Get":       {&api.OperationQuery{Id: "op"}, codes.NotFound, false, true, true},
		"PackageDeployment/Watch":     {&api.OperationQuery{Id: "op"}, codes.NotFound, false, true, true},
	}
	// Enumerating registered methods makes missing coverage fail when wiring changes.
	covered := 0
	for service, info := range g.GetServiceInfo() {
		for _, method := range info.Methods {
			name := strings.TrimPrefix(service, "fatima.opm.v2.") + "/" + method.Name
			tc, ok := cases[name]
			if !ok {
				t.Errorf("uncovered RPC %s", name)
				continue
			}
			covered++
			for _, scenario := range []struct {
				name, ip, token string
				allow           bool
			}{
				{"remote", "10.180.37.108", "operator", false},
				{"local", "127.0.0.1", "operator", false},
				{"enabled", "10.180.37.108", "operator", true},
				{"unauthenticated", "10.180.37.108", "", false},
				{"monitor", "127.0.0.1", "monitor", false},
				{"invalid-ticket", "127.0.0.1", "invalid", false},
			} {
				t.Run(name+"/"+scenario.name, func(t *testing.T) {
					remoteOperationAllowed, localIpAddress = scenario.allow, scenario.ip
					token := scenario.token
					if tc.ticket && token == "operator" {
						token = "ticket"
					}
					want := tc.want
					if token == "" {
						want = codes.Unauthenticated
					} else if token == "invalid" || (token == "monitor" && tc.operator) {
						want = codes.PermissionDenied
					} else if tc.restricted && !scenario.allow && scenario.ip != "127.0.0.1" {
						want = codes.PermissionDenied
					}
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					ctx = transport.WithToken(ctx, token)
					path := "/" + service + "/" + method.Name
					var err error
					if method.IsClientStream || method.IsServerStream {
						stream, e := c.NewStream(ctx, &grpc.StreamDesc{ClientStreams: method.IsClientStream, ServerStreams: method.IsServerStream}, path)
						err = e
						if err == nil {
							err = stream.SendMsg(tc.request)
						}
						if err == nil {
							err = stream.CloseSend()
						}
						if err == nil {
							err = stream.RecvMsg(&api.Empty{})
						}
					} else {
						err = c.Invoke(ctx, path, tc.request, &api.Empty{})
					}
					if status.Code(err) != want {
						t.Fatalf("got %v, want %v", err, want)
					}
					if want == codes.Unavailable && status.Convert(err).Message() != "handler reached" {
						t.Fatalf("handler not reached: %v", err)
					}
				})
			}
		}
	}
	if covered != len(cases) {
		t.Fatalf("covered %d registered RPCs, expected %d", covered, len(cases))
	}
}
