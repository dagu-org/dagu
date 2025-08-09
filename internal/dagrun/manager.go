package dagrun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/sock"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/google/uuid"
)

// New creates a new Manager instance.
// The Manager is used to interact with the DAG.
func New(
	drs models.DAGRunStore,
	ps models.ProcStore,
	executable string,
	workDir string,
) Manager {
	var configFile string
	if cfg := config.UsedConfigFile.Load(); cfg != nil {
		if cfgStr, ok := cfg.(string); ok {
			configFile = cfgStr
		}
	}

	return Manager{
		dagRunStore: drs,
		procStore:   ps,
		executable:  executable,
		workDir:     workDir,
		configFile:  configFile,
	}
}

// Manager provides methods to interact with DAGs, including starting, stopping,
// restarting, and retrieving status information. It communicates with the DAG
// through a socket interface and manages dag-run data.
type Manager struct {
	dagRunStore models.DAGRunStore // Store interface for persisting run data
	procStore   models.ProcStore   // Store interface for process management

	executable string // Path to the executable used to run DAGs
	workDir    string // Working directory for executing commands
	configFile string // Path to the config file (if any)
}

// LoadYAML loads a DAG from YAML specification bytes without evaluating it.
// It appends the WithoutEval option to any provided options.
func (m *Manager) LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	opts = append(slices.Clone(opts), digraph.WithoutEval())
	return digraph.LoadYAML(ctx, spec, opts...)
}

// Stop stops a running DAG by sending a stop request to its socket.
// If the DAG is not running, it logs a message and returns nil.
func (m *Manager) Stop(ctx context.Context, dag *digraph.DAG, dagRunID string) error {
	logger.Info(ctx, "Stopping", "name", dag.Name)

	if dagRunID == "" {
		// If DAGRunID is not specified, stop all matching DAG runs
		// Get the list of running DAG runs for the queue proc name
		aliveRuns, err := m.procStore.ListAlive(ctx, dag.ProcGroup())
		if err != nil {
			return fmt.Errorf("failed to list alive DAG runs: %w", err)
		}

		// If no runs are alive, nothing to stop
		if len(aliveRuns) == 0 {
			logger.Info(ctx, "No running DAG runs found", "name", dag.Name)
			return nil
		}

		// Collect all matching DAG run IDs
		var matchingRunIDs []string
		for _, runRef := range aliveRuns {
			// Find the attempt for this run
			attempt, err := m.dagRunStore.FindAttempt(ctx, runRef)
			if err != nil {
				logger.Warn(ctx, "Failed to find attempt for running DAG", "runRef", runRef, "err", err)
				continue
			}

			// Read the DAG to check if it matches
			runDAG, err := attempt.ReadDAG(ctx)
			if err != nil {
				logger.Warn(ctx, "Failed to read DAG for running attempt", "runRef", runRef, "err", err)
				continue
			}

			// Check if the DAG name matches
			if runDAG.Name == dag.Name {
				matchingRunIDs = append(matchingRunIDs, runRef.ID)
				logger.Info(ctx, "Found matching DAG run to stop", "name", dag.Name, "runID", runRef.ID)
			}
		}

		// If no matching DAGs were found
		if len(matchingRunIDs) == 0 {
			logger.Info(ctx, "No matching DAG run found to stop", "name", dag.Name)
			return nil
		}

		// Stop all matching DAG runs
		var stopErrors []error
		for _, runID := range matchingRunIDs {
			if err := m.stopSingleDAGRun(ctx, dag, runID); err != nil {
				stopErrors = append(stopErrors, fmt.Errorf("failed to stop DAG run %s: %w", runID, err))
			}
		}

		// If any errors occurred, return them
		if len(stopErrors) > 0 {
			return fmt.Errorf("errors occurred while stopping DAG runs: %v", stopErrors)
		}

		return nil
	}

	// If dagRunID is specified, stop just that specific run
	return m.stopSingleDAGRun(ctx, dag, dagRunID)
}

// stopSingleDAGRun stops a single DAG run by its ID
func (m *Manager) stopSingleDAGRun(ctx context.Context, dag *digraph.DAG, dagRunID string) error {
	// Check if the process is running using proc store
	alive, err := m.procStore.IsRunAlive(ctx, dag.ProcGroup(), digraph.NewDAGRunRef(dag.Name, dagRunID))
	if err != nil {
		return fmt.Errorf("failed to retrieve status from proc store: %w", err)
	}
	if !alive {
		logger.Info(ctx, "The DAG is not running", "name", dag.Name, "runID", dagRunID)
		return nil
	}

	addr := dag.SockAddr(dagRunID)
	if fileutil.FileExists(addr) {
		// In case the socket exists, we try to send a stop request
		client := sock.NewClient(addr)
		if _, err := client.Request("POST", "/stop"); err == nil {
			logger.Info(ctx, "Successfully stopped DAG via socket", "name", dag.Name, "runID", dagRunID)
			return nil
		}
	}

	// Try to find the running dag-run attempt and request cancel
	runRef := digraph.NewDAGRunRef(dag.Name, dagRunID)
	run, err := m.dagRunStore.FindAttempt(ctx, runRef)
	if err == nil {
		if err := run.RequestCancel(ctx); err != nil {
			return fmt.Errorf("failed to request cancel for dag-run %s: %w", dagRunID, err)
		}
		logger.Info(ctx, "Wrote stop file for running DAG", "name", dag.Name, "runID", dagRunID)
		return nil
	}

	return fmt.Errorf("failed to stop DAG %s (run %s): %w", dag.Name, dagRunID, err)
}

// GenDAGRunID generates a unique ID for a dag-run using UUID version 7.
func (m *Manager) GenDAGRunID(_ context.Context) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate dag-run ID: %w", err)
	}
	return id.String(), nil
}

// StartDAGRunAsync starts a dag-run by executing the configured executable with the start command.
// It sets up the command to run in its own process group and configures standard output/error.
func (m *Manager) StartDAGRunAsync(ctx context.Context, dag *digraph.DAG, opts StartOptions) error {
	args := []string{"start"}
	if opts.Params != "" {
		args = append(args, "-p")
		args = append(args, strconv.Quote(opts.Params))
	}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if opts.Immediate {
		args = append(args, "--no-queue")
	}
	if opts.DAGRunID != "" {
		args = append(args, fmt.Sprintf("--run-id=%s", opts.DAGRunID))
	}
	if m.configFile != "" {
		args = append(args, "--config")
		args = append(args, m.configFile)
	}
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	executor.SetupCommand(cmd)
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start dag-run: %w", err)
	}
	go execWithRecovery(ctx, func() {
		_ = cmd.Wait() // Wait for the command to finish in a goroutine to avoid blocking
	})
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
	if m.configFile != "" {
		args = append(args, "--config")
		args = append(args, m.configFile)
	}
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	executor.SetupCommand(cmd)
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
	if m.configFile != "" {
		args = append(args, "--config")
		args = append(args, m.configFile)
	}
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	executor.SetupCommand(cmd)
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
// It sets up the command to run in its own process group.
func (m *Manager) RestartDAG(ctx context.Context, dag *digraph.DAG, opts RestartOptions) error {
	args := []string{"restart"}
	if opts.Quiet {
		args = append(args, "-q")
	}
	if m.configFile != "" {
		args = append(args, "--config")
		args = append(args, m.configFile)
	}
	args = append(args, dag.Location)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	executor.SetupCommand(cmd)
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start restart command: %w", err)
	}
	go execWithRecovery(ctx, func() {
		_ = cmd.Wait() // Wait for the command to finish in a goroutine to avoid blocking
	})
	return nil
}

// RetryDAGRun retries a dag-run by executing the configured executable with the retry command.
func (m *Manager) RetryDAGRun(ctx context.Context, dag *digraph.DAG, dagRunID string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--run-id=%s", dagRunID))
	return m.runRetryCommand(ctx, args, dag)
}

// RetryDAGStep retries a dag-run from a specific step by executing the configured executable with the retry command and --step flag.
func (m *Manager) RetryDAGStep(ctx context.Context, dag *digraph.DAG, dagRunID string, stepName string) error {
	args := []string{"retry"}
	args = append(args, fmt.Sprintf("--run-id=%s", dagRunID))
	args = append(args, fmt.Sprintf("--step=%s", stepName))
	return m.runRetryCommand(ctx, args, dag)
}

// runRetryCommand builds the full command and starts the process for retrying a dag-run or step.
func (m *Manager) runRetryCommand(ctx context.Context, args []string, dag *digraph.DAG) error {
	if m.configFile != "" {
		args = append(args, "--config")
		args = append(args, m.configFile)
	}
	args = append(args, dag.Name)
	// nolint:gosec
	cmd := exec.Command(m.executable, args...)
	executor.SetupCommand(cmd)
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start retry command: %w", err)
	}
	go execWithRecovery(ctx, func() {
		_ = cmd.Wait() // Wait for the command to finish in a goroutine to avoid blocking
	})
	return nil
}

// IsRunning checks if a dag-run is currently running by querying its status.
// Returns true if the status can be retrieved without error, indicating the DAG is running.
func (m *Manager) IsRunning(ctx context.Context, dag *digraph.DAG, dagRunID string) bool {
	st, _ := m.currentStatus(ctx, dag, dagRunID)
	return st != nil && st.DAGRunID == dagRunID && st.Status == status.Running
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
	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the status is running, ensure if the process is still alive
	if dagRun.ID == st.Root.ID && st.Status == status.Running {
		if err := m.checkAndUpdateStaleRunningStatus(ctx, attempt, st); err != nil {
			logger.Error(ctx, "Failed to check and update stale running status", "err", err)
		}
	}

	return st, nil
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
	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the DAG is running, query the current status
	if st.Status == status.Running {
		currentStatus, err := m.currentStatus(ctx, dag, st.DAGRunID)
		if err == nil {
			return currentStatus, nil
		}
	}

	// If querying the current status fails, even if the status is running,
	// check if the process is actually alive before marking as error.
	if st.Status == status.Running {
		if err := m.checkAndUpdateStaleRunningStatus(ctx, attempt, st); err != nil {
			logger.Error(ctx, "Failed to check and update stale running status", "err", err)
		}
	}

	return st, nil
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
	var dagStatus *models.DAGRunStatus

	// Find the proc store to check if the DAG is running
	alive, _ := m.procStore.CountAlive(ctx, dag.ProcGroup())
	if alive > 0 {
		items, _ := m.dagRunStore.ListStatuses(
			ctx, models.WithName(dag.Name), models.WithStatuses([]status.Status{status.Running}),
		)
		if len(items) > 0 {
			return *items[0], nil
		}
	}

	// Find the latest status by name
	attempt, err := m.dagRunStore.LatestAttempt(ctx, dag.Name)
	if err != nil {
		// If the latest status is not found, return the default status
		ret := models.InitialStatus(dag)
		return ret, nil
	}

	// Read the latest status
	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		// If the latest status is not found, return the default status
		ret := models.InitialStatus(dag)
		return ret, nil
	}

	// If the DAG is running, query the current status
	if st.Status == status.Running {
		dag, err = attempt.ReadDAG(ctx)
		if err != nil {
			currentStatus, err := m.currentStatus(ctx, dag, st.DAGRunID)
			if err == nil {
				st = currentStatus
			} else {
				logger.Debug(ctx, "Failed to get current status from socket", "err", err)
			}
		}
	}

	// If querying the current status fails, ensure if the status is running,
	if st.Status == status.Running {
		if err := m.checkAndUpdateStaleRunningStatus(ctx, attempt, st); err != nil {
			logger.Error(ctx, "Failed to check and update stale running status", "err", err)
		}
	}
	dagStatus = st

	return *dagStatus, nil
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
	Params    string // Parameters to pass to the DAG
	Quiet     bool   // Whether to run in quiet mode
	DAGRunID  string // ID for the dag-run
	Immediate bool   // Start immediately without enqueue
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

// HandleTask executes a DAG run synchronously based on the task information.
// It handles both START (new runs) and RETRY (resume existing runs) operations.
func (m *Manager) HandleTask(ctx context.Context, task *coordinatorv1.Task) error {
	var args []string
	var tempFile string

	// If definition is provided, create a temporary DAG file
	if task.Definition != "" {
		logger.Info(ctx, "Creating temporary DAG file from definition",
			"dagName", task.Target,
			"definitionSize", len(task.Definition))

		tf, err := m.createTempDAGFile(task.Target, []byte(task.Definition))
		if err != nil {
			return fmt.Errorf("failed to create temp DAG file: %w", err)
		}
		tempFile = tf
		defer func() {
			// Clean up the temporary file
			if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
				logger.Errorf(ctx, "Failed to remove temp DAG file: %v", err)
			}
		}()
		// Update the target to use the temp file
		originalTarget := task.Target
		task.Target = tempFile

		logger.Info(ctx, "Created temporary DAG file",
			"tempFile", tempFile,
			"originalTarget", originalTarget)
	}

	switch task.Operation {
	case coordinatorv1.Operation_OPERATION_START:
		args = m.buildStartCommand(task)
	case coordinatorv1.Operation_OPERATION_RETRY:
		args = m.buildRetryCommand(task)
	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return fmt.Errorf("operation not specified")
	default:
		return fmt.Errorf("unknown operation: %v", task.Operation)
	}

	// Execute synchronously
	// nolint:gosec
	cmd := exec.CommandContext(ctx, m.executable, args...)
	executor.SetupCommand(cmd)
	cmd.Dir = m.workDir
	cmd.Env = os.Environ()

	// Execute and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v failed: %w\noutput: %s", task.Operation, err, output)
	}

	return nil
}

// buildStartCommand builds the command arguments for starting a new DAG run.
func (m *Manager) buildStartCommand(task *coordinatorv1.Task) []string {
	args := []string{"start"}

	// Add hierarchy flags for child DAGs
	if task.RootDagRunId != "" {
		args = append(args, fmt.Sprintf("--root=%s:%s", task.RootDagRunName, task.RootDagRunId))
	}
	if task.ParentDagRunId != "" {
		args = append(args, fmt.Sprintf("--parent=%s:%s", task.ParentDagRunName, task.ParentDagRunId))
	}

	args = append(args,
		fmt.Sprintf("--run-id=%s", task.DagRunId),
		"--no-queue", // Always bypass queue for worker execution
	)

	m.addConfigFlag(&args)
	args = append(args, task.Target)

	if task.Params != "" {
		args = append(args, "--", task.Params)
	}

	return args
}

// buildRetryCommand builds the command arguments for retrying an existing DAG run.
func (m *Manager) buildRetryCommand(task *coordinatorv1.Task) []string {
	args := []string{"retry", fmt.Sprintf("--run-id=%s", task.DagRunId)}

	if task.Step != "" {
		args = append(args, fmt.Sprintf("--step=%s", task.Step))
	}

	m.addConfigFlag(&args)
	args = append(args, task.Target) // DAG name

	return args
}

// addConfigFlag adds the config file flag if one is set.
func (m *Manager) addConfigFlag(args *[]string) {
	if m.configFile != "" {
		*args = append(*args, "--config", m.configFile)
	}
}

// execWithRecovery executes a function with panic recovery and detailed error reporting
// It captures stack traces and provides structured error information for debugging
func execWithRecovery(ctx context.Context, fn func()) {
	defer func() {
		if panicObj := recover(); panicObj != nil {
			stack := debug.Stack()

			// Convert panic object to error
			var err error
			switch v := panicObj.(type) {
			case error:
				err = v
			case string:
				err = fmt.Errorf("panic: %s", v)
			default:
				err = fmt.Errorf("panic: %v", v)
			}

			// Log with structured information
			logger.Error(ctx, "Recovered from panic",
				"err", err.Error(),
				"errType", fmt.Sprintf("%T", panicObj),
				"stackTrace", stack,
				"fullStack", string(stack))
		}
	}()

	// Execute the function
	fn()
}

// createTempDAGFile creates a temporary file with the DAG definition content.
func (m *Manager) createTempDAGFile(dagName string, yamlData []byte) (string, error) {
	// Create a temporary directory if it doesn't exist
	tempDir := filepath.Join(os.TempDir(), "dagu", "worker-dags")
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create a temporary file with a meaningful name
	pattern := fmt.Sprintf("%s-*.yaml", dagName)
	tempFile, err := os.CreateTemp(tempDir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
	}()

	// Write the YAML data
	if _, err := tempFile.Write(yamlData); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write YAML data: %w", err)
	}

	return tempFile.Name(), nil
}

// checkAndUpdateStaleRunningStatus checks if a running DAG has a live process
// and updates its status to error if the process is not alive.
func (m *Manager) checkAndUpdateStaleRunningStatus(
	ctx context.Context,
	att models.DAGRunAttempt,
	st *models.DAGRunStatus,
) error {
	dag, err := att.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG for stale status check: %w", err)
	}
	dagRun := digraph.DAGRunRef{
		Name: dag.Name,
		ID:   st.DAGRunID,
	}
	alive, err := m.procStore.IsRunAlive(ctx, dag.ProcGroup(), dagRun)
	if err != nil {
		// Log but don't fail - we can't determine if it's alive
		logger.Error(ctx, "Failed to check if DAG run is alive", "err", err)
		return nil
	}
	if alive {
		// Process is still alive, nothing to do
		return nil
	}
	// Process is not alive, update status to error
	st.Status = status.Error

	return nil
}
