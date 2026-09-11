package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatima-go/fatima-core/builder"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLogLevelCatalogAndApply(t *testing.T) {
	cfg := &builder.YamlFatimaPackageConfig{
		Groups:    []builder.GroupItem{{Id: 1, Name: "OPM"}, {Id: 4, Name: "SVC"}},
		Processes: []builder.ProcessItem{{Gid: 1, Name: "juno", Loglevel: "info"}, {Gid: 4, Name: "sample", Loglevel: "warn"}, {Gid: 4, Name: "plain"}},
	}
	path := filepath.Join(t.TempDir(), "loglevels")
	if err := os.WriteFile(path, []byte(`{"juno":"0x2F","other":"0x7"}`), 0644); err != nil {
		t.Fatal(err)
	}
	levels, err := readLogLevelFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range logLevelCatalog("host:default", cfg, levels).Entries {
		got[e.Process] = e.Group + "/" + e.Level
	}
	if got["juno"] != "OPM/debug" || got["sample"] != "SVC/warn" || got["plain"] != "SVC/" {
		t.Fatal(got)
	}
	e, err := applyLogLevel(path, cfg, "SAMPLE", "trace")
	if err != nil || e.Process != "sample" || e.Level != "trace" || e.Group != "SVC" {
		t.Fatal(e, err)
	}
	b, _ := os.ReadFile(path)
	for _, want := range []string{`"sample":"0xFF"`, `"juno":"0x2F"`, `"other":"0x7"`} {
		if !strings.Contains(string(b), want) {
			t.Fatal(string(b))
		}
	}
	if _, err = applyLogLevel(path, cfg, "missing", "info"); status.Code(err) != codes.NotFound {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte("{broken"), 0644)
	if _, err = applyLogLevel(path, cfg, "sample", "info"); err == nil {
		t.Fatal("unreadable loglevels file was rewritten")
	}
	if b, _ = os.ReadFile(path); string(b) != "{broken" {
		t.Fatal(string(b))
	}
}
