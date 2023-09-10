package engine

import (
	"errors"
	"fmt"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/sock"
	"github.com/dagu-dev/dagu/internal/suspend"
	"github.com/dagu-dev/dagu/internal/utils"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type Engine interface {
	CreateDAG(name string) (string, error)
	Grep(pattern string) ([]*persistence.GrepResult, []string, error)
	Rename(oldDAGPath, newDAGPath string) error
	Stop(dag *dag.DAG) error
	StartAsync(dag *dag.DAG, params string)
	Start(dag *dag.DAG, params string) error
	Restart(dag *dag.DAG) error
	Retry(dag *dag.DAG, reqId string) error
	GetStatus(dag *dag.DAG) (*model.Status, error)
	GetStatusByRequestId(dag *dag.DAG, requestId string) (*model.Status, error)
	GetLastStatus(dag *dag.DAG) (*model.Status, error)
	GetRecentStatuses(dag *dag.DAG, n int) []*model.StatusFile
	UpdateStatus(dag *dag.DAG, status *model.Status) error
	UpdateDAGSpec(d *dag.DAG, spec string) error
	DeleteDAG(dag *dag.DAG) error
	ReadStatusAll(DAGsDir string) (statuses []*persistence.DAGStatus, errs []string, err error)
	ReadStatus(dagLocation string) (*persistence.DAGStatus, error)
}

type engineImpl struct {
	dataStoreFactory persistence.DataStoreFactory
	suspendChecker   *suspend.SuspendChecker
	executable       string
	workDir          string
}

var (
	_DAGTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)
)

// CreateDAG creates a new DAG.
func (e *engineImpl) CreateDAG(name string) (string, error) {
	ds := e.dataStoreFactory.NewDAGStore()
	id, err := ds.Create(name, _DAGTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to create DAG file: %s", err)
	}
	return id, nil
}

func (e *engineImpl) Grep(pattern string) ([]*persistence.GrepResult, []string, error) {
	ds := e.dataStoreFactory.NewDAGStore()
	return ds.Grep(pattern)
}

func (e *engineImpl) Rename(oldName, newName string) error {
	ds := e.dataStoreFactory.NewDAGStore()
	if err := ds.Rename(oldName, newName); err != nil {
		return fmt.Errorf("failed to rename DAG: %s", err)
	}
	hs := e.dataStoreFactory.NewHistoryStore()
	if err := hs.Rename(oldName, newName); err != nil {
		return fmt.Errorf("failed to rename DAG: %s", err)
	}
	return nil
}

func (e *engineImpl) Stop(dag *dag.DAG) error {
	// TODO: fix this not to connect to the DAG directly
	client := sock.Client{Addr: dag.SockAddr()}
	_, err := client.Request("POST", "/stop")
	return err
}

func (e *engineImpl) StartAsync(dag *dag.DAG, params string) {
	go func() {
		err := e.Start(dag, params)
		utils.LogErr("starting a DAG", err)
	}()
}

func (e *engineImpl) Start(dag *dag.DAG, params string) error {
	args := []string{"start"}
	if params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, utils.EscapeArg(params, false)))
	}
	args = append(args, dag.Location)
	cmd := exec.Command(e.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = e.workDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

// TODO: fix params
func (e *engineImpl) Restart(dag *dag.DAG) error {
	args := []string{"restart", dag.Location}
	cmd := exec.Command(e.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = e.workDir
	cmd.Env = os.Environ()
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

// TODO: fix params
func (e *engineImpl) Retry(dag *dag.DAG, reqId string) (err error) {
	go func() {
		args := []string{"retry"}
		args = append(args, fmt.Sprintf("--req=%s", reqId))
		args = append(args, dag.Location)
		cmd := exec.Command(e.executable, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
		cmd.Dir = e.workDir
		cmd.Env = os.Environ()
		defer func() {
			_ = cmd.Wait()
		}()
		err = cmd.Start()
		utils.LogErr("retry a DAG", err)
	}()
	time.Sleep(time.Millisecond * 500)
	return
}

func (e *engineImpl) GetStatus(dag *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: dag.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		} else {
			return model.NewStatusDefault(dag), nil
		}
	}
	return model.StatusFromJson(ret)
}

func (e *engineImpl) GetStatusByRequestId(dag *dag.DAG, requestId string) (*model.Status, error) {
	ret, err := e.dataStoreFactory.NewHistoryStore().FindByRequestId(dag.Location, requestId)
	if err != nil {
		return nil, err
	}
	status, _ := e.GetStatus(dag)
	if status != nil && status.RequestId != requestId {
		// if the request id is not matched then correct the status
		ret.Status.CorrectRunningStatus()
	}
	return ret.Status, err
}

func (e *engineImpl) getCurrentStatus(dag *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: dag.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %s", err)
	}
	return model.StatusFromJson(ret)
}

func (e *engineImpl) GetLastStatus(dag *dag.DAG) (*model.Status, error) {
	currStatus, _ := e.getCurrentStatus(dag)
	if currStatus != nil {
		return currStatus, nil
	}
	status, err := e.dataStoreFactory.NewHistoryStore().ReadStatusToday(dag.Location)
	if errors.Is(err, persistence.ErrNoStatusDataToday) || errors.Is(err, persistence.ErrNoStatusData) {
		return model.NewStatusDefault(dag), nil
	}
	if err != nil {
		return model.NewStatusDefault(dag), err
	}
	status.CorrectRunningStatus()
	return status, nil
}

func (e *engineImpl) GetRecentStatuses(dag *dag.DAG, n int) []*model.StatusFile {
	return e.dataStoreFactory.NewHistoryStore().ReadStatusHist(dag.Location, n)
}

func (e *engineImpl) UpdateStatus(dag *dag.DAG, status *model.Status) error {
	client := sock.Client{Addr: dag.SockAddr()}
	res, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return err
		}
	} else {
		ss, _ := model.StatusFromJson(res)
		if ss != nil && ss.RequestId == status.RequestId &&
			ss.Status == scheduler.SchedulerStatus_Running {
			return fmt.Errorf("the DAG is running")
		}
	}
	return e.dataStoreFactory.NewHistoryStore().Update(dag.Location, status.RequestId, status)
}

func (e *engineImpl) UpdateDAGSpec(d *dag.DAG, spec string) error {
	// validation
	cl := dag.Loader{}
	_, err := cl.LoadData([]byte(spec))
	if err != nil {
		return err
	}

	if !utils.FileExists(d.Location) {
		return fmt.Errorf("the config file %s does not exist", d.Location)
	}
	err = os.WriteFile(d.Location, []byte(spec), 0755)

	return err
}

func (e *engineImpl) DeleteDAG(dag *dag.DAG) error {
	err := e.dataStoreFactory.NewHistoryStore().RemoveAll(dag.Location)
	if err != nil {
		return err
	}
	return os.Remove(dag.Location)
}

func (e *engineImpl) ReadStatusAll(DAGsDir string) (statuses []*persistence.DAGStatus, errs []string, err error) {
	statuses = []*persistence.DAGStatus{}
	errs = []string{}
	if !utils.FileExists(DAGsDir) {
		if err = os.MkdirAll(DAGsDir, 0755); err != nil {
			errs = append(errs, err.Error())
			return
		}
	}
	fis, err := os.ReadDir(DAGsDir)
	utils.LogErr("read DAGs directory", err)
	for _, fi := range fis {
		if utils.MatchExtension(fi.Name(), dag.EXTENSIONS) {
			d, err := e.readStatus(filepath.Join(DAGsDir, fi.Name()), true)
			utils.LogErr("read DAG config", err)
			if d != nil {
				statuses = append(statuses, d)
			} else {
				errs = append(errs, fmt.Sprintf("reading %s failed: %s", fi.Name(), err))
			}
		}
	}
	return statuses, errs, nil
}

func (e *engineImpl) getDAG(name string, metadataOnly bool) (*dag.DAG, error) {
	ds := e.dataStoreFactory.NewDAGStore()
	if metadataOnly {
		return ds.GetMetadata(name)
	} else {
		return ds.GetDetails(name)
	}
}

func (e *engineImpl) ReadStatus(dagLocation string) (*persistence.DAGStatus, error) {
	return e.readStatus(dagLocation, false)
}

func (e *engineImpl) readStatus(dagLocation string, metadataOnly bool) (*persistence.DAGStatus, error) {
	d, err := e.getDAG(dagLocation, metadataOnly)
	if d == nil {
		d = &dag.DAG{Location: dagLocation}
	}
	if err != nil {
		return persistence.NewDAGStatus(d, model.NewStatusDefault(d), e.isSuspended(d), err), err
	}
	if !metadataOnly {
		_, err = scheduler.NewExecutionGraph(d.Steps...)
	}
	status, err := e.GetLastStatus(d)
	return persistence.NewDAGStatus(d, status, e.isSuspended(d), err), err
}

func (e *engineImpl) isSuspended(d *dag.DAG) bool {
	return e.suspendChecker.IsSuspended(d)
}
