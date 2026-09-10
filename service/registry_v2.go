package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/artifact"
	"github.com/fatima-go/fatima-core/opm/operations"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

type registryDocument struct {
	raw      []byte
	node     yaml.Node
	config   builder.YamlFatimaPackageConfig
	revision string
}

func readRegistryDocument(path string) (*registryDocument, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("package configuration must be a regular file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := &registryDocument{raw: b}
	if err = yaml.Unmarshal(b, &d.node); err != nil {
		return nil, err
	}
	if err = d.node.Decode(&d.config); err != nil {
		return nil, err
	}
	if len(d.config.Groups) == 0 || len(d.node.Content) != 1 || d.node.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("invalid package registration configuration")
	}
	hash := sha256.Sum256(b)
	d.revision = hex.EncodeToString(hash[:])
	return d, nil
}
func registryProcessName(name string) bool {
	if !artifact.SafeName.MatchString(name) || len(name) > 128 {
		return false
	}
	switch strings.ToLower(name) {
	case "jupiter", "juno", "saturn", "revision", "package":
		return false
	}
	return true
}

// Edit only the process sequence, retaining comments and unrelated YAML fields.
func (d *registryDocument) prepare(q *api.RegistryRequest) (*api.RegistryRequest, error) {
	if !registryProcessName(q.Process) {
		return nil, status.Error(codes.InvalidArgument, "invalid or protected process name")
	}
	if q.Action != "add" && q.Action != "remove" {
		return nil, status.Error(codes.InvalidArgument, "action must be add or remove")
	}
	if q.ExpectedRevision != "" && q.ExpectedRevision != d.revision {
		return nil, status.Error(codes.FailedPrecondition, "package configuration changed; refresh the preview")
	}
	r := proto.Clone(q).(*api.RegistryRequest)
	r.ExpectedRevision = d.revision
	root := d.node.Content[0]
	var list *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "process" {
			list = root.Content[i+1]
		}
	}
	if list == nil || list.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("process sequence is missing")
	}
	found := -1
	for i, p := range d.config.Processes {
		if strings.EqualFold(p.Name, q.Process) {
			found = i
		}
	}
	if q.Action == "add" {
		if found >= 0 {
			return nil, status.Error(codes.AlreadyExists, "process is already registered")
		}
		gid, err := strconv.Atoi(q.Group)
		if err != nil {
			gid = d.config.GetGroupId(q.Group)
		}
		if !d.config.IsValidGroupId(gid) || gid == 1 || gid == d.config.GetGroupId("opm") {
			return nil, status.Error(codes.InvalidArgument, "select a registered non-OPM group")
		}
		r.Group = strconv.Itoa(gid)
		var node yaml.Node
		if err = node.Encode(builder.ProcessItem{Name: r.Process, Gid: gid, Loglevel: "info"}); err != nil {
			return nil, err
		}
		list.Content = append(list.Content, &node)
	} else {
		if found < 0 {
			return nil, status.Error(codes.NotFound, "process is not registered")
		}
		p := d.config.Processes[found]
		if p.Gid == 1 || p.Gid == d.config.GetGroupId("opm") {
			return nil, status.Error(codes.PermissionDenied, "OPM registration cannot be removed")
		}
		r.Process = p.Name
		r.Group = strconv.Itoa(p.Gid)
		list.Content = append(list.Content[:found], list.Content[found+1:]...)
	}
	return r, nil
}
func (s *DomainService) registryCatalogV2(q *api.RegistryQuery) (*api.RegistryCatalog, error) {
	d, err := readRegistryDocument(s.fatimaRuntime.GetEnv().GetFolderGuide().GetPackageProcFile())
	if err != nil {
		return nil, err
	}
	c, err := s.processCatalogV2(&api.ProcessQuery{Timezone: q.Timezone})
	if err != nil {
		return nil, err
	}
	out := &api.RegistryCatalog{Catalog: c, Revision: d.revision}
	for _, g := range d.config.Groups {
		out.Groups = append(out.Groups, &api.ProcessGroup{Id: int32(g.Id), Name: g.Name})
	}
	return out, nil
}
func (s *DomainService) registryCleanupPaths(name string) []string {
	env := s.fatimaRuntime.GetEnv()
	home := env.GetFolderGuide().GetFatimaHome()
	return []string{filepath.Join(home, "app", name), filepath.Join(home, "app", "revision", name), filepath.Join(home, "log", name), filepath.Join(home, "data", name), buildHistorySaveDir(env, name), filepath.Join(s.GetCronsDir(), buildCronJsonFilename(name))}
}
func (s *DomainService) registryPreviewV2(q *api.RegistryRequest) (*api.RegistryPlan, error) {
	d, err := readRegistryDocument(s.fatimaRuntime.GetEnv().GetFolderGuide().GetPackageProcFile())
	if err != nil {
		return nil, err
	}
	r, err := d.prepare(q)
	if err != nil {
		return nil, err
	}
	p := &api.RegistryPlan{Request: r}
	if r.Action == "add" {
		p.Effects = []string{"프로세스 등록과 INFO 로그 레벨 설정", "Juno의 기존 자동 기동 정책이 적용됩니다."}
	} else {
		p.Effects = []string{"실행 중이면 goaway와 PID 종료 확인 후 삭제", "삭제 대상 (복구되지 않음):"}
		p.Effects = append(p.Effects, s.registryCleanupPaths(r.Process)...)
		p.Effects = append(p.Effects, "로그 레벨과 프로세스 등록 제거")
	}
	return p, nil
}
func registryLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another registration operation is running")
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
func replaceRegistryFile(path string, data []byte) error {
	mode := os.FileMode(0644)
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace non-regular configuration %s", path)
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".registry-v2-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (d *registryDocument) save(path string) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, d.raw) {
		return status.Error(codes.FailedPrecondition, "package configuration changed during operation; inspect completed steps before retrying")
	}
	data, err := yaml.Marshal(&d.node)
	if err != nil {
		return err
	}
	return replaceRegistryFile(path, data)
}
func (s *DomainService) registryApplyV2(ctx context.Context, q *api.RegistryRequest, emit operations.Emit) error {
	path := s.fatimaRuntime.GetEnv().GetFolderGuide().GetPackageProcFile()
	unlock, err := registryLock(filepath.Join(filepath.Dir(path), ".registry-v2.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	release, err := s.lockProcessesV2([]string{q.Process})
	if err != nil {
		return err
	}
	defer release()
	d, err := readRegistryDocument(path)
	if err != nil {
		return err
	}
	r, err := d.prepare(q)
	if err != nil {
		return err
	}
	if err = emit("validate", "SUCCEEDED", "Registration preview and package configuration verified", 1, 1); err != nil {
		return err
	}
	if r.Action == "remove" {
		if err = s.stopObservedV2(ctx, d.config.GetProcByName(r.Process), emit); err != nil {
			return err
		}
		for i, p := range s.registryCleanupPaths(r.Process) {
			if err = ctx.Err(); err != nil {
				return err
			}
			if err = emit("cleanup", "RUNNING", "Removing "+p, int64(i), 6); err != nil {
				return err
			}
			if err = os.RemoveAll(p); err != nil {
				return fmt.Errorf("cleanup %s: %w", p, err)
			}
			if err = emit("cleanup", "SUCCEEDED", "Removed "+p, int64(i+1), 6); err != nil {
				return err
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	levelPath := filepath.Join(s.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(), domain.FOLDER_PACKAGE, domain.FOLDER_CFM, domain.FILE_LOG_LEVEL)
	levels := map[string]string{}
	b, err := os.ReadFile(levelPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(b) > 0 {
		if err = json.Unmarshal(b, &levels); err != nil {
			return err
		}
	}
	if levels == nil {
		levels = map[string]string{}
	}
	if r.Action == "add" {
		levels[r.Process] = log.ConvertLogLevelToHexa("info")
	} else {
		delete(levels, r.Process)
	}
	b, err = json.Marshal(levels)
	if err != nil {
		return err
	}
	if err = emit("loglevel", "RUNNING", "Updating process log level registration", 0, 1); err != nil {
		return err
	}
	if err = replaceRegistryFile(levelPath, b); err != nil {
		return err
	}
	if err = emit("loglevel", "SUCCEEDED", "Process log level registration updated", 1, 1); err != nil {
		return err
	}
	if err = emit("registration", "RUNNING", "Saving package configuration", 0, 1); err != nil {
		return err
	}
	if err = d.save(path); err != nil {
		return err
	}
	return emit("registration", "SUCCEEDED", "Process "+r.Action+" saved and synchronized to disk", 1, 1)
}
