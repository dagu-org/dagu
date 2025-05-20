package history

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
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
	repo models.HistoryRepository,
	executable string,
	workDir string,
) Manager {
	return Manager{
		historyRepo: repo,
		executable:  executable,
		workDir:     workDir,
	}
}

// Manager provides methods to interact with DAGs, including starting, stopping,
// restarting, and retrieving status information. It communicates with the DAG
// through a socket interface and manages execution history.
type Manager struct {
	historyRepo models.HistoryRepository // Store interface for persisting run data

	executable string // Path to the executable used to run DAGs
	workDir    string // Working directory for executing commands
}

// LoadYAML loads a DAG from YAML specification bytes without evaluating it.
// It appends the WithoutEval option to any provided options.
func (m *Manager) LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	opts = append(slices.Clone(opts), digraph.WithoutEval())
	return digraph.LoadYAML(ctx, spec, opts...)
}

// Stop stops a running DAG by sending a stop request to its socket.
// If the DAG is not running, it logs a message and returns nil.
func (m *Manager) Stop(ctx context.Context, dag *digraph.DAG, workflowID string) error {
	logger.Info(ctx, "Stopping", "name", dag.Name)
	addr := dag.SockAddr(workflowID)
	if !fileutil.FileExists(addr) {
		logger.Info(ctx, "The DAG is not running", "name", dag.Name)
		return nil
	}
	client := sock.NewClient(addr)
	_, err := client.Request("POST", "/stop")
	return err
}

// GenWorkflowID generates a unique workflow ID for a workflow using UUID v7.
func (m *Manager) GenWorkflowID(_ context.Context) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate workflow ID: %w", err)
	}
	return id.String(), nil
}

// StartDAG starts a workflow by executing the configured executable with the start command.
// It sets up the command to run in its own process group and configures standard output/error.
func (m *Manager) StartDAG(_ context.Context, dag *digraph.DAG, opts StartOptions) error {
	args := []string{"start"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, escapeArg(opts.Params)))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.WorkflowID != "" {
		args = append(args, fmt.Sprintf("--workflow-id=%s", opts.WorkflowID))
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

// RestartDAG restarts a DAG by executing the configured executable with the restart command.
// It sets up the command to run in its own process group.
func (m *Manager) RestartDAG(_ context.Context, dag *digraph.DAG, opts RestartOptions) error {
	args := []string{"restart"}
	if opts.Quiet {
		args = append(args, "-q")
	}
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	return cmd.Start()
}

// RetryDAG retries a workflow with the specified workflow ID by executing
// the configured executable with the retry command.
func (m *Manager) RetryDAG(_ context.Context, dag *digraph.DAG, workflowID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--workflow-id=%s", workflowID))
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	return cmd.Start()
}

// IsDAGRunning checks if a workflow is currently running by attempting to get its current status.
// Returns true if the status can be retrieved without error, false otherwise.
func (m *Manager) IsDAGRunning(ctx context.Context, dag *digraph.DAG, workflowID string) bool {
	_, err := m.currentStatus(ctx, dag, workflowID)
	return err == nil
}

// GetDAGRealtimeStatus retrieves the current status of a workflow.
// If the workflow is running, it gets the status from the socket.
// If the socket doesn't exist or times out, it falls back to stored status or creates an initial status.
func (m *Manager) GetDAGRealtimeStatus(ctx context.Context, dag *digraph.DAG, workflowID string) (*models.Status, error) {
	status, err := m.currentStatus(ctx, dag, workflowID)
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
	if workflowID == "" {
		// The DAG is not running so return the default status
		status := models.InitialStatus(dag)
		return &status, nil
	}
	return m.findPersistedStatus(ctx, dag, workflowID)
}

// FindWorkflowStatus retrieves the status of a workflow by name and workflow ID from the execution history.
func (e *Manager) FindWorkflowStatus(ctx context.Context, ref digraph.WorkflowRef) (*models.Status, error) {
	run, err := e.historyRepo.FindRun(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by workflow ID: %w", err)
	}
	status, err := run.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return status, nil
}

// findPersistedStatus retrieves the status of a workflow by workflow ID.
// If the stored status indicates the DAG is running, it attempts to get the current status.
// If that fails, it marks the status as error.
func (m *Manager) findPersistedStatus(ctx context.Context, dag *digraph.DAG, workflowID string) (
	*models.Status, error,
) {
	ref := digraph.NewWorkflowRef(dag.Name, workflowID)
	run, err := m.historyRepo.FindRun(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by workflow ID: %w", err)
	}
	latestStatus, err := run.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the DAG is running, query the current status
	if latestStatus.Status == scheduler.StatusRunning {
		currentStatus, err := m.currentStatus(ctx, dag, latestStatus.WorkflowID)
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

// FindChildWorkflowStatus retrieves the status of a child workflow by its workflow ID.
func (m *Manager) FindChildWorkflowStatus(ctx context.Context, ref digraph.WorkflowRef, workflowID string) (*models.Status, error) {
	run, err := m.historyRepo.FindChildWorkflowRun(ctx, ref, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to find child workflow status by workflow ID: %w", err)
	}
	latestStatus, err := run.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return latestStatus, nil
}

// currentStatus retrieves the current status of a running DAG by querying its socket.
// This is a private method used internally by other status-related methods.
func (*Manager) currentStatus(_ context.Context, dag *digraph.DAG, workflowID string) (*models.Status, error) {
	// FIXME: Should handle the case of dynamic DAG
	client := sock.NewClient(dag.SockAddr(workflowID))
	statusJSON, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get current status: %w", err)
	}

	return models.StatusFromJSON(statusJSON)
}

// GetLatestStatus retrieves the latest status of a DAG.
// If the DAG is running, it attempts to get the current status from the socket.
// If that fails or no status exists, it returns an initial status or an error.
func (m *Manager) GetLatestStatus(ctx context.Context, dag *digraph.DAG) (models.Status, error) {
	var latestStatus *models.Status

	// Find the latest status by name
	run, err := m.historyRepo.LatestRun(ctx, dag.Name)
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
		currentStatus, err := m.currentStatus(ctx, dag, latestStatus.WorkflowID)
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
	ret := models.InitialStatus(dag)
	if errors.Is(err, models.ErrNoStatusData) {
		// No status for today
		return ret, nil
	}

	return ret, err
}

// ListRecentStatus retrieves the n most recent statuses for a DAG by name.
// It returns a slice of Status objects, filtering out any that cannot be read.
func (m *Manager) ListRecentStatus(ctx context.Context, name string, n int) []models.Status {
	runs := m.historyRepo.RecentRuns(ctx, name, n)

	var statuses []models.Status
	for _, run := range runs {
		if status, err := run.ReadStatus(ctx); err == nil {
			statuses = append(statuses, *status)
		}
	}

	return statuses
}

// UpdateStatus updates the status of a workflow execution.
// It finds the run for the workflow and writes the new status to it.
func (e *Manager) UpdateStatus(ctx context.Context, root digraph.WorkflowRef, status models.Status) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("update canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Find the run for the status.
	var run models.Run

	if root.WorkflowID == status.WorkflowID {
		// If the workflow ID matches the root workflow ID, find the run by the root workflow ID
		r, err := e.historyRepo.FindRun(ctx, root)
		if err != nil {
			return fmt.Errorf("failed to find the run: %w", err)
		}
		run = r
	} else {
		// If the workflow ID does not match, find the child workflow run
		// by the root workflow ID and the child workflow ID
		r, err := e.historyRepo.FindChildWorkflowRun(ctx, root, status.WorkflowID)
		if err != nil {
			return fmt.Errorf("failed to find child workflow run: %w", err)
		}
		run = r
	}

	// Open, write, and close the run
	if err := run.Open(ctx); err != nil {
		return fmt.Errorf("failed to open workflow run data: %w", err)
	}

	// Ensure the run data is closed even if write fails
	defer func() {
		if closeErr := run.Close(ctx); closeErr != nil {
			logger.Errorf(ctx, "Failed to close workflow run data: %v", closeErr)
		}
	}()

	if err := run.Write(ctx, status); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

// escapeArg escapes special characters in command arguments.
// Currently handles carriage returns and newlines by adding backslashes.
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

// StartOptions contains options for starting a DAG.
type StartOptions struct {
	Params     string // Parameters to pass to the DAG
	Quiet      bool   // Whether to run in quiet mode
	WorkflowID string // Workflow ID for the workflow
}

// RestartOptions contains options for restarting a DAG.
type RestartOptions struct {
	Quiet bool // Whether to run in quiet mode
}
