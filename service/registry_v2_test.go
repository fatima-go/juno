package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fatima-go/fatima-opm/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func registryFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fatima-package.yaml")
	if err := os.WriteFile(path, []byte("# keep this comment\nextra: preserve\ngroup: [{id: 1, name: OPM}, {id: 4, name: SVC}]\nprocess:\n  - {name: juno, gid: 1, loglevel: info}\n  - {name: sample, gid: 4, loglevel: info, startsec: 7}\n"), 0640); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestRegistryEditsAndConflicts(t *testing.T) {
	path := registryFixture(t)
	d, err := readRegistryDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	q, err := d.prepare(&api.RegistryRequest{Action: "add", Process: "newproc", Group: "SVC"})
	if err != nil || q.Group != "4" {
		t.Fatal(q, err)
	}
	if err = d.save(path); err != nil {
		t.Fatal(err)
	}
	bytes, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if !strings.Contains(string(bytes), "keep this comment") || !strings.Contains(string(bytes), "extra: preserve") || !strings.Contains(string(bytes), "startsec: 7") || info.Mode().Perm() != 0640 {
		t.Fatal(string(bytes), info.Mode())
	}
	d, err = readRegistryDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.prepare(&api.RegistryRequest{Action: "remove", Process: "newproc", ExpectedRevision: q.ExpectedRevision})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	d, _ = readRegistryDocument(path)
	_, err = d.prepare(&api.RegistryRequest{Action: "remove", Process: "NEWPROC"})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	external := append(before, []byte("# external edit\n")...)
	os.WriteFile(path, external, 0640)
	if err = d.save(path); status.Code(err) != codes.FailedPrecondition {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(external) {
		t.Fatal("concurrent edit overwritten")
	}
}
func TestRegistryProtectsNamesAndGroups(t *testing.T) {
	path := registryFixture(t)
	for _, q := range []*api.RegistryRequest{
		{Action: "add", Process: "../sample", Group: "4"}, {Action: "add", Process: "revision", Group: "4"},
		{Action: "remove", Process: "juno"}, {Action: "add", Process: "other", Group: "OPM"},
		{Action: "add", Process: "other", Group: "999"}, {Action: "add", Process: "Sample", Group: "4"},
	} {
		d, _ := readRegistryDocument(path)
		if _, err := d.prepare(q); err == nil {
			t.Fatal("unsafe registration allowed", q)
		}
	}
}
func TestRegistryRejectsSymlinkConfig(t *testing.T) {
	path := registryFixture(t)
	link := filepath.Join(filepath.Dir(path), "link.yaml")
	os.Symlink(path, link)
	if err := replaceRegistryFile(link, []byte("replacement")); err == nil {
		t.Fatal("symlink configuration was replaced")
	}
}
