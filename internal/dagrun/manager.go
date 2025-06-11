package dagrun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"sync"
	"syscall"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/resource"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/google/uuid"
)

// New creates a new Manager instance.
// The Manager is used to interact with the DAG.
func New(
	drs models.DAGRunStore,
	ps models.ProcStore,
	executable string,
	workDir string,
) *Manager {
	return &Manager{
		dagRunStore: drs,
		procStore:   ps,
		executable:  executable,
		workDir:     workDir,
		// resourceController will be lazily initialized when first needed
	}
}

// Manager provides methods to interact with DAGs, including starting, stopping,
// restarting, and retrieving status information. It communicates with the DAG
// through a socket interface and manages dag-run data.
type Manager struct {
	dagRunStore models.DAGRunStore // Store interface for persisting run data
	procStore   models.ProcStore   // Store interface for process management

	executable         string                       // Path to the executable used to run DAGs
	workDir            string                       // Working directory for executing commands
	resourceController *resource.ResourceController // Controller for managing process resources
	resourceInitOnce   sync.Once                    // Ensures resource controller is initialized only once
	resourceInitError  error                        // Stores any initialization error
}

// getResourceController lazily initializes and returns the resource controller
func (m *Manager) getResourceController(ctx context.Context) *resource.ResourceController {
	m.resourceInitOnce.Do(func() {
		controller, err := resource.NewResourceController()
		if err != nil {
			logger.Warn(ctx, "Failed to initialize resource controller", "error", err)
			m.resourceInitError = err
		} else {
			m.resourceController = controller
		}
	})
	return m.resourceController
}

// LoadYAML loads a DAG from YAML specification bytes without evaluating it.
// It appends the WithoutEval option to any provided options.
func (m *Manager) LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	opts = append(slices.Clone(opts), digraph.WithoutEval())
	return digraph.LoadYAML(ctx, spec, opts...)
}

// Stop stops a running DAG by sending a stop request to its socket.
// If the DAG is not running, it logs a message and returns nil.
// It also cleans up any resource enforcement for the DAG.
func (m *Manager) Stop(ctx context.Context, dag *digraph.DAG, dagRunID string) error {
	logger.Info(ctx, "Stopping", "name", dag.Name)
	addr := dag.SockAddr(dagRunID)
	if !fileutil.FileExists(addr) {
		logger.Info(ctx, "The DAG is not running", "name", dag.Name)
		return nil
	}

	// Send stop request to the DAG
	client := sock.NewClient(addr)
	_, err := client.Request("POST", "/stop")
	return err
}

// GenDAGRunID generates a unique ID for a dag-run using UUID version 7.
func (m *Manager) GenDAGRunID(_ context.Context) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate dag-run ID: %w", err)
	}
	return id.String(), nil
}

// StartDAGRun starts a dag-run by executing the configured executable with the start command.
// It sets up the command to run in its own process group and configures standard output/error.
// If the DAG has resource requirements, they are applied to the entire DAG process tree.
func (m *Manager) StartDAGRun(ctx context.Context, dag *digraph.DAG, opts StartOptions) error {
	args := []string{"start"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, strconv.Quote(opts.Params))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.DAGRunID != "" {
		args = append(args, fmt.Sprintf("--run-id=%s", opts.DAGRunID))
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

	// Apply resource limits if available and configured
	if resourceController := m.getResourceController(ctx); resourceController != nil && dag.Resources != nil {
		// Create resource name using DAG name and run ID
		resourceName := fmt.Sprintf("%s-%s", dag.Name, opts.DAGRunID)
		if opts.DAGRunID == "" {
			resourceName = dag.Name
		}

		logger.Debug(ctx, "Applying resource limits to DAG process",
			"dag", dag.Name,
			"resourceName", resourceName,
			"cpu_limit", dag.Resources.CPULimitMillis,
			"memory_limit", dag.Resources.MemoryLimitBytes)

		// Use ResourceController to start the process with limits
		if err := resourceController.StartProcess(ctx, cmd, dag.Resources, resourceName); err != nil {
			logger.Error(ctx, "Failed to start DAG process with resource limits",
				"dag", dag.Name, "error", err)
			return fmt.Errorf("failed to start DAG with resource limits: %w", err)
		}

		logger.Info(ctx, "Started DAG process with resource enforcement",
			"dag", dag.Name, "pid", cmd.Process.Pid)
		return nil
	}

	// Fallback: start without resource management
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start DAG process: %w", err)
	}

	logger.Info(ctx, "Started DAG process without resource limits",
		"dag", dag.Name, "pid", cmd.Process.Pid)
	return nil
}

// EnqueueDAGRun enqueues a dag-run by executing the configured executable with the enqueue command.
func (m *Manager) EnqueueDAGRun(_ context.Context, dag *digraph.DAG, opts EnqueueOptions) error {
	args := []string{"enqueue"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, strconv.Quote(opts.Params))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.DAGRunID != "" {
		args = append(args, fmt.Sprintf("--run-id=%s", opts.DAGRunID))
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
		return fmt.Errorf("failed to enqueue dag-run: %w", err)
	}
	return nil
}

func (m *Manager) DequeueDAGRun(_ context.Context, dagRun digraph.DAGRunRef) error {
	args := []string{"dequeue", fmt.Sprintf("--dag-run=%s", dagRun.String())}
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
		return fmt.Errorf("failed to dequeue dag-run: %w", err)
	}
	return nil
}

// RestartDAG restarts a DAG by executing the configured executable with the restart command.
// It sets up the command to run in its own process group and applies resource limits if configured.
func (m *Manager) RestartDAG(ctx context.Context, dag *digraph.DAG, opts RestartOptions) error {
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

	// Apply resource limits if available and configured
	if resourceController := m.getResourceController(ctx); resourceController != nil && dag.Resources != nil {
		resourceName := fmt.Sprintf("%s-restart", dag.Name)

		logger.Debug(ctx, "Applying resource limits to restarted DAG process",
			"dag", dag.Name, "resourceName", resourceName)

		if err := resourceController.StartProcess(ctx, cmd, dag.Resources, resourceName); err != nil {
			logger.Error(ctx, "Failed to restart DAG process with resource limits",
				"dag", dag.Name, "error", err)
			return fmt.Errorf("failed to restart DAG with resource limits: %w", err)
		}

		logger.Info(ctx, "Restarted DAG process with resource enforcement",
			"dag", dag.Name, "pid", cmd.Process.Pid)
		return nil
	}

	// Fallback: restart without resource management
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to restart DAG process: %w", err)
	}

	logger.Info(ctx, "Restarted DAG process without resource limits",
		"dag", dag.Name, "pid", cmd.Process.Pid)
	return nil
}

// RetryDAGRun retries a dag-run by executing the configured executable with the retry command.
// It applies resource limits if configured for the DAG.
func (m *Manager) RetryDAGRun(ctx context.Context, dag *digraph.DAG, dagRunID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--run-id=%s", dagRunID))
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

	// Apply resource limits if available and configured
	if resourceController := m.getResourceController(ctx); resourceController != nil && dag.Resources != nil {
		resourceName := fmt.Sprintf("%s-retry-%s", dag.Name, dagRunID)

		logger.Debug(ctx, "Applying resource limits to retried DAG process",
			"dag", dag.Name, "dagRunID", dagRunID, "resourceName", resourceName)

		if err := resourceController.StartProcess(ctx, cmd, dag.Resources, resourceName); err != nil {
			logger.Error(ctx, "Failed to retry DAG process with resource limits",
				"dag", dag.Name, "dagRunID", dagRunID, "error", err)
			return fmt.Errorf("failed to retry DAG with resource limits: %w", err)
		}

		logger.Info(ctx, "Retried DAG process with resource enforcement",
			"dag", dag.Name, "dagRunID", dagRunID, "pid", cmd.Process.Pid)
		return nil
	}

	// Fallback: retry without resource management
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to retry DAG process: %w", err)
	}

	logger.Info(ctx, "Retried DAG process without resource limits",
		"dag", dag.Name, "dagRunID", dagRunID, "pid", cmd.Process.Pid)
	return nil
}

// IsRunning checks if a dag-run is currently running by querying its status.
// Returns true if the status can be retrieved without error, indicating the DAG is running.
func (m *Manager) IsRunning(ctx context.Context, dag *digraph.DAG, dagRunID string) bool {
	status, _ := m.currentStatus(ctx, dag, dagRunID)
	return status != nil && status.DAGRunID == dagRunID && status.Status == scheduler.StatusRunning
}

// GetCurrentStatus retrieves the current status of a dag-run by its run ID.
// If the dag-run is running, it queries the socket for the current status.
// If the socket doesn't exist or times out, it falls back to stored status or creates an initial status.
func (m *Manager) GetCurrentStatus(ctx context.Context, dag *digraph.DAG, dagRunID string) (*models.DAGRunStatus, error) {
	status, err := m.currentStatus(ctx, dag, dagRunID)
	if err != nil {
		goto FALLBACK
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

// GetSavedStatus retrieves the saved status of a dag-run by its digraph.DAGRun reference.
func (m *Manager) GetSavedStatus(ctx context.Context, dagRun digraph.DAGRunRef) (*models.DAGRunStatus, error) {
	attempt, err := m.dagRunStore.FindAttempt(ctx, dagRun)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by run reference: %w", err)
	}
	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return status, nil
}

// getPersistedOrCurrentStatus retrieves the persisted status of a dag-run by its ID.
// If the stored status indicates the DAG is running, it attempts to get the current status.
// If status is running and current status retrieval fails, it marks the status as error.
func (m *Manager) getPersistedOrCurrentStatus(ctx context.Context, dag *digraph.DAG, dagRunID string) (
	*models.DAGRunStatus, error,
) {
	dagRunRef := digraph.NewDAGRunRef(dag.Name, dagRunID)
	attempt, err := m.dagRunStore.FindAttempt(ctx, dagRunRef)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by dag-run ID: %w", err)
	}
	status, err := attempt.ReadStatus(ctx)
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

// FindChildDAGRunStatus retrieves the status of a child dag-run by its ID.
// It looks up the child attempt in the dag-run store and reads its status.
func (m *Manager) FindChildDAGRunStatus(ctx context.Context, rootDAGRun digraph.DAGRunRef, childRunID string) (*models.DAGRunStatus, error) {
	attempt, err := m.dagRunStore.FindChildAttempt(ctx, rootDAGRun, childRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to find child dag-run attempt: %w", err)
	}
	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return status, nil
}

// currentStatus retrieves the current status of a running DAG by querying its socket.
// This is a private method used internally by other status-related methods.
func (*Manager) currentStatus(_ context.Context, dag *digraph.DAG, dagRunID string) (*models.DAGRunStatus, error) {
	client := sock.NewClient(dag.SockAddr(dagRunID))

	// Check if the socket file exists
	if !fileutil.FileExists(client.SocketAddr()) {
		return nil, fmt.Errorf("socket file does not exist for dag-run ID %s", dagRunID)
	}

	statusJSON, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get current status: %w", err)
	}

	return models.StatusFromJSON(statusJSON)
}

// GetLatestStatus retrieves the latest status of a DAG.
// If the DAG is running, it attempts to get the current status from the socket.
// If that fails or no status exists, it returns an initial status or an error.
func (m *Manager) GetLatestStatus(ctx context.Context, dag *digraph.DAG) (models.DAGRunStatus, error) {
	var status *models.DAGRunStatus

	// Find the proc store to check if the DAG is running
	alive, _ := m.procStore.CountAlive(ctx, dag.Name)
	if alive > 0 {
		items, _ := m.dagRunStore.ListStatuses(
			ctx, models.WithName(dag.Name), models.WithStatuses([]scheduler.Status{scheduler.StatusRunning}),
		)
		if len(items) > 0 {
			return *items[0], nil
		}
	}

	// Find the latest status by name
	attempt, err := m.dagRunStore.LatestAttempt(ctx, dag.Name)
	if err != nil {
		goto handleError
	}

	// Read the latest status
	status, err = attempt.ReadStatus(ctx)
	if err != nil {
		goto handleError
	}

	// If the DAG is running, query the current status
	if status.Status == scheduler.StatusRunning {
		dag, err = attempt.ReadDAG(ctx)
		if err != nil {
			currentStatus, err := m.currentStatus(ctx, dag, status.DAGRunID)
			if err == nil {
				status = currentStatus
			} else {
				logger.Debug(ctx, "Failed to get current status from socket", "error", err)
			}
		}
	}

	// If querying the current status fails, ensure if the status is running,
	if status.Status == scheduler.StatusRunning {
		// Check the PID is still alive
		pid := int(status.PID)
		if pid > 0 {
			_, err := os.FindProcess(pid)
			if err != nil {
				// If we cannot find the process, mark the status as error
				status.Status = scheduler.StatusError
				logger.Warn(ctx, "No PID set for running status, marking status as error")
			}
		}
	}

	return *status, nil

handleError:

	// If the latest status is not found, return the default status
	ret := models.InitialStatus(dag)

	return ret, nil
}

// ListRecentStatus retrieves the n most recent statuses for a DAG by name.
// It returns a slice of Status objects, filtering out any that cannot be read.
func (m *Manager) ListRecentStatus(ctx context.Context, name string, n int) []models.DAGRunStatus {
	attempts := m.dagRunStore.RecentAttempts(ctx, name, n)

	var statuses []models.DAGRunStatus
	for _, att := range attempts {
		if status, err := att.ReadStatus(ctx); err == nil {
			statuses = append(statuses, *status)
		}
	}

	return statuses
}

// UpdateStatus updates the status of a dag-run.
func (m *Manager) UpdateStatus(ctx context.Context, rootDAGRun digraph.DAGRunRef, newStatus models.DAGRunStatus) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("update canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Find the attempt for the status.
	var attempt models.DAGRunAttempt

	if rootDAGRun.ID == newStatus.DAGRunID {
		// If the dag-run ID matches the root dag-run ID, find the attempt by the root dag-run ID
		att, err := m.dagRunStore.FindAttempt(ctx, rootDAGRun)
		if err != nil {
			return fmt.Errorf("failed to find the dag-run: %w", err)
		}
		attempt = att
	} else {
		// If the dag-run ID does not match the root dag-run ID,
		// find the attempt by the child dag-run ID
		att, err := m.dagRunStore.FindChildAttempt(ctx, rootDAGRun, newStatus.DAGRunID)
		if err != nil {
			return fmt.Errorf("failed to find child dag-run: %w", err)
		}
		attempt = att
	}

	// Open, write, and close the run
	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("failed to open dag-run data: %w", err)
	}

	// Ensure the run data is closed even if write fails
	defer func() {
		if closeErr := attempt.Close(ctx); closeErr != nil {
			logger.Errorf(ctx, "Failed to close dag-run data: %v", closeErr)
		}
	}()

	if err := attempt.Write(ctx, newStatus); err != nil {
		return fmt.Errorf("failed to write status: %w", err)
	}

	return nil
}

// StartOptions contains options for initiating a dag-run.
type StartOptions struct {
	Params   string // Parameters to pass to the DAG
	Quiet    bool   // Whether to run in quiet mode
	DAGRunID string // ID for the dag-run
}

// EnqueueOptions contains options for enqueuing a dag-run.
type EnqueueOptions struct {
	Params   string // Parameters to pass to the DAG
	Quiet    bool   // Whether to run in quiet mode
	DAGRunID string // ID for the dag-run
}

// RestartOptions contains options for restarting a dag-run.
type RestartOptions struct {
	Quiet bool // Whether to run in quiet mode
}
