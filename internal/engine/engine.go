package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/sock"
	"github.com/dagu-dev/dagu/internal/util"
)

type Engine interface {
	CreateDAG(name string) (string, error)
	GetDAGSpec(id string) (string, error)
	Grep(pattern string) ([]*persistence.GrepResult, []string, error)
	Rename(oldDAGPath, newDAGPath string) error
	Stop(d *dag.DAG) error
	StartAsync(d *dag.DAG, params string)
	Start(d *dag.DAG, params string) error
	Restart(d *dag.DAG) error
	Retry(d *dag.DAG, reqId string) error
	GetCurrentStatus(d *dag.DAG) (*model.Status, error)
	GetStatusByRequestId(d *dag.DAG, requestId string) (*model.Status, error)
	GetLatestStatus(d *dag.DAG) (*model.Status, error)
	GetRecentHistory(d *dag.DAG, n int) []*model.StatusFile
	UpdateStatus(d *dag.DAG, status *model.Status) error
	UpdateDAG(id string, spec string) error
	DeleteDAG(name, loc string) error
	GetAllStatus() (statuses []*persistence.DAGStatus, errs []string, err error)
	GetStatus(dagLocation string) (*persistence.DAGStatus, error)
	IsSuspended(id string) bool
	ToggleSuspend(id string, suspend bool) error
}

type engineImpl struct {
	dataStoreFactory persistence.DataStoreFactory
	executable       string
	workDir          string
}

var (
	_DAGTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)
)

var (
	errCreateDAGFile = errors.New("failed to create DAG file")
	errRenameDAG     = errors.New("failed to rename DAG")
	errGetStatus     = errors.New("failed to get status")
	errDAGIsRunning  = errors.New("the DAG is running")
)

func (e *engineImpl) GetDAGSpec(id string) (string, error) {
	ds := e.dataStoreFactory.NewDAGStore()
	return ds.GetSpec(id)
}

func (e *engineImpl) CreateDAG(name string) (string, error) {
	ds := e.dataStoreFactory.NewDAGStore()
	id, err := ds.Create(name, _DAGTemplate)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errCreateDAGFile, err)
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
		return fmt.Errorf("%w: %s", errRenameDAG, err)
	}
	hs := e.dataStoreFactory.NewHistoryStore()
	if err := hs.Rename(oldName, newName); err != nil {
		return fmt.Errorf("%w: %s", errRenameDAG, err)
	}
	return nil
}

func (e *engineImpl) Stop(d *dag.DAG) error {
	// TODO: fix this not to connect to the DAG directly
	client := sock.Client{Addr: d.SockAddr()}
	_, err := client.Request("POST", "/stop")
	return err
}

func (e *engineImpl) StartAsync(d *dag.DAG, params string) {
	go func() {
		err := e.Start(d, params)
		util.LogErr("starting a DAG", err)
	}()
}

func (e *engineImpl) Start(d *dag.DAG, params string) error {
	args := []string{"start"}
	if params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, escapeArg(params, false)))
	}
	args = append(args, d.Location)
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

func (e *engineImpl) Restart(d *dag.DAG) error {
	args := []string{"restart", d.Location}
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

func (e *engineImpl) Retry(d *dag.DAG, reqId string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--req=%s", reqId))
	args = append(args, d.Location)
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

func (e *engineImpl) GetCurrentStatus(d *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: d.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		}
		return model.NewStatusDefault(d), nil
	}
	return model.StatusFromJson(ret)
}

func (e *engineImpl) GetStatusByRequestId(d *dag.DAG, requestId string) (*model.Status, error) {
	ret, err := e.dataStoreFactory.NewHistoryStore().FindByRequestId(d.Location, requestId)
	if err != nil {
		return nil, err
	}
	status, _ := e.GetCurrentStatus(d)
	if status != nil && status.RequestId != requestId {
		// if the request id is not matched then correct the status
		ret.Status.CorrectRunningStatus()
	}
	return ret.Status, err
}

func (e *engineImpl) getCurrentStatus(d *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: d.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errGetStatus, err)
	}
	return model.StatusFromJson(ret)
}

func (e *engineImpl) GetLatestStatus(d *dag.DAG) (*model.Status, error) {
	currStatus, _ := e.getCurrentStatus(d)
	if currStatus != nil {
		return currStatus, nil
	}
	status, err := e.dataStoreFactory.NewHistoryStore().ReadStatusToday(d.Location)
	if errors.Is(err, persistence.ErrNoStatusDataToday) || errors.Is(err, persistence.ErrNoStatusData) {
		return model.NewStatusDefault(d), nil
	}
	if err != nil {
		return model.NewStatusDefault(d), err
	}
	status.CorrectRunningStatus()
	return status, nil
}

func (e *engineImpl) GetRecentHistory(d *dag.DAG, n int) []*model.StatusFile {
	return e.dataStoreFactory.NewHistoryStore().ReadStatusRecent(d.Location, n)
}

func (e *engineImpl) UpdateStatus(d *dag.DAG, status *model.Status) error {
	client := sock.Client{Addr: d.SockAddr()}
	res, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return err
		}
	} else {
		ss, _ := model.StatusFromJson(res)
		if ss != nil && ss.RequestId == status.RequestId &&
			ss.Status == scheduler.StatusRunning {
			return errDAGIsRunning
		}
	}
	return e.dataStoreFactory.NewHistoryStore().Update(d.Location, status.RequestId, status)
}

func (e *engineImpl) UpdateDAG(id string, spec string) error {
	ds := e.dataStoreFactory.NewDAGStore()
	return ds.UpdateSpec(id, []byte(spec))
}

func (e *engineImpl) DeleteDAG(name, loc string) error {
	err := e.dataStoreFactory.NewHistoryStore().RemoveAll(loc)
	if err != nil {
		return err
	}
	ds := e.dataStoreFactory.NewDAGStore()
	return ds.Delete(name)
}

func (e *engineImpl) GetAllStatus() (statuses []*persistence.DAGStatus, errs []string, err error) {
	ds := e.dataStoreFactory.NewDAGStore()
	dags, errs, err := ds.List()

	var ret []*persistence.DAGStatus
	for _, d := range dags {
		status, err := e.readStatus(d)
		if err != nil {
			errs = append(errs, err.Error())
		}
		ret = append(ret, status)
	}

	return ret, errs, err
}

func (e *engineImpl) getDAG(name string, metadataOnly bool) (*dag.DAG, error) {
	ds := e.dataStoreFactory.NewDAGStore()
	if metadataOnly {
		d, err := ds.GetMetadata(name)
		return e.emptyDAGIfNil(d, name), err
	}
	d, err := ds.GetDetails(name)
	return e.emptyDAGIfNil(d, name), err
}

func (e *engineImpl) GetStatus(id string) (*persistence.DAGStatus, error) {
	d, err := e.getDAG(id, false)
	if d == nil {
		// TODO: fix not to use location
		d = &dag.DAG{Name: id, Location: id}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(d.Steps...)
	}
	status, _ := e.GetLatestStatus(d)
	return persistence.NewDAGStatus(d, status, e.IsSuspended(d.Name), err), err
}

func (e *engineImpl) ToggleSuspend(id string, suspend bool) error {
	fs := e.dataStoreFactory.NewFlagStore()
	return fs.ToggleSuspend(id, suspend)
}

func (e *engineImpl) readStatus(d *dag.DAG) (*persistence.DAGStatus, error) {
	status, err := e.GetLatestStatus(d)
	return persistence.NewDAGStatus(d, status, e.IsSuspended(d.Name), err), err
}

func (e *engineImpl) emptyDAGIfNil(d *dag.DAG, dagLocation string) *dag.DAG {
	if d != nil {
		return d
	}
	return &dag.DAG{Location: dagLocation}
}

func (e *engineImpl) IsSuspended(id string) bool {
	fs := e.dataStoreFactory.NewFlagStore()
	return fs.IsSuspended(id)
}

func escapeArg(input string, doubleQuotes bool) string {
	escaped := strings.Builder{}

	for _, char := range input {
		if char == '\r' {
			escaped.WriteString("\\r")
		} else if char == '\n' {
			escaped.WriteString("\\n")
		} else if char == '"' && doubleQuotes {
			escaped.WriteString("\\\"")
		} else {
			escaped.WriteRune(char)
		}
	}

	return escaped.String()
}
