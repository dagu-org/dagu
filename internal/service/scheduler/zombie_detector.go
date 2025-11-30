package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// ZombieDetector finds and cleans up zombie DAG runs
type ZombieDetector struct {
	dagRunStore execution.DAGRunStore
	procStore   execution.ProcStore
	interval    time.Duration
	quit        chan struct{}
	closeOnce   sync.Once
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
		quit:        make(chan struct{}),
	}
}

// Start begins the zombie detection loop
func (z *ZombieDetector) Start(ctx context.Context) {
	var running atomic.Bool
	ticker := time.NewTicker(z.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !running.CompareAndSwap(false, true) {
				logger.Warn(ctx, "Skipping zombie detection, previous check still running")
				continue
			}

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer running.Store(false)
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						var err error
						switch v := r.(type) {
						case error:
							err = v
						case string:
							err = fmt.Errorf("%s", v)
						default:
							err = fmt.Errorf("%v", v)
						}
						logger.Error(ctx, "Zombie detection check panicked", tag.Error(err))
					}
				}()
				z.detectAndCleanZombies(ctx)
			}()
			wg.Wait()

		case <-z.quit:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (z *ZombieDetector) Stop(ctx context.Context) {
	logger.Info(ctx, "Stopping zombie detector")
	z.closeOnce.Do(func() {
		close(z.quit)
	})
}

// detectAndCleanZombies finds all running DAG runs and checks if they're actually alive
func (z *ZombieDetector) detectAndCleanZombies(ctx context.Context) {
	// Query all running DAG runs
	statuses, err := z.dagRunStore.ListStatuses(ctx,
		execution.WithStatuses([]core.Status{core.Running}))
	if err != nil {
		logger.Error(ctx, "Failed to list running DAG runs", tag.Error(err))
		return
	}

	logger.Debug(ctx, "Checking for zombie DAG runs", tag.Count(len(statuses)))

	for _, st := range statuses {
		// Check for quit signal
		select {
		case <-ctx.Done():
			return
		case <-z.quit:
			return
		default:
		}

		if err := z.checkAndCleanZombie(ctx, st); err != nil {
			logger.Error(ctx, "Failed to check zombie status",
				tag.Name(st.Name),
				tag.RunID(st.DAGRunID),
				tag.Error(err))
		}
	}
}

// checkAndCleanZombie checks if a single DAG run is a zombie and cleans it up
func (z *ZombieDetector) checkAndCleanZombie(ctx context.Context, st *execution.DAGRunStatus) error {
	ctx = logger.WithValues(ctx,
		tag.DAG(st.Name),
		tag.RunID(st.DAGRunID),
	)

	// Find the attempt for this status
	dagRunRef := execution.NewDAGRunRef(st.Name, st.DAGRunID)
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
	alive, err := z.procStore.IsRunAlive(ctx, dag.ProcGroup(), execution.DAGRunRef{Name: dag.Name, ID: st.DAGRunID})
	if err != nil {
		logger.Warn(ctx, "Failed to check process liveness for dag-run",
			tag.Error(err),
			tag.Queue(dag.ProcGroup()),
		)
		return fmt.Errorf("check alive: %w", err)
	}

	if alive {
		logger.Debug(ctx, "Dag-run heartbeat detected; skipping zombie cleanup",
			tag.Queue(dag.ProcGroup()),
		)
		return nil
	}

	// Process is zombie, update status to error
	logger.Info(ctx, "Found zombie DAG run, updating to error status",
		tag.Queue(dag.ProcGroup()),
	)

	// Update the status to error
	st.Status = core.Failed
	st.FinishedAt = time.Now().Format(time.RFC3339)
	for _, n := range st.Nodes {
		if n.Status == core.NodeRunning {
			n.Status = core.NodeFailed
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
			logger.Error(ctx, "Failed to close attempt", tag.Error(err))
		}
	}()

	if err := attempt.Write(ctx, status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}
