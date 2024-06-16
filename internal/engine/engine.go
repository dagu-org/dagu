package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/dagu-dev/dagu/internal/config"
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
	Stop(dg *dag.DAG) error
	StartAsync(dg *dag.DAG, params string)
	Start(dg *dag.DAG, params string) error
	Restart(dg *dag.DAG) error
	Retry(dg *dag.DAG, reqID string) error
	GetCurrentStatus(dg *dag.DAG) (*model.Status, error)
	GetStatusByRequestID(dg *dag.DAG, reqID string) (*model.Status, error)
	GetLatestStatus(dg *dag.DAG) (*model.Status, error)
	GetRecentHistory(dg *dag.DAG, n int) []*model.StatusFile
	UpdateStatus(dg *dag.DAG, status *model.Status) error
	UpdateDAG(id string, spec string) error
	DeleteDAG(name, loc string) error
	GetAllStatus() (statuses []*persistence.DAGStatus, errs []string, err error)
	GetStatus(dagLocation string) (*persistence.DAGStatus, error)
	IsSuspended(id string) bool
	ToggleSuspend(id string, suspend bool) error
	Config() *config.Config
}

// Config is the configuration for engine instance.
// The WorkDir is optional and specifies the working directory where the engine will operate.
type Config struct{ WorkDir string }

// DefaultConfig returns the default configuration for the engine.
func DefaultConfig() *Config {
	return &Config{}
}

// New creates a new Engine instance.
// The Engine is used to interact with the DAG execution engine.
func New(dataStore persistence.DataStoreFactory, cfg *Config, globalCfg *config.Config) Engine {
	return &engineImpl{
		dataStore:  dataStore,
		executable: globalCfg.Executable,
		workDir:    cfg.WorkDir,
		config:     globalCfg,
	}
}

type engineImpl struct {
	dataStore  persistence.DataStoreFactory
	executable string
	workDir    string
	config     *config.Config
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

func (e *engineImpl) Config() *config.Config {
	return e.config
}

func (e *engineImpl) GetDAGSpec(id string) (string, error) {
	dagStore := e.dataStore.NewDAGStore()
	return dagStore.GetSpec(id)
}

func (e *engineImpl) CreateDAG(name string) (string, error) {
	dagStore := e.dataStore.NewDAGStore()
	id, err := dagStore.Create(name, _DAGTemplate)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errCreateDAGFile, err)
	}
	return id, nil
}

func (e *engineImpl) Grep(pattern string) ([]*persistence.GrepResult, []string, error) {
	dagStore := e.dataStore.NewDAGStore()
	return dagStore.Grep(pattern)
}

func (e *engineImpl) Rename(oldName, newName string) error {
	dagStore := e.dataStore.NewDAGStore()
	if err := dagStore.Rename(oldName, newName); err != nil {
		return fmt.Errorf("%w: %s", errRenameDAG, err)
	}
	historyStore := e.dataStore.NewHistoryStore()
	if err := historyStore.Rename(oldName, newName); err != nil {
		return fmt.Errorf("%w: %s", errRenameDAG, err)
	}
	return nil
}

func (e *engineImpl) Stop(dg *dag.DAG) error {
	// TODO: fix this not to connect to the DAG directly
	client := sock.Client{Addr: dg.SockAddr()}
	_, err := client.Request("POST", "/stop")
	return err
}

func (e *engineImpl) StartAsync(dg *dag.DAG, params string) {
	go func() {
		err := e.Start(dg, params)
		util.LogErr("starting a DAG", err)
	}()
}

func (e *engineImpl) Start(dg *dag.DAG, params string) error {
	args := []string{"start"}
	if params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, escapeArg(params, false)))
	}
	args = append(args, dg.Location)
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

func (e *engineImpl) Restart(dg *dag.DAG) error {
	args := []string{"restart", dg.Location}
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

func (e *engineImpl) Retry(dg *dag.DAG, reqID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--req=%s", reqID))
	args = append(args, dg.Location)
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

func (e *engineImpl) GetCurrentStatus(dg *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: dg.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		}
		return model.NewStatusDefault(dg), nil
	}
	return model.StatusFromJson(ret)
}

func (e *engineImpl) GetStatusByRequestID(dg *dag.DAG, reqID string) (*model.Status, error) {
	ret, err := e.dataStore.NewHistoryStore().FindByRequestID(dg.Location, reqID)
	if err != nil {
		return nil, err
	}
	status, _ := e.GetCurrentStatus(dg)
	if status != nil && status.RequestID != reqID {
		// if the request id is not matched then correct the status
		ret.Status.CorrectRunningStatus()
	}
	return ret.Status, err
}

func (e *engineImpl) getCurrentStatus(dg *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: dg.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errGetStatus, err)
	}
	return model.StatusFromJson(ret)
}

func (e *engineImpl) GetLatestStatus(dg *dag.DAG) (*model.Status, error) {
	currStatus, _ := e.getCurrentStatus(dg)
	if currStatus != nil {
		return currStatus, nil
	}
	status, err := e.dataStore.NewHistoryStore().ReadStatusToday(dg.Location)
	if errors.Is(err, persistence.ErrNoStatusDataToday) || errors.Is(err, persistence.ErrNoStatusData) {
		return model.NewStatusDefault(dg), nil
	}
	if err != nil {
		return model.NewStatusDefault(dg), err
	}
	status.CorrectRunningStatus()
	return status, nil
}

func (e *engineImpl) GetRecentHistory(dg *dag.DAG, n int) []*model.StatusFile {
	return e.dataStore.NewHistoryStore().ReadStatusRecent(dg.Location, n)
}

func (e *engineImpl) UpdateStatus(dg *dag.DAG, status *model.Status) error {
	client := sock.Client{Addr: dg.SockAddr()}
	res, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return err
		}
	} else {
		unmarshalled, _ := model.StatusFromJson(res)
		if unmarshalled != nil && unmarshalled.RequestID == status.RequestID &&
			unmarshalled.Status == scheduler.StatusRunning {
			return errDAGIsRunning
		}
	}
	return e.dataStore.NewHistoryStore().Update(dg.Location, status.RequestID, status)
}

func (e *engineImpl) UpdateDAG(id string, spec string) error {
	dagStore := e.dataStore.NewDAGStore()
	return dagStore.UpdateSpec(id, []byte(spec))
}

func (e *engineImpl) DeleteDAG(name, loc string) error {
	err := e.dataStore.NewHistoryStore().RemoveAll(loc)
	if err != nil {
		return err
	}
	dagStore := e.dataStore.NewDAGStore()
	return dagStore.Delete(name)
}

func (e *engineImpl) GetAllStatus() (statuses []*persistence.DAGStatus, errs []string, err error) {
	dagStore := e.dataStore.NewDAGStore()
	dags, errs, err := dagStore.List()

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
	dagStore := e.dataStore.NewDAGStore()
	if metadataOnly {
		dg, err := dagStore.GetMetadata(name)
		return e.emptyDAGIfNil(dg, name), err
	}
	dagDetail, err := dagStore.GetDetails(name)
	return e.emptyDAGIfNil(dagDetail, name), err
}

func (e *engineImpl) GetStatus(id string) (*persistence.DAGStatus, error) {
	dg, err := e.getDAG(id, false)
	if dg == nil {
		// TODO: fix not to use location
		dg = &dag.DAG{Name: id, Location: id}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(dg.Steps...)
	}
	latestStatus, _ := e.GetLatestStatus(dg)
	return persistence.NewDAGStatus(dg, latestStatus, e.IsSuspended(dg.Name), err), err
}

func (e *engineImpl) ToggleSuspend(id string, suspend bool) error {
	flagStore := e.dataStore.NewFlagStore()
	return flagStore.ToggleSuspend(id, suspend)
}

func (e *engineImpl) readStatus(dg *dag.DAG) (*persistence.DAGStatus, error) {
	latestStatus, err := e.GetLatestStatus(dg)
	return persistence.NewDAGStatus(dg, latestStatus, e.IsSuspended(dg.Name), err), err
}

func (e *engineImpl) emptyDAGIfNil(dg *dag.DAG, dagLocation string) *dag.DAG {
	if dg != nil {
		return dg
	}
	return &dag.DAG{Location: dagLocation}
}

func (e *engineImpl) IsSuspended(id string) bool {
	flagStore := e.dataStore.NewFlagStore()
	return flagStore.IsSuspended(id)
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
