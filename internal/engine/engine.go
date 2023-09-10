package engine

import (
	"errors"
	"fmt"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/sock"
	"github.com/dagu-dev/dagu/internal/storage"
	"github.com/dagu-dev/dagu/internal/suspend"
	"github.com/dagu-dev/dagu/internal/utils"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"
)

type Engine interface {
	// CreateDAG creates a new DAG and returns the ID of the DAG.
	CreateDAG(name string) (string, error)
	GrepDAG(pattern string) ([]*persistence.GrepResult, []string, error)
	MoveDAG(oldDAGPath, newDAGPath string) error
	Stop(dag *dag.DAG) error
	// TODO: fix params
	StartAsync(dag *dag.DAG, binPath string, workDir string, params string)
	// TODO: fix params
	Start(dag *dag.DAG, binPath string, workDir string, params string) error
	// TODO: fix params
	Restart(dag *dag.DAG, bin string, workDir string) error
	// TODO: fix params
	Retry(dag *dag.DAG, binPath string, workDir string, reqId string) error
	GetStatus(dag *dag.DAG) (*model.Status, error)
	GetStatusByRequestId(dag *dag.DAG, requestId string) (*model.Status, error)
	GetLastStatus(dag *dag.DAG) (*model.Status, error)
	GetRecentStatuses(dag *dag.DAG, n int) []*model.StatusFile
	UpdateStatus(dag *dag.DAG, status *model.Status) error
	UpdateDAGSpec(d *dag.DAG, spec string) error
	DeleteDAG(dag *dag.DAG) error
	ReadAllStatus(DAGsDir string) (statuses []*DAGStatus, errs []string, err error)
	ReadStatus(dagLocation string, loadMetadataOnly bool) (*DAGStatus, error)
}

type engineImpl struct {
	dataStoreFactory persistence.DataStoreFactory
	// TODO: fix this to inject
	suspendChecker *suspend.SuspendChecker
}

func New(ds persistence.DataStoreFactory) Engine {
	return &engineImpl{
		dataStoreFactory: ds,
		// TODO: fix this to inject
		suspendChecker: suspend.NewSuspendChecker(
			storage.NewStorage(config.Get().SuspendFlagsDir),
		),
	}
}

// TODO: this should not be here.
// DAGStatus is the struct to contain DAGStatus spec and status.
type DAGStatus struct {
	File      string
	Dir       string
	DAG       *dag.DAG
	Status    *model.Status
	Suspended bool
	Error     error
	ErrorT    *string
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

// GrepDAG returns all DAGs that contain the given string.
func (e *engineImpl) GrepDAG(pattern string) ([]*persistence.GrepResult, []string, error) {
	ds := e.dataStoreFactory.NewDAGStore()
	return ds.Grep(pattern)
}

func (e *engineImpl) MoveDAG(oldDAGPath, newDAGPath string) error {
	if err := validateLocation(newDAGPath); err != nil {
		return err
	}
	if err := os.Rename(oldDAGPath, newDAGPath); err != nil {
		return err
	}
	// TODO: fix this to use DAG Manager not History Store
	return e.dataStoreFactory.NewHistoryStore().Rename(oldDAGPath, newDAGPath)
}

func validateLocation(dagLocation string) error {
	if path.Ext(dagLocation) != ".yaml" {
		return fmt.Errorf("the config file must be a yaml file with .yaml extension")
	}
	return nil
}

func (e *engineImpl) Stop(dag *dag.DAG) error {
	// TODO: fix this not to connect to the DAG directly
	client := sock.Client{Addr: dag.SockAddr()}
	_, err := client.Request("POST", "/stop")
	return err
}

// TODO: fix params
func (e *engineImpl) StartAsync(dag *dag.DAG, binPath string, workDir string, params string) {
	go func() {
		err := e.Start(dag, binPath, workDir, params)
		utils.LogErr("starting a DAG", err)
	}()
}

// TODO: fix params
func (e *engineImpl) Start(dag *dag.DAG, binPath string, workDir string, params string) error {
	args := []string{"start"}
	if params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, utils.EscapeArg(params, false)))
	}
	args = append(args, dag.Location)
	cmd := exec.Command(binPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = workDir
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
func (e *engineImpl) Restart(dag *dag.DAG, bin string, workDir string) error {
	args := []string{"restart", dag.Location}
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	err := cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}

// TODO: fix params
func (e *engineImpl) Retry(dag *dag.DAG, binPath string, workDir string, reqId string) (err error) {
	go func() {
		args := []string{"retry"}
		args = append(args, fmt.Sprintf("--req=%s", reqId))
		args = append(args, dag.Location)
		cmd := exec.Command(binPath, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
		cmd.Dir = workDir
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
			return defaultStatus(dag), nil
		}
	}
	return model.StatusFromJson(ret)
}

func defaultStatus(d *dag.DAG) *model.Status {
	return model.NewStatus(d, nil, scheduler.SchedulerStatus_None, int(model.PidNotRunning), nil, nil)
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

func (e *engineImpl) GetLastStatus(dag *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: dag.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err == nil {
		return model.StatusFromJson(ret)
	}

	if err == nil || !errors.Is(err, sock.ErrTimeout) {
		status, err := e.dataStoreFactory.NewHistoryStore().ReadStatusToday(dag.Location)
		if err != nil {
			var readErr error = nil
			if !errors.Is(err, persistence.ErrNoStatusDataToday) && !errors.Is(err, persistence.ErrNoStatusData) {
				fmt.Printf("read status failed : %s", err)
				readErr = err
			}
			return defaultStatus(dag), readErr
		}
		// it is wrong status if the status is running
		status.CorrectRunningStatus()
		return status, nil
	}
	return nil, err
}

func (e *engineImpl) GetRecentStatuses(dag *dag.DAG, n int) []*model.StatusFile {
	ret := e.dataStoreFactory.NewHistoryStore().ReadStatusHist(dag.Location, n)
	return ret
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

// ReadAllStatus reads all DAGStatus
func (e *engineImpl) ReadAllStatus(DAGsDir string) (statuses []*DAGStatus, errs []string, err error) {
	statuses = []*DAGStatus{}
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
			d, err := e.ReadStatus(filepath.Join(DAGsDir, fi.Name()), true)
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

// ReadStatus loads DAG from config file.
func (e *engineImpl) ReadStatus(dagLocation string, loadMetadataOnly bool) (*DAGStatus, error) {
	var (
		cl  = dag.Loader{}
		d   *dag.DAG
		err error
	)

	if loadMetadataOnly {
		d, err = cl.LoadMetadataOnly(dagLocation)
	} else {
		d, err = cl.LoadWithoutEval(dagLocation)
	}

	if err != nil {
		if d != nil {
			return e.newDAGStatus(d, defaultStatus(d), err), err
		}
		d := &dag.DAG{Location: dagLocation}
		return e.newDAGStatus(d, defaultStatus(d), err), err
	}

	if !loadMetadataOnly {
		if _, err := scheduler.NewExecutionGraph(d.Steps...); err != nil {
			return e.newDAGStatus(d, nil, err), err
		}
	}

	status, err := e.GetLastStatus(d)

	return e.newDAGStatus(d, status, err), err
}

func (e *engineImpl) newDAGStatus(d *dag.DAG, s *model.Status, err error) *DAGStatus {
	ret := &DAGStatus{
		File:      filepath.Base(d.Location),
		Dir:       filepath.Dir(d.Location),
		DAG:       d,
		Status:    s,
		Suspended: e.suspendChecker.IsSuspended(d),
		Error:     err,
	}
	if err != nil {
		errT := err.Error()
		ret.ErrorT = &errT
	}
	return ret
}
