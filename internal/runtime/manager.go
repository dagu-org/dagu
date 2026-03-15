// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/sock"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/google/uuid"
)

const staleLocalRunStartupGrace = 2 * time.Second

// New creates a new Manager instance.
// The Manager is used to interact with the DAG.
func NewManager(drs exec.DAGRunStore, ps exec.ProcStore, cfg *config.Config) Manager {
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
	dagRunStore   exec.DAGRunStore // Store interface for persisting run data
	procStore     exec.ProcStore   // Store interface for process management
	subCmdBuilder *SubCmdBuilder   // Command builder for constructing command specs
}

// Stop stops a running DAG by sending a stop request to its socket.
// If the DAG is not running, it logs a message and returns nil.
func (m *Manager) Stop(ctx context.Context, dag *core.DAG, dagRunID string) error {
	// Set DAG name in context for all logs in this function
	ctx = logger.WithValues(ctx, tag.Name(dag.Name))
	logger.Info(ctx, "Stopping DAG")

	if dagRunID == "" {
		// If DAGRunID is not specified, stop all matching DAG runs
		// Get the list of running DAG runs for the queue proc name
		aliveRuns, err := m.procStore.ListAlive(ctx, dag.ProcGroup())
		if err != nil {
			return fmt.Errorf("failed to list alive DAG runs: %w", err)
		}

		// If no runs are alive, nothing to stop
		if len(aliveRuns) == 0 {
			logger.Info(ctx, "No running DAG runs found")
			return nil
		}

		// Collect all matching DAG run IDs
		var matchingRunIDs []string
		for _, runRef := range aliveRuns {
			// Find the attempt for this run
			attempt, err := m.dagRunStore.FindAttempt(ctx, runRef)
			if err != nil {
				logger.Warn(ctx, "Failed to find attempt for running DAG",
					slog.String("run-ref", runRef.String()),
					tag.Error(err),
				)
				continue
			}

			// Read the DAG to check if it matches
			runDAG, err := attempt.ReadDAG(ctx)
			if err != nil {
				logger.Warn(ctx, "Failed to read DAG for running attempt",
					slog.String("run-ref", runRef.String()),
					tag.Error(err),
				)
				continue
			}

			// Check if the DAG name matches
			if runDAG.Name == dag.Name {
				matchingRunIDs = append(matchingRunIDs, runRef.ID)
				logger.Info(ctx, "Found matching DAG run to stop",
					tag.RunID(runRef.ID),
				)
			}
		}

		// If no matching DAGs were found
		if len(matchingRunIDs) == 0 {
			logger.Info(ctx, "No matching DAG run found to stop")
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
	// Set run ID in context for all logs in this function
	ctx = logger.WithValues(ctx, tag.RunID(dagRunID))

	// Check if the process is running locally using proc store
	alive, err := m.procStore.IsRunAlive(ctx, dag.ProcGroup(), exec.NewDAGRunRef(dag.Name, dagRunID))
	if err != nil {
		return fmt.Errorf("failed to retrieve status from proc store: %w", err)
	}

	// If running locally, try to stop via socket first
	if alive {
		addr := dag.SockAddr(dagRunID)
		if fileutil.FileExists(addr) {
			// In case the socket exists, we try to send a stop request
			client := sock.NewClient(addr)
			if _, err := client.Request("POST", "/stop"); err == nil {
				logger.Info(ctx, "Successfully stopped DAG via socket")
				return nil
			}
			logger.Debug(ctx, "Socket stop request failed, will use abort flag",
				tag.Error(err))
		}
	}

	// Try to find the dag-run attempt and request cancel (works for both local and distributed runs)
	// This creates an abort flag that the coordinator can detect on heartbeat
	runRef := exec.NewDAGRunRef(dag.Name, dagRunID)
	run, err := m.dagRunStore.FindAttempt(ctx, runRef)
	if err == nil {
		if err := run.Abort(ctx); err != nil {
			return fmt.Errorf("failed to request cancel for dag-run %s: %w", dagRunID, err)
		}
		logger.Info(ctx, "Abort flag created for DAG run")
		return nil
	}

	// If we couldn't find the attempt and the process isn't running locally, nothing to do
	if !alive {
		logger.Info(ctx, "The DAG is not running locally and no attempt found")
		return nil
	}

	// Process is alive but we couldn't find the attempt - this shouldn't happen
	return fmt.Errorf("failed to find dag-run attempt: %w", err)
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
	return st != nil && st.DAGRunID == dagRunID && st.Status == core.Running
}

// GetCurrentStatus retrieves the current status of a dag-run by its run ID.
// If the dag-run is running, it queries the socket for the current status.
// If the socket doesn't exist or times out, it falls back to stored status or creates an initial status.
func (m *Manager) GetCurrentStatus(ctx context.Context, dag *core.DAG, dagRunID string) (*exec.DAGRunStatus, error) {
	status, err := m.currentStatus(ctx, dag, dagRunID)
	if err != nil {
		goto FALLBACK
	}
	return status, nil

FALLBACK:
	if dagRunID == "" {
		// The DAG is not running so return the default status
		return new(exec.InitialStatus(dag)), nil
	}
	return m.getPersistedOrCurrentStatus(ctx, dag, dagRunID)
}

// GetSavedStatus retrieves the saved status of a dag-run by its core.DAGRun reference.
// For stale local runs, it repairs the persisted status before returning it.
func (m *Manager) GetSavedStatus(ctx context.Context, dagRun exec.DAGRunRef) (*exec.DAGRunStatus, error) {
	attempt, err := m.dagRunStore.FindAttempt(ctx, dagRun)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by run reference: %w", err)
	}
	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	if st.Status == core.Running && dagRun.ID == st.DAGRunID {
		dag, dagErr := attempt.ReadDAG(ctx)
		if dagErr != nil {
			logger.Error(ctx, "Failed to read DAG for stale status check", tag.Error(dagErr))
		} else if repaired, repairErr := m.repairStaleLocalRunIfDead(ctx, attempt, dag, st); repairErr != nil {
			logger.Error(ctx, "Failed to repair stale running status", tag.Error(repairErr))
		} else {
			st = repaired
		}
	}

	return st, nil
}

// getPersistedOrCurrentStatus retrieves the persisted status of a dag-run by its ID.
// If the stored status indicates the DAG is running, it attempts to get the current status.
// If current status retrieval fails and the local proc is dead, it repairs the stale run.
func (m *Manager) getPersistedOrCurrentStatus(ctx context.Context, dag *core.DAG, dagRunID string) (
	*exec.DAGRunStatus, error,
) {
	dagRunRef := exec.NewDAGRunRef(dag.Name, dagRunID)
	attempt, err := m.dagRunStore.FindAttempt(ctx, dagRunRef)
	if err != nil {
		return nil, fmt.Errorf("failed to find status by dag-run ID: %w", err)
	}
	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}

	// If the DAG is running, query the current status
	if st.Status == core.Running {
		currentStatus, err := m.currentStatus(ctx, dag, st.DAGRunID)
		if err == nil {
			return currentStatus, nil
		}
		repaired, repairErr := m.repairStaleLocalRunIfDead(ctx, attempt, dag, st)
		if repairErr != nil {
			logger.Error(ctx, "Failed to repair stale running status", tag.Error(repairErr))
		} else {
			st = repaired
		}
	}

	return st, nil
}

// FindSubDAGRunStatus retrieves the status of a sub dag-run by its ID.
// It looks up the child attempt in the dag-run store and reads its status.
func (m *Manager) FindSubDAGRunStatus(ctx context.Context, rootDAGRun exec.DAGRunRef, subRunID string) (*exec.DAGRunStatus, error) {
	attempt, err := m.dagRunStore.FindSubAttempt(ctx, rootDAGRun, subRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to find sub dag-run attempt: %w", err)
	}
	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read status: %w", err)
	}
	return status, nil
}

// currentStatus retrieves the current status of a running DAG by querying its socket.
// This is a private method used internally by other status-related methods.
func (*Manager) currentStatus(_ context.Context, dag *core.DAG, dagRunID string) (*exec.DAGRunStatus, error) {
	client := sock.NewClient(dag.SockAddr(dagRunID))

	// Check if the socket file exists
	if !fileutil.FileExists(client.SocketAddr()) {
		return nil, fmt.Errorf("socket file does not exist for dag-run ID %s", dagRunID)
	}

	statusJSON, err := client.Request("GET", "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to get current status: %w", err)
	}

	return exec.StatusFromJSON(statusJSON)
}

// GetLatestStatus retrieves the latest status of a DAG.
// If the DAG is running, it attempts to get the current status from the socket.
// If that fails and the local proc is dead, it repairs the stale run before returning it.
func (m *Manager) GetLatestStatus(ctx context.Context, dag *core.DAG) (exec.DAGRunStatus, error) {
	// Find the proc store to check if the DAG is running
	alive, _ := m.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if alive > 0 {
		items, _ := m.dagRunStore.ListStatuses(
			ctx, exec.WithName(dag.Name), exec.WithStatuses([]core.Status{core.Running}),
		)
		if len(items) > 0 {
			return *items[0], nil
		}
	}

	// Find the latest status by name
	attempt, err := m.dagRunStore.LatestAttempt(ctx, dag.Name)
	if err != nil {
		// If the latest status is not found, return the default status
		ret := exec.InitialStatus(dag)
		return ret, nil
	}

	// Read the latest status
	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		// If the latest status is not found, return the default status
		ret := exec.InitialStatus(dag)
		return ret, nil
	}

	// If the DAG is running, query the current status
	if st.Status == core.Running {
		runDAG, err := attempt.ReadDAG(ctx)
		if err != nil {
			logger.Debug(ctx, "Failed to read DAG for current status lookup", tag.Error(err))
		} else {
			dag = runDAG
		}
		currentStatus, err := m.currentStatus(ctx, dag, st.DAGRunID)
		if err == nil {
			st = currentStatus
		} else {
			logger.Debug(ctx, "Failed to get current status from socket", tag.Error(err))
			repaired, repairErr := m.repairStaleLocalRunIfDead(ctx, attempt, dag, st)
			if repairErr != nil {
				logger.Error(ctx, "Failed to repair stale running status", tag.Error(repairErr))
			} else {
				st = repaired
			}
		}
	}

	return *st, nil
}

// repairStaleLocalRunIfDead repairs a persisted local Running status only when
// the run has no matching fresh proc heartbeat. "Dead" here means the proc
// store cannot find any non-stale heartbeat file for the local run; it is not
// an OS-level PID liveness check. Distributed runs are excluded because local
// proc heartbeats are not authoritative for remote workers.
func (m *Manager) repairStaleLocalRunIfDead(
	ctx context.Context,
	attempt exec.DAGRunAttempt,
	dag *core.DAG,
	st *exec.DAGRunStatus,
) (*exec.DAGRunStatus, error) {
	if st.WorkerID != "" && st.WorkerID != "local" {
		return st, nil
	}
	if shouldDelayStaleLocalRunRepair(st, time.Now()) {
		logger.Debug(ctx, "Skipping stale local run repair during startup grace window",
			tag.RunID(st.DAGRunID),
			slog.String("started-at", st.StartedAt),
			slog.Duration("grace", staleLocalRunStartupGrace),
		)
		return st, nil
	}

	alive, err := m.procStore.IsRunAlive(ctx, dag.ProcGroup(), exec.DAGRunRef{
		Name: dag.Name,
		ID:   st.DAGRunID,
	})
	if err != nil {
		return nil, fmt.Errorf("check alive: %w", err)
	}
	if alive {
		return st, nil
	}

	repaired, _, err := RepairStaleLocalRun(ctx, attempt, dag)
	if err != nil {
		return nil, fmt.Errorf("repair stale local run: %w", err)
	}
	return repaired, nil
}

func shouldDelayStaleLocalRunRepair(st *exec.DAGRunStatus, now time.Time) bool {
	startedAt, ok := statusStartTime(st)
	if !ok {
		return false
	}
	age := now.Sub(startedAt)
	return age >= 0 && age < staleLocalRunStartupGrace
}

func statusStartTime(st *exec.DAGRunStatus) (time.Time, bool) {
	if startedAt, err := stringutil.ParseTime(st.StartedAt); err == nil && !startedAt.IsZero() {
		return startedAt, true
	}
	if st.CreatedAt > 0 {
		return time.UnixMilli(st.CreatedAt), true
	}
	return time.Time{}, false
}

// ListRecentStatus retrieves the n most recent statuses for a DAG by name.
// It returns a slice of Status objects, filtering out any that cannot be read.
func (m *Manager) ListRecentStatus(ctx context.Context, name string, n int) []exec.DAGRunStatus {
	attempts := m.dagRunStore.RecentAttempts(ctx, name, n)

	var statuses []exec.DAGRunStatus
	for _, att := range attempts {
		if status, err := att.ReadStatus(ctx); err == nil {
			statuses = append(statuses, *status)
		}
	}

	return statuses
}

// UpdateStatus updates the status of a dag-run.
func (m *Manager) UpdateStatus(ctx context.Context, rootDAGRun exec.DAGRunRef, newStatus exec.DAGRunStatus) error {
	// Find the attempt for the status.
	var attempt exec.DAGRunAttempt

	if rootDAGRun.ID == newStatus.DAGRunID {
		// If the dag-run ID matches the root dag-run ID, find the attempt by the root dag-run ID
		att, err := m.dagRunStore.FindAttempt(ctx, rootDAGRun)
		if err != nil {
			return fmt.Errorf("failed to find the dag-run: %w", err)
		}
		attempt = att
	} else {
		// If the dag-run ID does not match the root dag-run ID,
		// find the attempt by the sub dag-run ID
		att, err := m.dagRunStore.FindSubAttempt(ctx, rootDAGRun, newStatus.DAGRunID)
		if err != nil {
			return fmt.Errorf("failed to find sub dag-run: %w", err)
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
				slog.String("err", err.Error()),
				slog.String("err-type", fmt.Sprintf("%T", panicObj)),
				slog.String("stack-trace", string(stack)),
			)
		}
	}()

	// Execute the function
	fn()
}
