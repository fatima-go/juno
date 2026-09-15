package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fatima-go/fatima-core/v2/builder"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func historyFixture(t *testing.T) (string, *builder.YamlFatimaPackageConfig) {
	t.Helper()
	root := t.TempDir()
	write := func(process, name, body string) {
		dir := filepath.Join(root, process)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("sample", "1700000000000", `{"process":"sample","build":{"user":"dave","time":"20260911120000 KST","git":{"branch":"main","commit":"abc123","message":"first\n"}}}`)
	write("sample", "1700000500000", `{"build":{"user":"jin"}}`)
	write("sample", "notes.txt", "ignored")
	write("sample", "1700000900000", "{broken")
	write("worker", "1690000000000", `{"build":{"user":"kim"}}`)
	write("removed", "1710000000000", `{"build":{"user":"gone"}}`)
	cfg := &builder.YamlFatimaPackageConfig{
		Groups:    []builder.GroupItem{{Id: 1, Name: "OPM"}, {Id: 4, Name: "SVC"}},
		Processes: []builder.ProcessItem{{Gid: 1, Name: "juno"}, {Gid: 4, Name: "sample"}, {Gid: 4, Name: "worker"}},
	}
	return root, cfg
}

func TestDeploymentHistoryV2(t *testing.T) {
	root, cfg := historyFixture(t)
	all, err := deploymentHistory("host:default", cfg, root, "")
	if err != nil || len(all.Records) != 3 {
		t.Fatal(all, err)
	}
	if r := all.Records[0]; r.Process != "sample" || r.DeployedAt != 1700000500000 || r.BuildUser != "jin" || r.Group != "SVC" {
		t.Fatal(r)
	}
	if r := all.Records[1]; r.GitBranch != "main" || r.GitCommit != "abc123" || r.GitMessage != "first" || r.BuildTime != "20260911120000 KST" {
		t.Fatal(r)
	}
	one, err := deploymentHistory("host:default", cfg, root, "WORKER")
	if err != nil || len(one.Records) != 1 || one.Records[0].BuildUser != "kim" {
		t.Fatal(one, err)
	}
	if _, err = deploymentHistory("host:default", cfg, root, "removed"); status.Code(err) != codes.NotFound {
		t.Fatal(err)
	}
}

func TestLegacyHistoryReadsEachProcess(t *testing.T) {
	root, _ := historyFixture(t)
	h := legacyHistory(root, []string{"sample", "worker"})
	if len(h) != 3 || h[0]["process"] != "sample" || h[0]["deployment_time"] != 1700000500000 || h[2]["process"] != "worker" {
		t.Fatal(h)
	}
}
