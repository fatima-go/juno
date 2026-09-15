package service

import (
	"context"
	"debug/elf"
	"debug/macho"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/ipc"
	"github.com/fatima-go/fatima-core/opm/api"
	"github.com/fatima-go/fatima-core/opm/artifact"
	"github.com/fatima-go/fatima-core/opm/transport"
	"github.com/fatima-go/juno/deployment"
	"github.com/fatima-go/juno/domain"
	"github.com/fatima-go/juno/service/goaway"
)

// NewDeploymentV2 is an additive adapter. The old DeployPackage, start/stop and
// goaway handlers retain their existing behavior and response contracts.
func (service *DomainService) NewDeploymentV2() (*deployment.Server, error) {
	rt := service.fatimaRuntime
	pack := rt.GetPackaging()
	root := filepath.Join(rt.GetEnv().GetFolderGuide().GetDataFolder(), "deployment-v2")
	if v, ok := rt.GetConfig().GetValue("deployment.v2.storage"); ok && v != "" {
		root = v
	}
	auth := func(ctx context.Context, q *api.ValidateRequest) error {
		conn, e := transport.Dial(service.getGatewayAddress(""))
		if e != nil {
			return e
		}
		defer conn.Close()
		check, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, e = api.NewIdentityClient(conn).Validate(transport.WithToken(check, transport.Token(ctx)), q)
		return e
	}
	s, e := deployment.New(root, pack.GetHost()+":"+pack.GetName(), runtime.GOOS+"_"+runtime.GOARCH, auth, service.executeDeploymentV2)
	if e == nil {
		s.CheckRevision = service.checkRevisionV2
	}
	return s, e
}

type revisionManifest struct {
	OperationID    string
	ArtifactSHA256 string
	Files          map[string]string
}

const manifestName = ".fatima-v2.json"

func (service *DomainService) executeDeploymentV2(ctx context.Context, spec *api.OperationSpec, far string, emit deployment.Emit) (result deployment.Result, err error) {
	unlock, lockErr := service.lockProcessesV2([]string{spec.Process})
	if lockErr != nil {
		return result, lockErr
	}
	defer unlock()
	env := service.fatimaRuntime.GetEnv()
	proc := builder.NewYamlFatimaPackageConfig(env).GetProcByName(spec.Process)
	if proc == nil {
		return result, fmt.Errorf("process %s is not registered", spec.Process)
	}
	if domain.IsManagedOpmProcessName(spec.Process) {
		return result, fmt.Errorf("update OPM with the package updater; self-deployment is unsupported")
	}
	if proc.GetPath() != "" {
		return result, fmt.Errorf("process uses an external executable path; v2 requires an executable inside its FAR")
	}
	if err = emit("validate", "RUNNING", "checking FAR and process configuration", 0, 0); err != nil {
		return
	}
	hash, size, e := artifact.Digest(far)
	if e != nil || hash != spec.Sha256 || size != spec.Size {
		return result, fmt.Errorf("staged artifact digest changed: %v", e)
	}
	appDir := filepath.Join(env.GetFolderGuide().GetFatimaHome(), builder.FatimaFolderApp)
	appPath := filepath.Join(appDir, spec.Process)
	revision := filepath.Join(appDir, domain.FOLDER_APP_REVISION, spec.Process, "v2_"+spec.Id)
	result.RevisionPath = revision
	if err = os.MkdirAll(revision, 0755); err != nil {
		return
	}
	installed := false
	defer func() {
		if !installed {
			_ = os.RemoveAll(revision)
		}
	}()
	if err = artifact.Extract(far, revision, runtime.GOOS+"_"+runtime.GOARCH); err != nil {
		return
	}
	if _, e = os.Lstat(filepath.Join(revision, manifestName)); e == nil {
		return result, fmt.Errorf("FAR contains reserved revision manifest %s", manifestName)
	} else if !os.IsNotExist(e) {
		return result, e
	}
	binary := filepath.Join(revision, spec.Process)
	if _, e = os.Stat(binary + ".sh"); e == nil {
		binary += ".sh"
	}
	if err = validateExecutableV2(binary); err != nil {
		return
	}
	manifest := revisionManifest{OperationID: spec.Id, ArtifactSHA256: spec.Sha256, Files: map[string]string{}}
	err = filepath.WalkDir(revision, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(revision, path)
		if e != nil {
			return e
		}
		sum, _, e := artifact.Digest(path)
		if e == nil {
			manifest.Files[rel] = sum
		}
		return e
	})
	if err != nil {
		return
	}
	encoded, e := json.Marshal(manifest)
	if e != nil {
		return result, e
	}
	if err = os.WriteFile(filepath.Join(revision, manifestName), encoded, 0600); err != nil {
		return
	}
	if err = emit("validate", "SUCCEEDED", "platform executable verified; installation prepared", size, size); err != nil {
		return
	}

	GetProcessMonitor().ProcessStop(spec.Process)
	success := false
	defer func() {
		if success {
			GetProcessMonitor().ProcessStart(spec.Process)
		} else {
			GetProcessMonitor().ProcessStop(spec.Process)
		}
	}()
	pid := GetPid(env, proc)
	if pid > 1 && runningV2(spec.Process, pid) {
		if err = emit("goaway", "RUNNING", fmt.Sprintf("begin drain for pid %d", pid), 0, 0); err != nil {
			return
		}
		if ipc.IsFatimaIPCAvailable(spec.Process) {
			err = goaway.ExecuteObserved(ctx, spec.Process, func(message string, elapsed int64) error { return emit("goaway", "WAITING", message, elapsed, 31) })
			if err != nil {
				return
			}
		} else if isFatimaOrientProcess(env, proc) {
			// Old processes cannot acknowledge goaway. Preserve their signal flow but
			// expose the distinction and a bounded grace window instead of inventing
			// a completion acknowledgement.
			if err = syscall.Kill(pid, syscall.SIGUSR1); err != nil {
				return
			}
			for elapsed := int64(0); elapsed < 31; elapsed++ {
				if err = emit("goaway", "WAITING", "legacy SIGUSR1 sent; no completion acknowledgement available", elapsed, 31); err != nil {
					return
				}
				select {
				case <-ctx.Done():
					return result, ctx.Err()
				case <-time.After(time.Second):
				}
			}
		}
		script := filepath.Join(appPath, shellGoaway)
		if _, e = os.Stat(script); e == nil {
			cmd := exec.CommandContext(ctx, "/bin/sh", script)
			cmd.Dir = appPath
			if err = cmd.Run(); err != nil {
				return
			}
		}
		if err = emit("goaway", "SUCCEEDED", "drain protocol / configured grace window finished", 0, 0); err != nil {
			return
		}
		if err = emit("shutdown", "RUNNING", fmt.Sprintf("sending SIGTERM to pid %d", pid), 0, 0); err != nil {
			return
		}
		if err = KillProgram(spec.Process, pid); err != nil && err != syscall.ESRCH {
			return
		}
		stopped := false
		for elapsed := int64(0); elapsed < 120; elapsed++ {
			if !runningV2(spec.Process, pid) {
				stopped = true
				break
			}
			if err = emit("shutdown", "WAITING", fmt.Sprintf("waiting for pid %d to exit", pid), elapsed, 120); err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(time.Second):
			}
		}
		if !stopped {
			return result, fmt.Errorf("process did not stop within 120 seconds; installation not applied")
		}
	}
	if err = emit("shutdown", "SUCCEEDED", "previous process is stopped", 0, 0); err != nil {
		return
	}
	if err = emit("install", "RUNNING", "switching to verified revision", 0, 0); err != nil {
		return
	}
	if st, e := os.Lstat(appPath); e == nil {
		if st.Mode()&os.ModeSymlink != 0 {
			result.PreviousRevision, _ = os.Readlink(appPath)
		} else if st.IsDir() {
			result.PreviousRevision = revision + "_previous"
			if err = os.Rename(appPath, result.PreviousRevision); err != nil {
				return
			}
		} else {
			return result, fmt.Errorf("application path is not a directory or revision link")
		}
	} else if !os.IsNotExist(e) {
		return result, e
	}
	relative, e := filepath.Rel(appDir, revision)
	if e != nil {
		return result, e
	}
	tempLink := filepath.Join(appDir, ".v2_"+spec.Id)
	if err = os.Symlink(relative, tempLink); err != nil {
		return
	}
	defer os.Remove(tempLink)
	if err = os.Rename(tempLink, appPath); err != nil {
		if result.PreviousRevision == revision+"_previous" {
			_ = os.Rename(result.PreviousRevision, appPath)
		}
		return
	}
	installed = true
	if err = emit("install", "SUCCEEDED", "revision installed: "+relative, 0, 0); err != nil {
		return
	}
	if err = emit("start", "RUNNING", "starting process", 0, 0); err != nil {
		return
	}
	pid, err = ExecuteProgram(env, proc)
	GetProcessMonitor().ProcessStop(spec.Process)
	if err != nil {
		return
	}
	if pid <= 1 {
		return result, fmt.Errorf("process start returned no valid PID")
	}
	for elapsed := int64(0); elapsed < 3; elapsed++ {
		if err = emit("start", "WAITING", fmt.Sprintf("checking pid %d remains alive (application readiness is not configured)", pid), elapsed, 3); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(time.Second):
		}
		if !runningV2(spec.Process, pid) {
			return result, fmt.Errorf("pid %d exited during startup", pid)
		}
	}
	if err = emit("start", "SUCCEEDED", fmt.Sprintf("pid %d is running; verify application health before continuing", pid), 3, 3); err != nil {
		return
	}
	dep := &Deployment{Process: spec.Process, extractPath: revision, revisionPath: revision}
	if e = createDeployHistory(env, dep); e != nil {
		if err = emit("history", "WARNING", e.Error(), 0, 0); err != nil {
			return
		}
	} else {
		// apply the same keep count/day rule as the legacy deploy path
		go stripDeployHistory(service.fatimaRuntime.GetConfig(), env, dep)
	}
	GetProcessMonitor().ResetICount(spec.Process)
	success = true
	return result, nil
}

func validateExecutableV2(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	var prefix [2]byte
	_, e = f.Read(prefix[:])
	f.Close()
	if e != nil {
		return e
	}
	if string(prefix[:]) == "#!" {
		return nil
	}
	switch runtime.GOOS {
	case "darwin":
		m, e := macho.Open(path)
		if e != nil {
			return fmt.Errorf("not a Darwin executable: %w", e)
		}
		defer m.Close()
		expected := macho.CpuArm64
		if runtime.GOARCH == "amd64" {
			expected = macho.CpuAmd64
		}
		if m.Cpu != expected {
			return fmt.Errorf("executable architecture mismatch")
		}
	case "linux":
		m, e := elf.Open(path)
		if e != nil {
			return fmt.Errorf("not a Linux executable: %w", e)
		}
		defer m.Close()
		expected := elf.EM_AARCH64
		if runtime.GOARCH == "amd64" {
			expected = elf.EM_X86_64
		}
		if m.Machine != expected {
			return fmt.Errorf("executable architecture mismatch")
		}
	default:
		return fmt.Errorf("unsupported host platform")
	}
	return nil
}
func (service *DomainService) checkRevisionV2(op *api.Operation) error {
	path := filepath.Join(service.fatimaRuntime.GetEnv().GetFolderGuide().GetFatimaHome(), builder.FatimaFolderApp, op.Process)
	actual, e := filepath.EvalSymlinks(path)
	if e != nil || actual != op.RevisionPath {
		return fmt.Errorf("installed revision changed after deployment")
	}
	b, e := os.ReadFile(filepath.Join(path, manifestName))
	if e != nil {
		return e
	}
	var m revisionManifest
	if e = json.Unmarshal(b, &m); e != nil {
		return e
	}
	if m.OperationID != op.Id || m.ArtifactSHA256 != op.Sha256 {
		return fmt.Errorf("installed artifact identity changed")
	}
	for name, expected := range m.Files {
		if filepath.IsAbs(name) || strings.HasPrefix(filepath.Clean(name), "..") {
			return fmt.Errorf("invalid manifest path")
		}
		sum, _, e := artifact.Digest(filepath.Join(path, name))
		if e != nil || sum != expected {
			return fmt.Errorf("installed file changed: %s", name)
		}
	}
	return nil
}
