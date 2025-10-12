package runtime

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/sock"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/google/uuid"
)

// New creates a new Manager instance.
// The Manager is used to interact with the DAG.
func NewManager(drs execution.DAGRunStore, ps execution.ProcStore, cfg *config.Config) Manager {
	return Manager{
		dagRunStore:   drs,
		procStore:     ps,
		subCmdBuilder: NewSubCmdBuilder(cfg),
	}
}

// Manager provides methods to interact with DAGs, including starting, stopping,
// restarting, and retrieving status information. It communicates with the DAG
// through a socket interface and manages dag-run data.
type Manager struct {
	dagRunStore   execution.DAGRunStore // Store interface for persisting run data
	procStore     execution.ProcStore   // Store interface for process management
	subCmdBuilder *SubCmdBuilder        // Command builder for constructing command specs
}

// Stop stops a running DAG by sending a stop request to its socket.
// If the DAG is not running, it logs a message and returns nil.
func (m *Manager) Stop(ctx context.Context, dag *core.DAG, dagRunID string) error {
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
func (m *Manager) stopSingleDAGRun(ctx context.Context, dag *core.DAG, dagRunID string) error {
	// Check if the process is running using proc store
	alive, err := m.procStore.IsRunAlive(ctx, dag.ProcGroup(), core.NewDAGRunRef(dag.Name, dagRunID))

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
	runRef := core.NewDAGRunRef(dag.Name, dagRunID)
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

// IsRunning checks if a dag-run is currently running by querying its status.
// Returns true if the status can be retrieved without error, indicating the DAG is running.
func (m *Manager) IsRunning(ctx context.Context, dag *core.DAG, dagRunID string) bool {
	st, _ := m.currentStatus(ctx, dag, dagRunID)
	return st != nil && st.DAGRunID == dagRunID && st.Status == status.Running
}

// GetCurrentStatus retrieves the current status of a dag-run by its run ID.
// If the dag-run is running, it queries the socket for the current status.
// If the socket doesn't exist or times out, it falls back to stored status or creates an initial status.
func (m *Manager) GetCurrentStatus(ctx context.Context, dag *core.DAG, dagRunID string) (*execution.DAGRunStatus, error) {
	status, err := m.currentStatus(ctx, dag, dagRunID)
	if err != nil {
		goto FALLBACK
	}
	return status, nil

FALLBACK:
	if dagRunID == "" {
		// The DAG is not running so return the default status
		status := execution.InitialStatus(dag)
		return &status, nil
	}
	return m.getPersistedOrCurrentStatus(ctx, dag, dagRunID)
}

// GetSavedStatus retrieves the saved status of a dag-run by its core.DAGRun reference.
func (m *Manager) GetSavedStatus(ctx context.Context, dagRun core.DAGRunRef) (*execution.DAGRunStatus, error) {
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
func (m *Manager) getPersistedOrCurrentStatus(ctx context.Context, dag *core.DAG, dagRunID string) (
	*execution.DAGRunStatus, error,
) {
	dagRunRef := core.NewDAGRunRef(dag.Name, dagRunID)
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
func (m *Manager) FindChildDAGRunStatus(ctx context.Context, rootDAGRun core.DAGRunRef, childRunID string) (*execution.DAGRunStatus, error) {
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
func (*Manager) currentStatus(_ context.Context, dag *core.DAG, dagRunID string) (*execution.DAGRunStatus, error) {
	client := sock.NewClient(dag.SockAddr(dagRunID))

	// Check if the socket file exists
	if !fileutil.FileExists(client.SocketAddr()) {
		return nil, fmt.Errorf("socket file does not exist for dag-run ID %s", dagRunID)
	}

	statusJSON, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get current status: %w", err)
	}

	return execution.StatusFromJSON(statusJSON)
}

// GetLatestStatus retrieves the latest status of a DAG.
// If the DAG is running, it attempts to get the current status from the socket.
// If that fails or no status exists, it returns an initial status or an error.
func (m *Manager) GetLatestStatus(ctx context.Context, dag *core.DAG) (execution.DAGRunStatus, error) {
	// Find the proc store to check if the DAG is running
	alive, _ := m.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if alive > 0 {
		items, _ := m.dagRunStore.ListStatuses(
			ctx, execution.WithName(dag.Name), execution.WithStatuses([]status.Status{status.Running}),
		)
		if len(items) > 0 {
			return *items[0], nil
		}
	}

	// Find the latest status by name
	attempt, err := m.dagRunStore.LatestAttempt(ctx, dag.Name)
	if err != nil {
		// If the latest status is not found, return the default status
		ret := execution.InitialStatus(dag)
		return ret, nil
	}

	// Read the latest status
	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		// If the latest status is not found, return the default status
		ret := execution.InitialStatus(dag)
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

	return *st, nil
}

// ListRecentStatus retrieves the n most recent statuses for a DAG by name.
// It returns a slice of Status objects, filtering out any that cannot be read.
func (m *Manager) ListRecentStatus(ctx context.Context, name string, n int) []execution.DAGRunStatus {
	attempts := m.dagRunStore.RecentAttempts(ctx, name, n)

	var statuses []execution.DAGRunStatus
	for _, att := range attempts {
		if status, err := att.ReadStatus(ctx); err == nil {
			statuses = append(statuses, *status)
		}
	}

	return statuses
}

// UpdateStatus updates the status of a dag-run.
func (m *Manager) UpdateStatus(ctx context.Context, rootDAGRun core.DAGRunRef, newStatus execution.DAGRunStatus) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("update canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Find the attempt for the status.
	var attempt execution.DAGRunAttempt

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

// checkAndUpdateStaleRunningStatus checks if a running DAG has a live process
// and updates its status to error if the process is not alive.
func (m *Manager) checkAndUpdateStaleRunningStatus(
	ctx context.Context,
	att execution.DAGRunAttempt,
	st *execution.DAGRunStatus,
) error {
	dag, err := att.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG for stale status check: %w", err)
	}
	dagRun := core.DAGRunRef{
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
