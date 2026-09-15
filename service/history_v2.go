package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fatima-go/fatima-core/v2/builder"
	"github.com/fatima-go/fatima-opm/api"
	"github.com/fatima-go/fatima-log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *DomainService) historyRootV2() string {
	return filepath.Join(s.fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder(), deploymentHistoryDataDir)
}
func (s *DomainService) deploymentHistoryV2(q *api.HistoryQuery) (*api.HistoryList, error) {
	pack := s.fatimaRuntime.GetPackaging()
	return deploymentHistory(pack.GetHost()+":"+pack.GetName(), builder.NewYamlFatimaPackageConfig(s.fatimaRuntime.GetEnv()), s.historyRootV2(), q.Process)
}

// deploymentHistory lists the records kept for each registered process, newest
// first. A non-empty process narrows the list to that registered process.
func deploymentHistory(packageID string, cfg *builder.YamlFatimaPackageConfig, root, process string) (*api.HistoryList, error) {
	out := &api.HistoryList{PackageId: packageID}
	found := false
	for _, p := range cfg.Processes {
		if process != "" && !strings.EqualFold(p.Name, process) {
			continue
		}
		found = true
		records, err := readProcessHistory(root, p.Name, groupName(cfg, p.Gid))
		if err != nil {
			return nil, err
		}
		out.Records = append(out.Records, records...)
	}
	if process != "" && !found {
		return nil, status.Error(codes.NotFound, "process "+process+" is not registered")
	}
	sort.SliceStable(out.Records, func(i, j int) bool { return out.Records[i].DeployedAt > out.Records[j].DeployedAt })
	return out, nil
}
func groupName(cfg *builder.YamlFatimaPackageConfig, gid int) string {
	for _, g := range cfg.Groups {
		if g.Id == gid {
			return g.Name
		}
	}
	return ""
}
func readProcessHistory(root, process, group string) ([]*api.DeploymentRecord, error) {
	dir := filepath.Join(root, process)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*api.DeploymentRecord
	for _, e := range entries {
		ms := isNumeric(e.Name())
		if e.IsDir() || ms <= 0 {
			continue
		}
		m, err := readFileAsMap(filepath.Join(dir, e.Name()))
		if err != nil {
			log.Warn("skip deployment history : %s", err.Error())
			continue
		}
		out = append(out, &api.DeploymentRecord{
			Process: process, Group: group, DeployedAt: ms,
			BuildUser: stringAt(m, "build", "user"), BuildTime: stringAt(m, "build", "time"),
			GitBranch: stringAt(m, "build", "git", "branch"), GitCommit: stringAt(m, "build", "git", "commit"),
			GitMessage: strings.TrimSpace(stringAt(m, "build", "git", "message")),
		})
	}
	return out, nil
}

// stringAt walks the nested deployment.json maps; a missing key reads as "".
func stringAt(m map[string]interface{}, path ...string) string {
	var v interface{} = m
	for _, key := range path {
		next, ok := v.(map[string]interface{})
		if !ok {
			return ""
		}
		v = next[key]
	}
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// legacyHistory reads each process's own history directory, newest first.
// Group and all queries used to read the parent directory and found nothing.
func legacyHistory(root string, names []string) []map[string]interface{} {
	history := make([]map[string]interface{}, 0)
	for _, name := range names {
		dir := filepath.Join(root, name)
		files, _ := readFilesInDir(dir)
		for _, f := range files {
			m, err := readFileAsMap(filepath.Join(dir, strconv.Itoa(f)))
			if err != nil {
				log.Warn("readFileAsMap : %s", err.Error())
				continue
			}
			m["deployment_time"] = f
			m["process"] = name
			history = append(history, m)
		}
	}
	sort.SliceStable(history, func(i, j int) bool {
		return history[i]["deployment_time"].(int) > history[j]["deployment_time"].(int)
	})
	return history
}
