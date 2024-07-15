package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/dag/scheduler"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/sock"
	"github.com/dagu-dev/dagu/internal/util"
)

type NewEngineArgs struct {
	DataStore  persistence.DataStores
	Executable string
	WorkDir    string
}

// New creates a new Engine instance.
// The Engine is used to interact with the DAG execution engine.
func New(args *NewEngineArgs) Engine {
	return &engineImpl{
		dataStore:  args.DataStore,
		executable: args.Executable,
		workDir:    args.WorkDir,
	}
}

type engineImpl struct {
	dataStore  persistence.DataStores
	executable string
	workDir    string
}

var (
	dagTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)
)

var (
	errCreateDAGFile = errors.New("failed to create DAG file")
	errGetStatus     = errors.New("failed to get status")
	errDAGIsRunning  = errors.New("the DAG is running")
)

func (e *engineImpl) GetDAGSpec(id string) (string, error) {
	dagStore := e.dataStore.DAGStore()
	return dagStore.GetSpec(id)
}

func (e *engineImpl) CreateDAG(name string) (string, error) {
	dagStore := e.dataStore.DAGStore()
	id, err := dagStore.Create(name, dagTemplate)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errCreateDAGFile, err)
	}
	return id, nil
}

func (e *engineImpl) Grep(pattern string) (
	[]*persistence.GrepResult, []string, error,
) {
	dagStore := e.dataStore.DAGStore()
	return dagStore.Grep(pattern)
}

func (e *engineImpl) Rename(oldID, newID string) error {
	dagStore := e.dataStore.DAGStore()
	if err := dagStore.Rename(oldID, newID); err != nil {
		return err
	}
	historyStore := e.dataStore.HistoryStore()
	return historyStore.Rename(oldID, newID)
}

func (*engineImpl) Stop(dg *dag.DAG) error {
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
		args = append(args, fmt.Sprintf(`"%s"`, escapeArg(params)))
	}
	args = append(args, dg.Location)
	// nolint:gosec
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
	// nolint:gosec
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
	// nolint:gosec
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

func (*engineImpl) GetCurrentStatus(dg *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: dg.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		}
		return model.NewStatusDefault(dg), nil
	}
	return model.StatusFromJSON(ret)
}

func (e *engineImpl) GetStatusByRequestID(dg *dag.DAG, reqID string) (
	*model.Status, error,
) {
	ret, err := e.dataStore.HistoryStore().FindByRequestID(
		dg.Location, reqID,
	)
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

func (*engineImpl) currentStatus(dg *dag.DAG) (*model.Status, error) {
	client := sock.Client{Addr: dg.SockAddr()}
	ret, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errGetStatus, err)
	}
	return model.StatusFromJSON(ret)
}

func (e *engineImpl) GetLatestStatus(dg *dag.DAG) (*model.Status, error) {
	currStatus, _ := e.currentStatus(dg)
	if currStatus != nil {
		return currStatus, nil
	}
	status, err := e.dataStore.HistoryStore().ReadStatusToday(dg.Location)
	if errors.Is(err, persistence.ErrNoStatusDataToday) ||
		errors.Is(err, persistence.ErrNoStatusData) {
		return model.NewStatusDefault(dg), nil
	}
	if err != nil {
		return model.NewStatusDefault(dg), err
	}
	status.CorrectRunningStatus()
	return status, nil
}

func (e *engineImpl) GetRecentHistory(dg *dag.DAG, n int) []*model.StatusFile {
	return e.dataStore.HistoryStore().ReadStatusRecent(dg.Location, n)
}

func (e *engineImpl) UpdateStatus(dg *dag.DAG, status *model.Status) error {
	client := sock.Client{Addr: dg.SockAddr()}
	res, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return err
		}
	} else {
		unmarshalled, _ := model.StatusFromJSON(res)
		if unmarshalled != nil && unmarshalled.RequestID == status.RequestID &&
			unmarshalled.Status == scheduler.StatusRunning {
			return errDAGIsRunning
		}
	}
	return e.dataStore.HistoryStore().Update(
		dg.Location, status.RequestID, status,
	)
}

func (e *engineImpl) UpdateDAG(id string, spec string) error {
	dagStore := e.dataStore.DAGStore()
	return dagStore.UpdateSpec(id, []byte(spec))
}

func (e *engineImpl) DeleteDAG(name, loc string) error {
	err := e.dataStore.HistoryStore().RemoveAll(loc)
	if err != nil {
		return err
	}
	dagStore := e.dataStore.DAGStore()
	return dagStore.Delete(name)
}

func (e *engineImpl) GetAllStatus() (
	statuses []*DAGStatus, errs []string, err error,
) {
	dagStore := e.dataStore.DAGStore()
	dags, errs, err := dagStore.List()

	var ret []*DAGStatus
	for _, d := range dags {
		status, err := e.readStatus(d)
		if err != nil {
			errs = append(errs, err.Error())
		}
		ret = append(ret, status)
	}

	return ret, errs, err
}

func (e *engineImpl) getDAG(name string) (*dag.DAG, error) {
	dagStore := e.dataStore.DAGStore()
	dagDetail, err := dagStore.GetDetails(name)
	return e.emptyDAGIfNil(dagDetail, name), err
}

func (e *engineImpl) GetStatus(id string) (*DAGStatus, error) {
	dg, err := e.getDAG(id)
	if dg == nil {
		// TODO: fix not to use location
		dg = &dag.DAG{Name: id, Location: id}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(dg.Steps...)
	}
	latestStatus, _ := e.GetLatestStatus(dg)
	return newDAGStatus(
		dg, latestStatus, e.IsSuspended(dg.Name), err,
	), err
}

func (e *engineImpl) ToggleSuspend(id string, suspend bool) error {
	flagStore := e.dataStore.FlagStore()
	return flagStore.ToggleSuspend(id, suspend)
}

func (e *engineImpl) readStatus(dg *dag.DAG) (*DAGStatus, error) {
	latestStatus, err := e.GetLatestStatus(dg)
	return newDAGStatus(
		dg, latestStatus, e.IsSuspended(dg.Name), err,
	), err
}

func (*engineImpl) emptyDAGIfNil(dg *dag.DAG, dagLocation string) *dag.DAG {
	if dg != nil {
		return dg
	}
	return &dag.DAG{Location: dagLocation}
}

func (e *engineImpl) IsSuspended(id string) bool {
	flagStore := e.dataStore.FlagStore()
	return flagStore.IsSuspended(id)
}

func escapeArg(input string) string {
	escaped := strings.Builder{}

	for _, char := range input {
		if char == '\r' {
			_, _ = escaped.WriteString("\\r")
		} else if char == '\n' {
			_, _ = escaped.WriteString("\\n")
		} else {
			_, _ = escaped.WriteRune(char)
		}
	}

	return escaped.String()
}
