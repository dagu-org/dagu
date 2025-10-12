package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/status"
)

// ZombieDetector finds and cleans up zombie DAG runs
type ZombieDetector struct {
	dagRunStore execution.DAGRunStore
	procStore   execution.ProcStore
	interval    time.Duration
}

// NewZombieDetector creates a new zombie detector
func NewZombieDetector(
	dagRunStore execution.DAGRunStore,
	procStore execution.ProcStore,
	interval time.Duration,
) *ZombieDetector {
	if interval <= 0 {
		interval = 45 * time.Second
	}
	return &ZombieDetector{
		dagRunStore: dagRunStore,
		procStore:   procStore,
		interval:    interval,
	}
}

// Start begins the zombie detection loop
func (z *ZombieDetector) Start(ctx context.Context) {
	ticker := time.NewTicker(z.interval)
	defer ticker.Stop()

	running := atomic.Bool{}

	for {
		select {
		case <-ticker.C:
			if !running.CompareAndSwap(false, true) {
				logger.Warn(ctx, "Skipping zombie detection, previous check still running")
				continue
			}

			go func() {
				defer running.Store(false)
				defer func() {
					if r := recover(); r != nil {
						logger.Error(ctx, "Zombie detection check panicked", "panic", r)
					}
				}()
				z.detectAndCleanZombies(ctx)
			}()

		case <-ctx.Done():
			logger.Info(ctx, "Stopping zombie detector")
			return
		}
	}
}

// detectAndCleanZombies finds all running DAG runs and checks if they're actually alive
func (z *ZombieDetector) detectAndCleanZombies(ctx context.Context) {
	// Query all running DAG runs
	statuses, err := z.dagRunStore.ListStatuses(ctx,
		execution.WithStatuses([]status.Status{status.Running}))
	if err != nil {
		logger.Error(ctx, "Failed to list running DAG runs", "err", err)
		return
	}

	logger.Debug(ctx, "Checking for zombie DAG runs", "count", len(statuses))

	for _, st := range statuses {
		if err := z.checkAndCleanZombie(ctx, st); err != nil {
			logger.Error(ctx, "Failed to check zombie status",
				"name", st.Name, "dagRunID", st.DAGRunID, "err", err)
		}
	}
}

// checkAndCleanZombie checks if a single DAG run is a zombie and cleans it up
func (z *ZombieDetector) checkAndCleanZombie(ctx context.Context, st *execution.DAGRunStatus) error {
	// Find the attempt for this status
	dagRunRef := core.NewDAGRunRef(st.Name, st.DAGRunID)
	attempt, err := z.dagRunStore.FindAttempt(ctx, dagRunRef)
	if err != nil {
		return fmt.Errorf("find attempt: %w", err)
	}

	// Read the DAG to get queue proc name
	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("read dag: %w", err)
	}

	// Check if process is alive
	alive, err := z.procStore.IsRunAlive(ctx, dag.ProcGroup(), core.DAGRunRef{Name: dag.Name, ID: st.DAGRunID})
	if err != nil {
		return fmt.Errorf("check alive: %w", err)
	}

	if alive {
		return nil
	}

	// Process is zombie, update status to error
	logger.Info(ctx, "Found zombie DAG run, updating to error status",
		"name", st.Name, "dagRunID", st.DAGRunID)

	// Update the status to error
	st.Status = status.Error
	st.FinishedAt = time.Now().Format(time.RFC3339)
	for _, n := range st.Nodes {
		if n.Status == status.NodeRunning {
			n.Status = status.NodeError
			n.Error = "process terminated unexpectedly - zombie process detected"
		}
	}

	if err := z.updateStatus(ctx, attempt, *st); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

// updateStatus updates the status of a DAG run attempt
func (z *ZombieDetector) updateStatus(ctx context.Context,
	attempt execution.DAGRunAttempt, status execution.DAGRunStatus) error {
	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("open attempt: %w", err)
	}
	defer func() {
		if err := attempt.Close(ctx); err != nil {
			logger.Error(ctx, "Failed to close attempt", "err", err)
		}
	}()

	if err := attempt.Write(ctx, status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}
