package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fatima-go/fatima-core/v2/builder"
	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// logLevelMutex serialises read-modify-write of the shared loglevels file.
var logLevelMutex sync.Mutex

func (s *DomainService) logLevelFileV2() string {
	return filepath.Join(s.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(), domain.FOLDER_PACKAGE, domain.FOLDER_CFM, domain.FILE_LOG_LEVEL)
}
func (s *DomainService) logLevelCatalogV2() (*api.LogLevelCatalog, error) {
	pack := s.fatimaRuntime.GetPackaging()
	levels, err := readLogLevelFile(s.logLevelFileV2())
	if err != nil {
		return nil, err
	}
	return logLevelCatalog(pack.GetHost()+":"+pack.GetName(), builder.NewYamlFatimaPackageConfig(s.fatimaRuntime.GetEnv()), levels), nil
}
func (s *DomainService) setLogLevelV2(process, level string) (*api.LogLevelEntry, error) {
	logLevelMutex.Lock()
	defer logLevelMutex.Unlock()
	return applyLogLevel(s.logLevelFileV2(), builder.NewYamlFatimaPackageConfig(s.fatimaRuntime.GetEnv()), process, level)
}

func logLevelCatalog(packageID string, cfg *builder.YamlFatimaPackageConfig, levels map[string]string) *api.LogLevelCatalog {
	out := &api.LogLevelCatalog{PackageId: packageID}
	for _, p := range cfg.Processes {
		out.Entries = append(out.Entries, logLevelEntry(cfg, p, effectiveLogLevel(p, levels)))
	}
	return out
}
func logLevelEntry(cfg *builder.YamlFatimaPackageConfig, p builder.ProcessItem, level string) *api.LogLevelEntry {
	group := ""
	for _, g := range cfg.Groups {
		if g.Id == p.Gid {
			group = g.Name
		}
	}
	return &api.LogLevelEntry{Process: p.Name, Group: group, Level: level, Opm: p.Gid == 1}
}

// effectiveLogLevel mirrors what a process applies: its loglevels file entry,
// otherwise the level configured in fatima-package.yaml.
func effectiveLogLevel(p builder.ProcessItem, levels map[string]string) string {
	if v, ok := levels[p.Name]; ok {
		if l, err := log.ConvertHexaToLogLevel(v); err == nil && l != log.LOG_NONE {
			return strings.ToLower(l.String())
		}
	}
	if l := log.ConvertStringToLogLevel(p.Loglevel); l != log.LOG_NONE {
		return strings.ToLower(l.String())
	}
	return ""
}

// applyLogLevel records level for a registered process. Unlike the legacy
// handler it refuses an unreadable file instead of rewriting it from scratch.
func applyLogLevel(path string, cfg *builder.YamlFatimaPackageConfig, process, level string) (*api.LogLevelEntry, error) {
	var item *builder.ProcessItem
	for i := range cfg.Processes {
		if strings.EqualFold(cfg.Processes[i].Name, process) {
			item = &cfg.Processes[i]
		}
	}
	if item == nil {
		return nil, status.Error(codes.NotFound, "process "+process+" is not registered")
	}
	levels, err := readLogLevelFile(path)
	if err != nil {
		return nil, err
	}
	levels[item.Name] = log.ConvertLogLevelToHexa(level)
	if err = writeLogLevelFile(path, levels); err != nil {
		return nil, err
	}
	return logLevelEntry(cfg, *item, level), nil
}
func readLogLevelFile(path string) (map[string]string, error) {
	levels := map[string]string{}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return levels, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 0 {
		if err = json.Unmarshal(b, &levels); err != nil {
			return nil, fmt.Errorf("invalid loglevels file: %w", err)
		}
	}
	if levels == nil {
		levels = map[string]string{}
	}
	return levels, nil
}

// writeLogLevelFile replaces the file atomically because every process reads
// it once a second.
func writeLogLevelFile(path string, levels map[string]string) error {
	b, err := json.Marshal(levels)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
