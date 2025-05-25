package history

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"syscall"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/google/uuid"
)

// New creates a new Manager instance.
// The Manager is used to interact with the DAG.
func New(
	hs models.DAGRunStore,
	executable string,
	workDir string,
) DAGRunManager {
	return DAGRunManager{
		dagRunStore: hs,
		executable:  executable,
		workDir:     workDir,
	}
}

// DAGRunManager provides methods to interact with DAGs, including starting, stopping,
// restarting, and retrieving status information. It communicates with the DAG
// through a socket interface and manages execution history.
type DAGRunManager struct {
	dagRunStore models.DAGRunStore // Store interface for persisting run data

	executable string // Path to the executable used to run DAGs
	workDir    string // Working directory for executing commands
}

// LoadYAML loads a DAG from YAML specification bytes without evaluating it.
// It appends the WithoutEval option to any provided options.
func (m *DAGRunManager) LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	opts = append(slices.Clone(opts), digraph.WithoutEval())
	return digraph.LoadYAML(ctx, spec, opts...)
}

// Stop stops a running DAG by sending a stop request to its socket.
// If the DAG is not running, it logs a message and returns nil.
func (m *DAGRunManager) Stop(ctx context.Context, dag *digraph.DAG, dagRunID string) error {
	logger.Info(ctx, "Stopping", "name", dag.Name)
	addr := dag.SockAddr(dagRunID)
	if !fileutil.FileExists(addr) {
		logger.Info(ctx, "The DAG is not running", "name", dag.Name)
		return nil
	}
	client := sock.NewClient(addr)
	_, err := client.Request("POST", "/stop")
	return err
}

// GenDAGRunID generates a unique ID for a DAG run using UUID version 7.
func (m *DAGRunManager) GenDAGRunID(_ context.Context) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate DAG run ID: %w", err)
	}
	return id.String(), nil
}

// StartDAGRun starts a DAG run by executing the configured executable with the start command.
// It sets up the command to run in its own process group and configures standard output/error.
func (m *DAGRunManager) StartDAGRun(_ context.Context, dag *digraph.DAG, opts StartOptions) error {
	args := []string{"start"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, strconv.Quote(opts.Params))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.DAGRunID != "" {
		args = append(args, fmt.Sprintf("--workflow-id=%s", opts.DAGRunID))
	}
	if configFile := config.UsedConfigFile.Load(); configFile != nil {
		if configFile, ok := configFile.(string); ok {
			args = append(args, "--config")
			args = append(args, configFile)
		}
	}
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Start()
}

// EnqueueDAGRun enqueues a DAG run by executing the configured executable with the enqueue command.
func (m *DAGRunManager) EnqueueDAGRun(_ context.Context, dag *digraph.DAG, opts EnqueueOptions) error {
	args := []string{"enqueue"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, strconv.Quote(opts.Params))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.DAGRunID != "" {
		args = append(args, fmt.Sprintf("--workflow-id=%s", opts.DAGRunID))
	}
	if configFile := config.UsedConfigFile.Load(); configFile != nil {
		if configFile, ok := configFile.(string); ok {
			args = append(args, "--config")
			args = append(args, configFile)
		}
	}
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start enqueue command: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to enqueue DAG run: %w", err)
	}
	return nil
}

func (m *DAGRunManager) DequeueDAGRun(_ context.Context, dagRun digraph.DAGRunRef) error {
	args := []string{"dequeue", fmt.Sprintf("--workflow=%s", dagRun.String())}
	if configFile := config.UsedConfigFile.Load(); configFile != nil {
		if configFile, ok := configFile.(string); ok {
			args = append(args, "--config")
			args = append(args, configFile)
		}
	}
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dequeue command: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to dequeue DAG run: %w", err)
	}
	return nil
}

// RestartDAG restarts a DAG by executing the configured executable with the restart command.
// It sets up the command to run in its own process group.
func (m *DAGRunManager) RestartDAG(_ context.Context, dag *digraph.DAG, opts RestartOptions) error {
	args := []string{"restart"}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if configFile := config.UsedConfigFile.Load(); configFile != nil {
		if configFile, ok := configFile.(string); ok {
			args = append(args, "--config")
			args = append(args, configFile)
		}
	}
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	return cmd.Start()
}

// RetryDAGRun retries a DAG run by executing the configured executable with the retry command.
func (m *DAGRunManager) RetryDAGRun(_ context.Context, dag *digraph.DAG, dagRunID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--workflow-id=%s", dagRunID))
	if configFile := config.UsedConfigFile.Load(); configFile != nil {
		if configFile, ok := configFile.(string); ok {
			args = append(args, "--config")
			args = append(args, configFile)
		}
	}
	args = append(args, dag.Name)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	return cmd.Start()
}

// IsRunning checks if a DAG run is currently running by querying its status.
// Returns true if the status can be retrieved without error, indicating the DAG is running.
func (m *DAGRunManager) IsRunning(ctx context.Context, dag *digraph.DAG, dagRunID string) bool {
	_, err := m.currentStatus(ctx, dag, dagRunID)
	return err == nil
}

// GetCurrentStatus retrieves the current status of a DAG run by its run ID.
// If the DAG run is running, it queries the socket for the current status.
// If the socket doesn't exist or times out, it falls back to stored status or creates an initial status.
func (m *DAGRunManager) GetCurrentStatus(ctx context.Context, dag *digraph.DAG, dagRunID string) (*models.DAGRunStatus, error) {
	status, err := m.currentStatus(ctx, dag, dagRunID)
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
	if dagRunID == "" {
		// The DAG is not running so return the default status
		status := models.InitialStatus(dag)
		return &status, nil
	}
	return m.getPersistedOrCurrentStatus(ctx, dag, dagRunID)
}

// GetSavedStatus retrieves the saved status of a DAG run by its digraph.DAGRun reference.
func (e *DAGRunManager) GetSavedStatus(ctx context.Context, dagRun digraph.DAGRunRef) (*models.DAGRunStatus, error) {
	run, err := e.dagRunStore.FindAttempt(ctx, dagRun)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by run reference: %w", err)
	}
	status, err := run.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return status, nil
}

// getPersistedOrCurrentStatus retrieves the persisted status of a DAG run by its ID.
// If the stored status indicates the DAG is running, it attempts to get the current status.
// If status is running and current status retrieval fails, it marks the status as error.
func (m *DAGRunManager) getPersistedOrCurrentStatus(ctx context.Context, dag *digraph.DAG, dagRunID string) (
	*models.DAGRunStatus, error,
) {
	ref := digraph.NewDAGRunRef(dag.Name, dagRunID)
	att, err := m.dagRunStore.FindAttempt(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by DAG run ID: %w", err)
	}
	status, err := att.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the DAG is running, query the current status
	if status.Status == scheduler.StatusRunning {
		currentStatus, err := m.currentStatus(ctx, dag, status.DAGRunID)
		if err == nil {
			return currentStatus, nil
		}
	}

	// If querying the current status fails, even if the status is running,
	// set the status to error because it indicates the process is not responding.
	if status.Status == scheduler.StatusRunning {
		status.Status = scheduler.StatusError
	}

	return status, nil
}

// FindChildDAGRunStatus retrieves the status of a child DAG run by its ID.
// It looks up the child attempt in the history store and reads its status.
func (m *DAGRunManager) FindChildDAGRunStatus(ctx context.Context, rootDAGRun digraph.DAGRunRef, childRunID string) (*models.DAGRunStatus, error) {
	att, err := m.dagRunStore.FindChildAttempt(ctx, rootDAGRun, childRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to find child DAG run attempt: %w", err)
	}
	status, err := att.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return status, nil
}

// currentStatus retrieves the current status of a running DAG by querying its socket.
// This is a private method used internally by other status-related methods.
func (*DAGRunManager) currentStatus(_ context.Context, dag *digraph.DAG, dagRunID string) (*models.DAGRunStatus, error) {
	// FIXME: Should handle the case of dynamic DAG
	client := sock.NewClient(dag.SockAddr(dagRunID))
	statusJSON, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get current status: %w", err)
	}

	return models.StatusFromJSON(statusJSON)
}

// GetLatestStatus retrieves the latest status of a DAG.
// If the DAG is running, it attempts to get the current status from the socket.
// If that fails or no status exists, it returns an initial status or an error.
func (m *DAGRunManager) GetLatestStatus(ctx context.Context, dag *digraph.DAG) (models.DAGRunStatus, error) {
	var latestStatus *models.DAGRunStatus

	// Find the latest status by name
	run, err := m.dagRunStore.LatestAttempt(ctx, dag.Name)
	if err != nil {
		goto handleError
	}

	// Read the latest status
	latestStatus, err = run.ReadStatus(ctx)
	if err != nil {
		goto handleError
	}

	// If the DAG is running, query the current status
	if latestStatus.Status == scheduler.StatusRunning {
		currentStatus, err := m.currentStatus(ctx, dag, latestStatus.DAGRunID)
		if err == nil {
			return *currentStatus, nil
		} else {
			logger.Debug(ctx, "Failed to get current status from socket", "error", err)
		}
	}

	// If querying the current status fails, ensure if the status is running,
	if latestStatus.Status == scheduler.StatusRunning {
		// Check the PID is still alive
		pid := int(latestStatus.PID)
		if pid > 0 {
			_, err := os.FindProcess(pid)
			if err != nil {
				// If we cannot find the process, mark the status as error
				latestStatus.Status = scheduler.StatusError
				logger.Warn(ctx, "No PID set for running status, marking status as error")
			}
		}
	}

	return *latestStatus, nil

handleError:

	// If the latest status is not found, return the default status
	ret := models.InitialStatus(dag)
	if errors.Is(err, models.ErrNoStatusData) {
		// No status for today
		return ret, nil
	}

	return ret, err
}

// ListRecentStatus retrieves the n most recent statuses for a DAG by name.
// It returns a slice of Status objects, filtering out any that cannot be read.
func (m *DAGRunManager) ListRecentStatus(ctx context.Context, name string, n int) []models.DAGRunStatus {
	runs := m.dagRunStore.RecentAttempts(ctx, name, n)

	var statuses []models.DAGRunStatus
	for _, run := range runs {
		if status, err := run.ReadStatus(ctx); err == nil {
			statuses = append(statuses, *status)
		}
	}

	return statuses
}

// UpdateStatus updates the status of a DAG run.
func (e *DAGRunManager) UpdateStatus(ctx context.Context, rootDAGRun digraph.DAGRunRef, newStatus models.DAGRunStatus) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("update canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Find the run for the status.
	var run models.DAGRunAttempt

	if rootDAGRun.ID == newStatus.DAGRunID {
		// If the DAG-run ID matches the root DAG-run ID, find the attempt by the root DAG-run ID
		r, err := e.dagRunStore.FindAttempt(ctx, rootDAGRun)
		if err != nil {
			return fmt.Errorf("failed to find the DAG-run: %w", err)
		}
		run = r
	} else {
		// If the DAG-run ID does not match the root DAG-run ID,
		// find the attempt by the child DAG-run ID
		r, err := e.dagRunStore.FindChildAttempt(ctx, rootDAGRun, newStatus.DAGRunID)
		if err != nil {
			return fmt.Errorf("failed to find child DAG-run: %w", err)
		}
		run = r
	}

	// Open, write, and close the run
	if err := run.Open(ctx); err != nil {
		return fmt.Errorf("failed to open DAG-run data: %w", err)
	}

	// Ensure the run data is closed even if write fails
	defer func() {
		if closeErr := run.Close(ctx); closeErr != nil {
			logger.Errorf(ctx, "Failed to close DAG-run data: %v", closeErr)
		}
	}()

	if err := run.Write(ctx, newStatus); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

// StartOptions contains options for initiating a DAG run.
type StartOptions struct {
	Params   string // Parameters to pass to the DAG
	Quiet    bool   // Whether to run in quiet mode
	DAGRunID string // ID for the DAG run
}

// EnqueueOptions contains options for enqueuing a DAG run.
type EnqueueOptions struct {
	Params   string // Parameters to pass to the DAG
	Quiet    bool   // Whether to run in quiet mode
	DAGRunID string // ID for the DAG run
}

// RestartOptions contains options for restarting a DAG run.
type RestartOptions struct {
	Quiet bool // Whether to run in quiet mode
}
