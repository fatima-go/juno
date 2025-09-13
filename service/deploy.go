/*
 * Copyright 2023 github.com/fatima-go
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @project fatima-core
 * @author jin
 * @date 23. 4. 14. 오후 5:20
 */

package service

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatima-go/fatima-core"
	"github.com/fatima-go/fatima-core/builder"
	"github.com/fatima-go/fatima-core/lib"
	"github.com/fatima-go/fatima-log"
	"github.com/fatima-go/juno/domain"
)

const (
	ConstProcessTypeGeneral = "GENERAL"
	ConstProcessTypeUi      = "USER_INTERACTIVE"
	FilenameDeploymentJson  = "deployment.json"
)

type DeployRequest struct {
	filename  string `json:"file_name"`
	localpath string
	when      string `json:"when"`
}

func (d DeployRequest) removeLocalFile() {
	if len(d.localpath) > 0 {
		dir := filepath.Dir(d.localpath)
		os.Remove(d.localpath)
		os.Remove(dir)
	}
}

type Deployment struct {
	Process      string   `json:"process"`
	ProcessType  string   `json:"process_type, omitempty"` // GENERAL, USER_INTERACTIVE
	ExtraBin     []string `json:"extra_bin, omitempty"`
	extractPath  string
	revisionPath string
}

func (d Deployment) GetDeploymentJsonFilePath() string {
	return filepath.Join(d.extractPath, FilenameDeploymentJson)
}

func (d Deployment) IsGeneralProcessType() bool {
	if len(d.ProcessType) == 0 {
		return true
	}

	if strings.ToUpper(d.ProcessType) == ConstProcessTypeGeneral {
		return true
	}
	return false
}

func (service *DomainService) DeployPackage(mr *multipart.Reader) (string, error) {
	req, err := buildDeployRequest(service.fatimaRuntime.GetEnv(), mr)
	if err != nil {
		return "", err
	}

	defer req.removeLocalFile()

	var dep *Deployment
	dep, err = extractFarfile(req)
	if err != nil {
		return "", err
	}

	err = deployToPackage(service.fatimaRuntime.GetEnv(), dep)
	if err != nil {
		return "", err
	}

	runtime.GC()

	// create deploy history
	err = createDeployHistory(service.fatimaRuntime.GetEnv(), dep)
	if err != nil {
		log.Warn("createDeployHistory fail : %s", err.Error())
		return req.localpath, nil
	}

	go stripDeployHistory(service.fatimaRuntime.GetConfig(), service.fatimaRuntime.GetEnv(), dep)

	return req.localpath, nil
}

// createDeployHistory create deploy history into $FATIMA_HOME/data/juno/deployment/my_process_folder
func createDeployHistory(env fatima.FatimaEnv, dep *Deployment) error {
	processHistoryDir := buildHistorySaveDir(env, dep.Process)
	err := os.MkdirAll(processHistoryDir, 0755)
	if err != nil {
		return fmt.Errorf("fail to make dir %s : %s", processHistoryDir, err.Error())
	}

	// copy deployment.json
	// destination deployment file name MUST be numeric format
	destFileName := fmt.Sprintf("%d", lib.CurrentTimeMillis())
	destFile := filepath.Join(processHistoryDir, destFileName)
	err = copyFile(dep.GetDeploymentJsonFilePath(), destFile)
	if err != nil {
		return fmt.Errorf("fail to copy deployment(%s -> %s) : %s",
			dep.GetDeploymentJsonFilePath(), destFileName, err.Error())
	}
	log.Info("create deploy history : %s", destFile)
	return nil
}

func copyFile(srcFile, dstFile string) error {
	from, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer from.Close()
	stat, _ := from.Stat()
	to, err := os.OpenFile(dstFile, os.O_RDWR|os.O_CREATE, stat.Mode())
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}

	return nil
}

const (
	deploymentHistoryDataDir      = "deployment"
	propDeployHistoryKeepCount    = "deployment.history.keep.count"
	defaultDeployHistoryKeepCount = 10
	propDeployHistoryKeepDay      = "deployment.history.keep.day"
	defaultDeployHistoryKeepDay   = 180
)

// stripDeployHistory remove old history info
func stripDeployHistory(config fatima.Config, env fatima.FatimaEnv, dep *Deployment) {
	keepCount, keepDay := getDeployHistoryKeepConfig(config)

	log.Debug("keepCount : %d, keepDay : %d", keepCount, keepDay)

	processHistoryDir := buildHistorySaveDir(env, dep.Process)
	savedFileTimeMillisList, err := readFilesInDir(processHistoryDir)
	if err != nil {
		log.Warn("readFilesInDir error %s : %s", processHistoryDir, err.Error())
		return
	}

	log.Debug("saved deployment count=%d", len(savedFileTimeMillisList))

	if len(savedFileTimeMillisList) < keepCount {
		return
	}

	adjustTimemillis := int(time.Now().AddDate(0, 0, -keepDay).UnixMilli())
	log.Debug("adjustTimemillis : %d", adjustTimemillis)

	for i, fileTimeMillis := range savedFileTimeMillisList {
		log.Debug("check %d:%d", i, fileTimeMillis)

		if i < keepCount {
			continue
		}

		if fileTimeMillis < adjustTimemillis {
			delFilePath := fmt.Sprintf("%s/%d", processHistoryDir, fileTimeMillis)
			os.Remove(delFilePath)
			log.Info("remove %s", delFilePath)
		}
	}
}

type DeploymentTimeDescending []int

func (a DeploymentTimeDescending) Len() int           { return len(a) }
func (a DeploymentTimeDescending) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a DeploymentTimeDescending) Less(i, j int) bool { return a[j] < a[i] }

func buildHistorySaveDir(env fatima.FatimaEnv, proc string) string {
	return filepath.Join(env.GetFolderGuide().GetDataFolder(), deploymentHistoryDataDir, proc)
}

// getDeployHistoryKeepConfig get keepCount and keepDay property value
func getDeployHistoryKeepConfig(config fatima.Config) (int, int) {
	historyKeepCount := defaultDeployHistoryKeepCount
	historyKeepDay := defaultDeployHistoryKeepDay

	v, err := config.GetInt(propDeployHistoryKeepCount)
	if err == nil && v > 1 {
		historyKeepCount = v
	}

	v, err = config.GetInt(propDeployHistoryKeepDay)
	if err == nil && v > 0 {
		historyKeepDay = v
	}

	return historyKeepCount, historyKeepDay
}

func readFilesInDir(root string) ([]int, error) {
	log.Debug("readFilesInDir : %s", root)
	var files []int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Debug("readFilesInDir error : %s", err.Error())
			return nil
		}
		if !info.IsDir() {
			// deployment filename is consist of numeric value like int
			i := isNumeric(info.Name())
			if i > 0 {
				files = append(files, int(i))
			}
		}
		return nil
	})

	sort.Sort(DeploymentTimeDescending(files))
	return files, err
}

func isNumeric(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return -1
	}
	return i
}

func buildDeployRequest(env fatima.FatimaEnv, mr *multipart.Reader) (*DeployRequest, error) {
	r := DeployRequest{when: "now"}

	completeCount := 0
	tmpFile := env.GetFolderGuide().CreateTmpFilePath()
	log.Debug("tmp path : %s", tmpFile)

	for {
		p, err := mr.NextPart()
		defer p.Close()

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("fail to parse multipart data : %s", err.Error())
		}

		s := p.Header.Get("Content-Disposition")
		log.Debug("content disposition : %s", s)
		if len(s) == 0 {
			return nil, fmt.Errorf("invalid content-disposition value")
		}

		m := buildContentDispositionMap(s)
		name := m["name"]
		if len(name) == 0 {
			return nil, fmt.Errorf("invalid content-disposition name value")
		}

		slurp, err := io.ReadAll(p)
		name = cutQuatation(name)
		log.Trace("name value : %s", name)
		if name == domain.PACKAGE_DEPLOY_FAR {
			// form-data; name="far"; filename="example.far"
			err := os.WriteFile(filepath.Join(tmpFile), slurp, 0644)
			if err != nil {
				return nil, fmt.Errorf("fail to save file to local : %s", err.Error())
			}
			r.localpath = tmpFile
			completeCount = completeCount + 1
		} else if name == "json" {
			// form-data; name="json"; filename="json"
			var items map[string]string
			err = json.Unmarshal(slurp, &items)
			if err != nil {
				return nil, fmt.Errorf("fail to unmarshal json data : %s", err.Error())
			}

			completeCount = completeCount + 1
		}

		if completeCount >= 2 {
			break
		}
	}

	return &r, nil
}

// e.g) Content-Disposition: form-data; name="data"; filename="data"
func buildContentDispositionMap(source string) map[string]string {
	var ss []string

	ss = strings.Split(source, ";")
	m := make(map[string]string)
	for _, pair := range ss {
		z := strings.Split(pair, "=")
		if len(z) != 2 {
			continue
		}
		k := strings.Trim(z[0], " ")
		v := strings.Trim(z[1], " ")
		m[k] = cutQuatation(v)
	}

	return m
}

func cutQuatation(value string) string {
	if len(value) < 2 {
		return value
	}

	if value[0] == '\'' || value[0] == '"' {
		value = value[1:]
	}

	i := len(value)
	if i > 0 {
		if value[i-1] == '\'' || value[i-1] == '"' {
			value = value[:i-1]
		}
	}

	return value
}

func extractFarfile(far *DeployRequest) (*Deployment, error) {
	r, err := zip.OpenReader(far.localpath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	dest := filepath.Dir(far.localpath)
	dest = filepath.Join(dest, "extract")
	os.MkdirAll(dest, 0755)

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		//defer rc.Close()

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			//changeMode := f.Mode() | os.OS_USER_W
			// https://github.com/phayes/permbits/blob/master/permbits.go
			// http://stackoverflow.com/questions/28969455/golang-properly-instantiate-os-filemode
			os.MkdirAll(path, 0755)
		} else {
			//os.MkdirAll(filepath.Dir(path), f.Mode())
			mode := f.Mode()
			if strings.HasSuffix(f.Name, ".sh") {
				mode = 0744
			}
			fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
			if err != nil {
				rc.Close()
				return nil, err
			}

			_, err = io.Copy(fh, rc)
			if err != nil {
				rc.Close()
				fh.Close()
				return nil, err
			}
			fh.Close()
		}
		rc.Close()
	}

	log.Debug("extract tmp finished : %s", dest)

	file := filepath.Join(dest, FilenameDeploymentJson)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("fail to read deployment info : %s", err.Error())
	}

	dep := Deployment{}
	err = json.Unmarshal(data, &dep)
	if err != nil {
		return nil, fmt.Errorf("fail to parse deployment info : %s", err.Error())
	}

	dep.extractPath = dest
	return &dep, nil
}

func deployToPackage(env fatima.FatimaEnv, dep *Deployment) error {
	yamlConfig := builder.NewYamlFatimaPackageConfig(env)
	proc := yamlConfig.GetProcByName(dep.Process)

	if dep.IsGeneralProcessType() {
		if proc == nil {
			return fmt.Errorf("not found in process configuration : %s", dep.Process)
		}
	}

	// get next revision number
	revDir := filepath.Join(env.GetFolderGuide().GetFatimaHome(),
		builder.FatimaFolderApp,
		domain.FOLDER_APP_REVISION,
		dep.Process)

	e := os.MkdirAll(revDir, 0755)
	if e != nil {
		return fmt.Errorf("fail to make dir for revision : %s", e.Error())
	}

	dep.revisionPath = getNextRevision(revDir)
	e = os.Rename(dep.extractPath, dep.revisionPath)
	if e != nil {
		return fmt.Errorf("fail to move far to revision : %s", e.Error())
	}

	// adjust extractPath
	dep.extractPath = dep.revisionPath

	// stop process
	appName := dep.Process
	if dep.IsGeneralProcessType() {
		pid := GetPid(env, proc)
		if pid > 1 {
			if inspector.CheckProcessRunningByPid(proc.GetName(), pid) {
				log.Warn("executing goaway %s [%d]", proc.GetName(), pid)
				executeGoaway(env, proc, pid)
				KillProgram(proc.GetName(), pid)
				// wait for previous process finish gracefully shutdown
				time.Sleep(5 * time.Second)
			}
		}
		appName = proc.GetName()
	}

	// unlink / link
	e = linkRevision(env, appName, dep.revisionPath)
	if e != nil {
		log.Error("fail to link revision : %s", e.Error())
		return e
	}

	// start process
	if dep.IsGeneralProcessType() {
		startProcess(env, proc)
	} else {
		// remove all previous revision files
		removeAllPreviousRevisions()
	}

	return nil
}

func removeAllPreviousRevisions() {
	// TODO
}

func getNextRevision(path string) string {
	dirs, err := filepath.Glob(path + "/*_R[0-9]*")
	if err != nil {
		log.Info("fail to read directory list for revision : %s\n", err.Error())
		return ""
	}

	if len(dirs) == 0 {
		return filepath.Join(path,
			fmt.Sprintf("%s_R001", time.Now().Format("2006.01.02-15.04")))
	}

	max := 0
	for _, v := range dirs {
		idx := strings.LastIndex(v, "R")
		m, err := strconv.Atoi(v[idx+1:])
		if err != nil {
			continue
		}
		if m > max {
			max = m
		}
	}

	return filepath.Join(path,
		fmt.Sprintf("%s_R%03d", time.Now().Format("2006.01.02-15.04"), max+1))
}

func linkRevision(env fatima.FatimaEnv, proc string, revDir string) error {
	// unlink $FATIMA_HOME/app/example
	appDir := filepath.Join(env.GetFolderGuide().GetFatimaHome(), builder.FatimaFolderApp)
	appLink := filepath.Join(appDir, proc)
	log.Info("remove applink : %s", appLink)
	err := os.Remove(appLink)
	if err != nil {
		log.Warn("fail to remove applink : %s", err.Error())
	}

	// link revdir to $FATIMA_HOME/app/example
	relPath, e := filepath.Rel(appDir, revDir)
	if e != nil {
		return fmt.Errorf("fail to create relative link : %s", e.Error())
	}

	command := fmt.Sprintf("ln -s %s %s", relPath, proc)
	log.Info("exec : %s", command)

	var cmd *exec.Cmd
	s := regexp.MustCompile("\\s+").Split(command, -1)
	cmd = exec.Command(s[0], s[1:]...)
	cmd.Dir = appDir
	e = cmd.Run()
	if e != nil {
		log.Warn("fail to process link. command=[%s], err=[%s]", command, e.Error())
		if verifyAppLink(env, relPath, proc) {
			return nil
		}
		return fmt.Errorf("invalid app revision link")
	}

	return nil
}

func verifyAppLink(env fatima.FatimaEnv, relPath, proc string) bool {
	app := filepath.Join(env.GetFolderGuide().GetFatimaHome(), builder.FatimaFolderApp, proc)
	fi, err := os.Lstat(app)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		log.Warn("not symbolic link : %s", app)
		return false
	}

	linkPath, err := os.Readlink(app)
	if err != nil {
		log.Warn("fail to read link : %s", err.Error())
		return false
	}

	if linkPath != relPath {
		return false
	}

	return true
}

func unlinkApp(env fatima.FatimaEnv, proc string) {
	// unlink $FATIMA_HOME/app/example
	appDir := filepath.Join(env.GetFolderGuide().GetFatimaHome(), builder.FatimaFolderApp)
	appLink := filepath.Join(appDir, proc)
	os.Remove(appLink)
}
