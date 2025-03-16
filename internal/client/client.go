package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations/dags"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/sock"
)

// New creates a new Client instance.
// The Client is used to interact with the DAG.
func New(
	dagStore persistence.DAGStore,
	historyStore persistence.HistoryStore,
	flagStore persistence.FlagStore,
	executable string,
	workDir string,
) Client {
	return &client{
		dagStore:     dagStore,
		historyStore: historyStore,
		flagStore:    flagStore,
		executable:   executable,
		workDir:      workDir,
	}
}

var _ Client = (*client)(nil)

type client struct {
	dagStore     persistence.DAGStore
	historyStore persistence.HistoryStore
	flagStore    persistence.FlagStore
	executable   string
	workDir      string
}

var (
	dagTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)
)

func (e *client) GetDAGSpec(ctx context.Context, id string) (string, error) {
	return e.dagStore.GetSpec(ctx, id)
}

func (e *client) CreateDAG(ctx context.Context, name string) (string, error) {
	id, err := e.dagStore.Create(ctx, name, dagTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to create DAG: %w", err)
	}
	return id, nil
}

func (e *client) Grep(ctx context.Context, pattern string) (
	[]*persistence.GrepResult, []string, error,
) {
	return e.dagStore.Grep(ctx, pattern)
}

func (e *client) Rename(ctx context.Context, oldID, newID string) error {
	oldDAG, err := e.dagStore.GetMetadata(ctx, oldID)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", oldID, err)
	}
	if err := e.dagStore.Rename(ctx, oldID, newID); err != nil {
		return err
	}
	newDAG, err := e.dagStore.GetMetadata(ctx, newID)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", newID, err)
	}
	if err := e.historyStore.Rename(ctx, oldDAG.Name, newDAG.Name); err != nil {
		return fmt.Errorf("failed to rename history for %s: %w", oldID, err)
	}
	return nil
}

func (e *client) Stop(ctx context.Context, dag *digraph.DAG) error {
	logger.Info(ctx, "Stopping", "name", dag.Name)
	addr := dag.SockAddr()
	if !fileutil.FileExists(addr) {
		logger.Info(ctx, "The DAG is not running", "name", dag.Name)
		return nil
	}
	client := sock.NewClient(addr)
	_, err := client.Request("POST", "/stop")
	return err
}

func (e *client) StartAsync(ctx context.Context, dag *digraph.DAG, opts StartOptions) {
	go func() {
		if err := e.Start(ctx, dag, opts); err != nil {
			logger.Error(ctx, "DAG start operation failed", "err", err)
		}
	}()
}

func (e *client) Start(_ context.Context, dag *digraph.DAG, opts StartOptions) error {
	args := []string{"start"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, escapeArg(opts.Params)))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	args = append(args, dag.Location)
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

func (e *client) Restart(_ context.Context, dag *digraph.DAG, opts RestartOptions) error {
	args := []string{"restart"}
	if opts.Quiet {
		args = append(args, "-q")
	}
	args = append(args, dag.Location)
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

func (e *client) Retry(_ context.Context, dag *digraph.DAG, requestID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--request-id=%s", requestID))
	args = append(args, dag.Location)
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

func (*client) GetCurrentStatus(_ context.Context, dag *digraph.DAG) (*persistence.Status, error) {
	client := sock.NewClient(dag.SockAddr())
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		}
		// The DAG is not running so return the default status
		status := persistence.NewStatusFactory(dag).CreateDefault()
		return &status, nil
	}
	return persistence.StatusFromJSON(ret)
}

func (e *client) GetStatusByRequestID(ctx context.Context, dag *digraph.DAG, requestID string) (
	*persistence.Status, error,
) {
	record, err := e.historyStore.FindByRequestID(ctx, dag.Name, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by request id: %w", err)
	}
	historyStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the DAG is running, set the status to error if the request ID does not match
	// Because the DAG execution must be stopped
	// TODO: Handle different request IDs for the same DAG
	status, _ := e.GetCurrentStatus(ctx, dag)
	if status != nil && status.RequestID != requestID {
		historyStatus.SetStatusToErrorIfRunning()
	}

	return historyStatus, err
}

func (*client) currentStatus(_ context.Context, dag *digraph.DAG) (*persistence.Status, error) {
	client := sock.NewClient(dag.SockAddr())
	statusJSON, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	return persistence.StatusFromJSON(statusJSON)
}

func (e *client) GetLatestStatus(ctx context.Context, dag *digraph.DAG) (persistence.Status, error) {
	currStatus, _ := e.currentStatus(ctx, dag)
	if currStatus != nil {
		return *currStatus, nil
	}

	var latestStatus *persistence.Status

	record, err := e.historyStore.Latest(ctx, dag.Name)
	if err != nil {
		goto handleError
	}

	latestStatus, err = record.ReadStatus(ctx)
	if err != nil {
		goto handleError
	}

	latestStatus.SetStatusToErrorIfRunning()
	return *latestStatus, nil

handleError:

	if errors.Is(err, persistence.ErrNoStatusData) {
		// No status for today
		return persistence.NewStatusFactory(dag).CreateDefault(), nil
	}

	return persistence.NewStatusFactory(dag).CreateDefault(), err
}

func (e *client) GetRecentHistory(ctx context.Context, dag *digraph.DAG, n int) []persistence.StatusFile {
	records := e.historyStore.Recent(ctx, dag.Name, n)

	var ret []persistence.StatusFile
	for _, record := range records {
		if statusFile, err := record.Read(ctx); err == nil {
			ret = append(ret, *statusFile)
		}
	}

	return ret
}

var errDAGIsRunning = errors.New("the DAG is running")

func (e *client) UpdateStatus(ctx context.Context, dag *digraph.DAG, status persistence.Status) error {
	client := sock.NewClient(dag.SockAddr())

	res, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return err
		}
	} else {
		unmarshalled, _ := persistence.StatusFromJSON(res)
		if unmarshalled != nil && unmarshalled.RequestID == status.RequestID &&
			unmarshalled.Status == scheduler.StatusRunning {
			return errDAGIsRunning
		}
	}

	return e.historyStore.Update(ctx, dag.Name, status.RequestID, status)
}

func (e *client) UpdateDAG(ctx context.Context, id string, spec string) error {
	return e.dagStore.UpdateSpec(ctx, id, []byte(spec))
}

func (e *client) DeleteDAG(ctx context.Context, name string) error {
	return e.dagStore.Delete(ctx, name)
}

func (e *client) GetAllStatus(ctx context.Context) (
	statuses []DAGStatus, errs []string, err error,
) {
	dagList, errs, err := e.dagStore.List(ctx)

	var ret []DAGStatus
	for _, d := range dagList {
		status, err := e.readStatus(ctx, d)
		if err != nil {
			errs = append(errs, err.Error())
		}
		ret = append(ret, status)
	}

	return ret, errs, err
}

func (e *client) getPageCount(total int, limit int) int {
	return (total-1)/(limit) + 1
}

func (e *client) GetAllStatusPagination(ctx context.Context, params dags.ListDAGsParams) ([]DAGStatus, *DagListPaginationSummaryResult, error) {
	var (
		dagListPaginationResult *persistence.DagListPaginationResult
		err                     error
		dagStatusList           = make([]DAGStatus, 0)
	)

	page := 1
	if params.Page != nil {
		page = int(*params.Page)
	}
	limit := 100
	if params.Limit != nil {
		limit = int(*params.Limit)
	}

	if dagListPaginationResult, err = e.dagStore.ListPagination(ctx, persistence.DAGListPaginationArgs{
		Page:  page,
		Limit: limit,
		Name:  fromPtr(params.SearchName),
		Tag:   fromPtr(params.SearchTag),
	}); err != nil {
		return dagStatusList, &DagListPaginationSummaryResult{PageCount: 1}, err
	}

	for _, currentDag := range dagListPaginationResult.DagList {
		var (
			currentStatus DAGStatus
			err           error
		)
		if currentStatus, err = e.readStatus(ctx, currentDag); err != nil {
			dagListPaginationResult.ErrorList = append(dagListPaginationResult.ErrorList, err.Error())
		}
		dagStatusList = append(dagStatusList, currentStatus)
	}

	return dagStatusList, &DagListPaginationSummaryResult{
		PageCount: e.getPageCount(dagListPaginationResult.Count, limit),
		ErrorList: dagListPaginationResult.ErrorList,
	}, nil
}

func (e *client) getDAG(ctx context.Context, name string) (*digraph.DAG, error) {
	dagDetail, err := e.dagStore.GetDetails(ctx, name)
	return e.emptyDAGIfNil(dagDetail, name), err
}

func (e *client) GetStatus(ctx context.Context, name string) (DAGStatus, error) {
	dag, err := e.getDAG(ctx, name)
	if dag == nil {
		// TODO: fix not to use location
		dag = &digraph.DAG{Name: name, Location: name}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(dag.Steps...)
	}
	latestStatus, _ := e.GetLatestStatus(ctx, dag)
	return newDAGStatus(
		dag, latestStatus, e.IsSuspended(ctx, name), err,
	), err
}

func (e *client) ToggleSuspend(_ context.Context, name string, suspend bool) error {
	return e.flagStore.ToggleSuspend(name, suspend)
}

func (e *client) readStatus(ctx context.Context, dag *digraph.DAG) (DAGStatus, error) {
	latestStatus, err := e.GetLatestStatus(ctx, dag)
	id := strings.TrimSuffix(
		filepath.Base(dag.Location),
		filepath.Ext(dag.Location),
	)

	return newDAGStatus(
		dag, latestStatus, e.IsSuspended(ctx, id), err,
	), err
}

func (*client) emptyDAGIfNil(dag *digraph.DAG, dagLocation string) *digraph.DAG {
	if dag != nil {
		return dag
	}
	return &digraph.DAG{Location: dagLocation}
}

func (e *client) IsSuspended(_ context.Context, id string) bool {
	return e.flagStore.IsSuspended(id)
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

func (e *client) GetTagList(ctx context.Context) ([]string, []string, error) {
	return e.dagStore.TagList(ctx)
}

func fromPtr[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
