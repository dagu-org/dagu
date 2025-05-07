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

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/google/uuid"
)

// New creates a new Manager instance.
// The Manager is used to interact with the DAG.
func New(
	repo HistoryRepository,
	executable string,
	workDir string,
	configPath string,
) Manager {
	return Manager{
		HistoryRepository: repo,
		executable:        executable,
		workDir:           workDir,
		configPath:        configPath,
	}
}

// Manager provides methods to interact with DAGs, including starting, stopping,
// restarting, and retrieving status information. It communicates with the DAG
// through a socket interface and manages run records through a Store.
type Manager struct {
	HistoryRepository // Store interface for persisting run data

	executable string // Path to the executable used to run DAGs
	workDir    string // Working directory for executing commands
	configPath string // Path to the configuration file
}

// LoadYAML loads a DAG from YAML specification bytes without evaluating it.
// It appends the WithoutEval option to any provided options.
func (m *Manager) LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	opts = append(slices.Clone(opts), digraph.WithoutEval())
	return digraph.LoadYAML(ctx, spec, opts...)
}

// Stop stops a running DAG by sending a stop request to its socket.
// If the DAG is not running, it logs a message and returns nil.
func (m *Manager) Stop(ctx context.Context, dag *digraph.DAG, requestID string) error {
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

// GenerateRequestID generates a unique request ID for a DAG run using UUID v7.
func (m *Manager) GenerateRequestID(_ context.Context) (string, error) {
	// Generate a unique request ID for the DAG run
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate request ID: %w", err)
	}
	return id.String(), nil
}

// Start starts a DAG by executing the configured executable with appropriate arguments.
// It sets up the command to run in its own process group and configures standard output/error.
func (m *Manager) Start(_ context.Context, dag *digraph.DAG, opts StartOptions) error {
	args := []string{"start"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, fmt.Sprintf(`"%s"`, escapeArg(opts.Params)))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.RequestID != "" {
		args = append(args, fmt.Sprintf("--request-id=%s", opts.RequestID))
	}
	if m.configPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", m.configPath))
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

// Restart restarts a DAG by executing the configured executable with the restart command.
// It sets up the command to run in its own process group.
func (m *Manager) Restart(_ context.Context, dag *digraph.DAG, opts RestartOptions) error {
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

// Retry retries a DAG execution with the specified requestID by executing
// the configured executable with the retry command.
func (m *Manager) Retry(_ context.Context, dag *digraph.DAG, requestID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--request-id=%s", requestID))
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: 0}
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	return cmd.Start()
}

// IsRunning checks if a DAG is currently running by attempting to get its current status.
// Returns true if the status can be retrieved without error, false otherwise.
func (m *Manager) IsRunning(ctx context.Context, dag *digraph.DAG, requestID string) bool {
	_, err := m.currentStatus(ctx, dag, requestID)
	return err == nil
}

// GetRealtimeStatus retrieves the current status of a DAG.
// If the DAG is running, it gets the status from the socket.
// If the socket doesn't exist or times out, it falls back to stored status or creates an initial status.
func (m *Manager) GetRealtimeStatus(ctx context.Context, dag *digraph.DAG, requestId string) (*Status, error) {
	status, err := m.currentStatus(ctx, dag, requestId)
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
	return m.findPersistedStatus(ctx, dag, requestId)
}

// FindByRequestID retrieves the status of a DAG run by name and requestID from the run store.
func (e *Manager) FindByRequestID(ctx context.Context, name string, requestID string) (*Status, error) {
	record, err := e.Find(ctx, name, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by request id: %w", err)
	}
	latestStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return latestStatus, nil
}

// findPersistedStatus retrieves the status of a DAG run by requestID.
// If the stored status indicates the DAG is running, it attempts to get the current status.
// If that fails, it marks the status as error.
func (m *Manager) findPersistedStatus(ctx context.Context, dag *digraph.DAG, requestID string) (
	*Status, error,
) {
	record, err := m.Find(ctx, dag.Name, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by request id: %w", err)
	}
	latestStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the DAG is running, query the current status
	if latestStatus.Status == scheduler.StatusRunning {
		currentStatus, err := m.currentStatus(ctx, dag, latestStatus.RequestID)
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

// FindBySubRunRequestID retrieves the status of a sub-run by its request ID.
func (m *Manager) FindBySubRunRequestID(ctx context.Context, root digraph.RootDAG, requestID string) (*Status, error) {
	record, err := m.FindSubRun(ctx, root.RootName, root.RootID, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to find sub-run status by request id: %w", err)
	}
	latestStatus, err := record.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return latestStatus, nil
}

// currentStatus retrieves the current status of a running DAG by querying its socket.
// This is a private method used internally by other status-related methods.
func (*Manager) currentStatus(_ context.Context, dag *digraph.DAG, requestId string) (*Status, error) {
	// FIXME: Should handle the case of dynamic DAG
	client := sock.NewClient(dag.SockAddr(requestId))
	statusJSON, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get current status: %w", err)
	}

	return StatusFromJSON(statusJSON)
}

// GetLatestStatus retrieves the latest status of a DAG.
// If the DAG is running, it attempts to get the current status from the socket.
// If that fails or no status exists, it returns an initial status or an error.
func (m *Manager) GetLatestStatus(ctx context.Context, dag *digraph.DAG) (Status, error) {
	var latestStatus *Status

	// Find the latest status by name
	record, err := m.Latest(ctx, dag.Name)
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
		currentStatus, err := m.currentStatus(ctx, dag, latestStatus.RequestID)
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

// ListRecentHistory retrieves the n most recent status records for a DAG by name.
// It returns a slice of Status objects, filtering out any that cannot be read.
func (m *Manager) ListRecentHistory(ctx context.Context, name string, n int) []Status {
	records := m.Recent(ctx, name, n)

	var runs []Status
	for _, record := range records {
		if status, err := record.ReadStatus(ctx); err == nil {
			runs = append(runs, *status)
		}
	}

	return runs
}

// UpdateStatus updates the status of a DAG run in the run store.
func (e *Manager) UpdateStatus(ctx context.Context, root digraph.RootDAG, status Status) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("update canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Find the runstore record
	var historyRecord Record

	if root.RootID == status.RequestID {
		// If the request ID matches the root DAG's request ID, find the runstore record by request ID
		r, err := e.Find(ctx, root.RootName, status.RequestID)
		if err != nil {
			return fmt.Errorf("failed to find runstore record: %w", err)
		}
		historyRecord = r
	} else {
		// If the request ID does not match, find the runstore record by sub-run request ID
		r, err := e.FindSubRun(ctx, root.RootName, root.RootID, status.RequestID)
		if err != nil {
			return fmt.Errorf("failed to find sub-runstore record: %w", err)
		}
		historyRecord = r
	}

	// Open, write, and close the runstore record
	if err := historyRecord.Open(ctx); err != nil {
		return fmt.Errorf("failed to open runstore record: %w", err)
	}

	// Ensure the record is closed even if write fails
	defer func() {
		if closeErr := historyRecord.Close(ctx); closeErr != nil {
			logger.Errorf(ctx, "Failed to close runstore record: %v", closeErr)
		}
	}()

	if err := historyRecord.Write(ctx, status); err != nil {
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
	Params    string // Parameters to pass to the DAG
	Quiet     bool   // Whether to run in quiet mode
	RequestID string // Request ID for the DAG run
}

// RestartOptions contains options for restarting a DAG.
type RestartOptions struct {
	Quiet bool // Whether to run in quiet mode
}
