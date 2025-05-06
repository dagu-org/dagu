package runstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/sock"
)

// NewClient creates a new Client instance.
// The Client is used to interact with the DAG.
func NewClient(
	runStore Store,
	executable string,
	workDir string,
) Client {
	return Client{
		runStore:   runStore,
		executable: executable,
		workDir:    workDir,
	}
}

type Client struct {
	runStore   Store
	executable string
	workDir    string
}

func (e *Client) LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	opts = append(slices.Clone(opts), digraph.WithoutEval())
	return digraph.LoadYAML(ctx, spec, opts...)
}

func (e *Client) Rename(ctx context.Context, oldName, newName string) error {
	if err := e.runStore.Rename(ctx, oldName, newName); err != nil {
		return fmt.Errorf("failed to rename DAG: %w", err)
	}
	return nil
}

func (e *Client) StopDAG(ctx context.Context, dag *digraph.DAG, requestID string) error {
	logger.Info(ctx, "Stopping", "name", dag.Name)
	addr := dag.SockAddr(requestID)
	if !fileutil.FileExists(addr) {
		logger.Info(ctx, "The DAG is not running", "name", dag.Name)
		return nil
	}
	client := sock.NewClient(addr)
	_, err := client.Request("POST", "/stop")
	return err
}

func (e *Client) StartDAG(_ context.Context, dag *digraph.DAG, opts StartOptions) error {
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

func (e *Client) RestartDAG(_ context.Context, dag *digraph.DAG, opts RestartOptions) error {
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

func (e *Client) RetryDAG(_ context.Context, dag *digraph.DAG, requestID string) error {
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

func (e *Client) IsRunning(ctx context.Context, dag *digraph.DAG, requestID string) bool {
	_, err := e.currentStatus(ctx, dag, requestID)
	return err == nil
}

func (e *Client) GetCurrentStatus(ctx context.Context, dag *digraph.DAG, requestId string) (*Status, error) {
	status, err := e.currentStatus(ctx, dag, requestId)
	if err != nil {
		// No such file or directory
		if errors.Is(err, os.ErrNotExist) {
			goto FALLBACK
		}
		if errors.Is(err, sock.ErrTimeout) {
			goto FALLBACK
		}
		return nil, fmt.Errorf("failed to get current status: %w", err)
	}
	return status, nil

FALLBACK:
	if requestId == "" {
		// The DAG is not running so return the default status
		status := InitialStatus(dag)
		return &status, nil
	}
	return e.GetStatusByRequestID(ctx, dag, requestId)
}

func (e *Client) GetStatus(ctx context.Context, name string, requestID string) (*Status, error) {
	record, err := e.runStore.FindByRequestID(ctx, name, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by request id: %w", err)
	}
	latestStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return latestStatus, nil
}

func (e *Client) GetStatusByRequestID(ctx context.Context, dag *digraph.DAG, requestID string) (
	*Status, error,
) {
	record, err := e.runStore.FindByRequestID(ctx, dag.Name, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by request id: %w", err)
	}
	latestStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the DAG is running, query the current status
	if latestStatus.Status == scheduler.StatusRunning {
		currentStatus, err := e.currentStatus(ctx, dag, latestStatus.RequestID)
		if err == nil {
			return currentStatus, nil
		}
	}

	// If querying the current status fails, even if the status is running,
	// set the status to error
	if latestStatus.Status == scheduler.StatusRunning {
		latestStatus.Status = scheduler.StatusError
	}

	return latestStatus, nil
}

// GetStatusByChildRunRequestID retrieves the status of a child run by its request ID.
func (e *Client) GetStatusByChildRunRequestID(ctx context.Context, name string, requestID string) (*Status, error) {
	root := digraph.NewRootDAG(name, requestID)
	record, err := e.runStore.FindByChildRequestID(ctx, name, root)
	if err != nil {
		return nil, fmt.Errorf("failed to find child status by request id: %w", err)
	}
	latestStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return latestStatus, nil
}

func (*Client) currentStatus(_ context.Context, dag *digraph.DAG, requestId string) (*Status, error) {
	// FIXME: Should handle the case of dynamic DAG
	client := sock.NewClient(dag.SockAddr(requestId))
	statusJSON, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get current status: %w", err)
	}

	return StatusFromJSON(statusJSON)
}

func (e *Client) GetLatestStatus(ctx context.Context, dag *digraph.DAG) (Status, error) {
	var latestStatus *Status

	// Find the latest status by name
	record, err := e.runStore.Latest(ctx, dag.Name)
	if err != nil {
		goto handleError
	}

	// Read the latest status
	latestStatus, err = record.ReadStatus(ctx)
	if err != nil {
		goto handleError
	}

	// If the DAG is running, query the current status
	if latestStatus.Status == scheduler.StatusRunning {
		currentStatus, err := e.currentStatus(ctx, dag, latestStatus.RequestID)
		if err == nil {
			return *currentStatus, nil
		}
	}

	// If querying the current status fails, even if the status is running,
	// set the status to error
	if latestStatus.Status == scheduler.StatusRunning {
		latestStatus.Status = scheduler.StatusError
	}

	return *latestStatus, nil

handleError:

	// If the latest status is not found, return the default status
	ret := InitialStatus(dag)
	if errors.Is(err, ErrNoStatusData) {
		// No status for today
		return ret, nil
	}

	return ret, err
}

func (e *Client) GetRecentHistory(ctx context.Context, name string, n int) []Status {
	records := e.runStore.Recent(ctx, name, n)

	var runs []Status
	for _, record := range records {
		if status, err := record.ReadStatus(ctx); err == nil {
			runs = append(runs, *status)
		}
	}

	return runs
}

func (e *Client) UpdateStatus(ctx context.Context, name string, status Status) error {
	return e.runStore.Update(ctx, name, status.RequestID, status)
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

type StartOptions struct {
	Params string
	Quiet  bool
}

type RestartOptions struct {
	Quiet bool
}
