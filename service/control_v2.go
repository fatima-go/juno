package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/ipc"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/transport"
	"github.com/fatima-go/juno/control"
	"github.com/fatima-go/juno/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *DomainService) NewControlV2() (*control.Server, error) {
	pack := s.fatimaRuntime.GetPackaging()
	auth := func(ctx context.Context, role string) error {
		p, ok := peer.FromContext(ctx)
		if !ok || !s.IsRemoteOperationAllowed(p.Addr.String()) {
			return status.Error(codes.PermissionDenied, "remote operation is disabled")
		}
		c, err := transport.Dial(s.getGatewayAddress(""))
		if err != nil {
			return err
		}
		defer c.Close()
		check, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err = api.NewIdentityClient(c).Validate(transport.WithToken(check, transport.Token(ctx)), &api.ValidateRequest{Role: role})
		return err
	}
	root := filepath.Join(s.fatimaRuntime.GetEnv().GetFolderGuide().GetDataFolder(), "control-v2")
	c, err := control.New(root, pack.GetHost()+":"+pack.GetName(), auth)
	if err != nil {
		return nil, err
	}
	c.ListJobs = s.listCronV2
	c.RunJob = s.runCronV2
	c.Processes = s.processCatalogV2
	c.StopProcesses = s.stopProcessesV2
	c.StartProcesses = s.startProcessesV2
	c.RegistryCatalog = s.registryCatalogV2
	c.RegistryPreview = s.registryPreviewV2
	c.RegistryApply = s.registryApplyV2
	c.LogLevels = s.logLevelCatalogV2
	c.SetLogLevel = s.setLogLevelV2
	c.History = s.deploymentHistoryV2
	return c, nil
}
func (s *DomainService) listCronV2() (result *api.CronCatalog, err error) {
	defer func() {
		if v := recover(); v != nil {
			result = nil
			err = fmt.Errorf("cannot read cron catalog: %v", v)
		}
	}()
	pack := s.fatimaRuntime.GetPackaging()
	out := &api.CronCatalog{PackageId: pack.GetHost() + ":" + pack.GetName()}
	cfg := builder.NewYamlFatimaPackageConfig(s.fatimaRuntime.GetEnv())
	for _, p := range cfg.Processes {
		if p.Gid == 1 {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.GetCronsDir(), buildCronJsonFilename(p.Name)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var record struct {
			Jobs []struct{ Name, Desc, Spec, Sample string }
		}
		if err = json.Unmarshal(b, &record); err != nil {
			return nil, fmt.Errorf("invalid cron catalog for %s: %w", p.Name, err)
		}
		for _, j := range record.Jobs {
			out.Jobs = append(out.Jobs, &api.CronEntry{Process: p.Name, Name: j.Name, Description: j.Desc, Spec: j.Spec, Sample: j.Sample})
		}
	}
	var processes []domain.ProcessBatch
	for _, j := range out.Jobs {
		processes = append(processes, domain.NewProcessBatch(j.Process, domain.BatchJob{Name: j.Name, Description: j.Description, Spec: j.Spec}))
	}
	for _, hour := range rebuildHourlyBatches(processes).List {
		h := &api.CronHour{Hour: int32(hour.Hour)}
		for _, p := range hour.ProcessList {
			for _, j := range p.JobList {
				h.Jobs = append(h.Jobs, &api.CronEntry{Process: p.ProcessName, Name: j.Name, Description: j.Description, Spec: j.Spec})
			}
		}
		out.Hours = append(out.Hours, h)
	}
	return out, nil
}
func (s *DomainService) runCronV2(ctx context.Context, q *api.CronRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	p := builder.NewYamlFatimaPackageConfig(s.fatimaRuntime.GetEnv()).GetProcByName(q.Process)
	if p == nil || GetPid(s.fatimaRuntime.GetEnv(), p) < 1 {
		return "", fmt.Errorf("process is not running")
	}
	args := strings.TrimSpace(q.Arguments)
	if ipc.IsFatimaIPCAvailable(q.Process) {
		if err := requestRerunCronWithIPC(q.Process, q.Job, args); err != nil {
			return "", fmt.Errorf("IPC delivery could not be confirmed; inspect the batch before a new request: %w", err)
		}
		return "Execution request delivered through IPC. Batch start, completion and success are not observable.", nil
	}
	file := filepath.Join(s.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(), "data", q.Process, "cron.rerun")
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", fmt.Errorf("cannot queue request (an earlier request may still be pending): %w", err)
	}
	content := q.Job
	if args != "" {
		content += " " + args
	}
	_, err = f.WriteString(content)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return "Execution request queued in cron.rerun. Batch start, completion and success are not observable.", nil
}
