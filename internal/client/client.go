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

func (e *client) LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	return e.dagStore.LoadSpec(ctx, spec, opts...)
}

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

func (e *client) GrepDAG(ctx context.Context, pattern string) (
	[]*persistence.GrepResult, []string, error,
) {
	return e.dagStore.Grep(ctx, pattern)
}

func (e *client) MoveDAG(ctx context.Context, oldLoc, newLoc string) error {
	oldDAG, err := e.dagStore.GetMetadata(ctx, oldLoc)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", oldLoc, err)
	}
	if err := e.dagStore.Rename(ctx, oldLoc, newLoc); err != nil {
		return err
	}
	newDAG, err := e.dagStore.GetMetadata(ctx, newLoc)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", newLoc, err)
	}
	if err := e.historyStore.Rename(ctx, oldDAG.Name, newDAG.Name); err != nil {
		return fmt.Errorf("failed to rename history for %s: %w", oldLoc, err)
	}
	return nil
}

func (e *client) StopDAG(ctx context.Context, dag *digraph.DAG) error {
	logger.Info(ctx, "Stopping", "name", dag.Name)
	addr := dag.SockAddr("") // FIXME: Should handle the case of dynamic DAG
	if !fileutil.FileExists(addr) {
		logger.Info(ctx, "The DAG is not running", "name", dag.Name)
		return nil
	}
	client := sock.NewClient(addr)
	_, err := client.Request("POST", "/stop")
	return err
}

func (e *client) StartDAG(_ context.Context, dag *digraph.DAG, opts StartOptions) error {
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

	return cmd.Start()
}

func (e *client) RestartDAG(_ context.Context, dag *digraph.DAG, opts RestartOptions) error {
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
	return cmd.Start()
}

func (e *client) RetryDAG(_ context.Context, dag *digraph.DAG, requestID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--request-id=%s", requestID))
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(e.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = e.workDir
	cmd.Env = os.Environ()
	return cmd.Start()
}

func (*client) GetCurrentStatus(_ context.Context, dag *digraph.DAG) (*persistence.Status, error) {
	// FIXME: Should handle the case of dynamic DAG
	client := sock.NewClient(dag.SockAddr(""))
	ret, err := client.Request("GET", "/status")
	if err != nil {
		if errors.Is(err, sock.ErrTimeout) {
			return nil, err
		}
		// The DAG is not running so return the default status
		status := persistence.NewStatusFactory(dag).Default()
		return &status, nil
	}
	return persistence.StatusFromJSON(ret)
}

func (e *client) GetStatus(ctx context.Context, name string, requestID string) (*persistence.Status, error) {
	record, err := e.historyStore.FindByRequestID(ctx, name, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by request id: %w", err)
	}
	latestStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return latestStatus, nil
}

func (e *client) GetStatusByRequestID(ctx context.Context, dag *digraph.DAG, requestID string) (
	*persistence.Status, error,
) {
	record, err := e.historyStore.FindByRequestID(ctx, dag.Name, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by request id: %w", err)
	}
	latestStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the DAG is running, set the currentStatus to error if the request ID does not match
	// Because the DAG run must be stopped
	// TODO: Handle different request IDs for the same DAG
	currentStatus, _ := e.GetCurrentStatus(ctx, dag)
	if currentStatus != nil && currentStatus.RequestID != requestID {
		latestStatus.SetStatusToErrorIfRunning()
	}

	return latestStatus, err
}

func (*client) currentStatus(_ context.Context, dag *digraph.DAG) (*persistence.Status, error) {
	// FIXME: Should handle the case of dynamic DAG
	client := sock.NewClient(dag.SockAddr(""))
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
		return persistence.NewStatusFactory(dag).Default(), nil
	}

	return persistence.NewStatusFactory(dag).Default(), err
}

func (e *client) GetRecentHistory(ctx context.Context, name string, n int) []persistence.Run {
	records := e.historyStore.Recent(ctx, name, n)

	var runs []persistence.Run
	for _, record := range records {
		if run, err := record.ReadRun(ctx); err == nil {
			runs = append(runs, *run)
		}
	}

	return runs
}

var errDAGIsRunning = errors.New("the DAG is running")

func (e *client) UpdateRunStatus(ctx context.Context, name, requestID string, status persistence.Status) error {
	return e.historyStore.Update(ctx, name, status.RequestID, status)
}

func (e *client) UpdateStatus(ctx context.Context, dag *digraph.DAG, status persistence.Status) error {
	// FIXME: Should handle the case of dynamic DAG
	client := sock.NewClient(dag.SockAddr(""))

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

func (e *client) ListStatus(ctx context.Context, opts ...ListStatusOption) (*persistence.PaginatedResult[DAGStatus], []string, error) {
	var options GetAllStatusOptions
	for _, opt := range opts {
		opt(&options)
	}
	if options.Limit == nil {
		options.Limit = new(int)
		*options.Limit = 100
	}
	if options.Page == nil {
		options.Page = new(int)
		*options.Page = 1
	}

	pg := persistence.NewPaginator(*options.Page, *options.Limit)

	dags, errList, err := e.dagStore.List(ctx, persistence.ListOptions{
		Paginator: &pg,
		Name:      fromPtr(options.Name),
		Tag:       fromPtr(options.Tag),
	})
	if err != nil {
		return nil, errList, err
	}

	var items []DAGStatus
	for _, d := range dags.Items {
		status, err := e.readStatus(ctx, d)
		if err != nil {
			errList = append(errList, err.Error())
		}
		items = append(items, status)
	}

	r := persistence.NewPaginatedResult(items, dags.TotalCount, pg)
	return &r, errList, nil
}

func (e *client) getDAG(ctx context.Context, loc string) (*digraph.DAG, error) {
	dagDetail, err := e.dagStore.GetDetails(ctx, loc)
	return e.emptyDAGIfNil(dagDetail, loc), err
}

func (e *client) GetDAGStatus(ctx context.Context, loc string) (DAGStatus, error) {
	dag, err := e.getDAG(ctx, loc)
	if dag == nil {
		// TODO: fix not to use location
		dag = &digraph.DAG{Name: loc, Location: loc}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(dag.Steps...)
	}
	latestStatus, _ := e.GetLatestStatus(ctx, dag)
	return newDAGStatus(
		dag, latestStatus, e.IsSuspended(ctx, loc), err,
	), err
}

func (e *client) ToggleSuspend(_ context.Context, loc string, suspend bool) error {
	return e.flagStore.ToggleSuspend(loc, suspend)
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
		switch char {
		case '\r':
			_, _ = escaped.WriteString("\\r")
		case '\n':
			_, _ = escaped.WriteString("\\n")
		default:
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
