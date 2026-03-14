// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// panicToError converts a panic value to an error, including stack trace.
func panicToError(r any) error {
	stack := string(debug.Stack())
	if err, ok := r.(error); ok {
		return fmt.Errorf("panic: %w\n%s", err, stack)
	}
	return fmt.Errorf("panic: %v\n%s", r, stack)
}

// ZombieDetector finds and cleans up zombie DAG runs
type ZombieDetector struct {
	dagRunStore      exec.DAGRunStore
	procStore        exec.ProcStore
	interval         time.Duration
	failureThreshold int
	staleCounters    map[string]int // dagRunID -> consecutive stale count
	runMutexesMu     sync.Mutex
	runMutexes       map[string]*sync.Mutex
	quit             chan struct{}
	closeOnce        sync.Once
}

// NewZombieDetector creates a new zombie detector
func NewZombieDetector(
	dagRunStore exec.DAGRunStore,
	procStore exec.ProcStore,
	interval time.Duration,
	failureThreshold int,
) *ZombieDetector {
	if interval <= 0 {
		interval = 45 * time.Second
	}
	if failureThreshold <= 0 {
		failureThreshold = 3
	}
	return &ZombieDetector{
		dagRunStore:      dagRunStore,
		procStore:        procStore,
		interval:         interval,
		failureThreshold: failureThreshold,
		staleCounters:    make(map[string]int),
		runMutexes:       make(map[string]*sync.Mutex),
		quit:             make(chan struct{}),
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
						logger.Error(ctx, "Zombie detection check panicked", tag.Error(panicToError(r)))
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

// getRunMutex returns or creates a per-run mutex for serializing status access.
func (z *ZombieDetector) getRunMutex(dagRunID string) *sync.Mutex {
	z.runMutexesMu.Lock()
	defer z.runMutexesMu.Unlock()

	if mu, ok := z.runMutexes[dagRunID]; ok {
		return mu
	}

	mu := &sync.Mutex{}
	z.runMutexes[dagRunID] = mu
	return mu
}

// detectAndCleanZombies finds all running DAG runs and checks if they're actually alive
func (z *ZombieDetector) detectAndCleanZombies(ctx context.Context) {
	// Query all running DAG runs
	statuses, err := z.dagRunStore.ListStatuses(ctx,
		exec.WithStatuses([]core.Status{core.Running}))
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

	// Prune stale counters for runs no longer in Running state
	runningIDs := make(map[string]struct{}, len(statuses))
	for _, st := range statuses {
		runningIDs[st.DAGRunID] = struct{}{}
	}
	for id := range z.staleCounters {
		if _, ok := runningIDs[id]; !ok {
			delete(z.staleCounters, id)
		}
	}
}

// checkAndCleanZombie checks if a single DAG run is a zombie and cleans it up
func (z *ZombieDetector) checkAndCleanZombie(ctx context.Context, st *exec.DAGRunStatus) error {
	ctx = logger.WithValues(ctx,
		tag.DAG(st.Name),
		tag.RunID(st.DAGRunID),
	)

	// Find the attempt for this status
	dagRunRef := exec.NewDAGRunRef(st.Name, st.DAGRunID)
	attempt, err := z.dagRunStore.FindAttempt(ctx, dagRunRef)
	if err != nil {
		return fmt.Errorf("find attempt: %w", err)
	}

	// Read the DAG to get queue proc name
	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("read dag: %w", err)
	}

	// Skip zombie detection for distributed runs - they're monitored by coordinator's heartbeat detector
	if st.WorkerID != "" && st.WorkerID != "local" {
		logger.Debug(ctx, "Skipping zombie detection for distributed run",
			tag.Queue(dag.ProcGroup()),
			tag.WorkerID(st.WorkerID),
		)
		return nil
	}

	// Check if process is alive (only for local runs)
	alive, err := z.procStore.IsRunAlive(ctx, dag.ProcGroup(), exec.DAGRunRef{Name: dag.Name, ID: st.DAGRunID})
	if err != nil {
		logger.Warn(ctx, "Failed to check process liveness for dag-run",
			tag.Error(err),
			tag.Queue(dag.ProcGroup()),
		)
		return fmt.Errorf("check alive: %w", err)
	}

	if alive {
		// Reset consecutive stale counter — run is healthy
		delete(z.staleCounters, st.DAGRunID)
		logger.Debug(ctx, "Dag-run heartbeat detected; skipping zombie cleanup",
			tag.Queue(dag.ProcGroup()),
		)
		return nil
	}

	// Increment consecutive stale counter
	z.staleCounters[st.DAGRunID]++
	count := z.staleCounters[st.DAGRunID]

	if count < z.failureThreshold {
		logger.Warn(ctx, "DAG run appears stale, waiting for threshold",
			tag.Queue(dag.ProcGroup()),
			slog.Int("stale_count", count),
			slog.Int("threshold", z.failureThreshold),
		)
		return nil
	}

	// Threshold reached — acquire per-run mutex and proceed with kill
	runMu := z.getRunMutex(st.DAGRunID)
	runMu.Lock()
	defer runMu.Unlock()

	// Read the full status from the attempt rather than using the summary
	// from ListStatuses, which lacks node data. This ensures we preserve
	// the complete status (nodes, logs, params, etc.) when updating.
	fullStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("read full status: %w", err)
	}

	// Compare-and-set guard: verify status is still active before writing Failed.
	// If the run completed (Success/Cancelled/Failed) between ListStatuses and now,
	// we must not overwrite the terminal status.
	if !fullStatus.Status.IsActive() {
		logger.Info(ctx, "Run already in terminal state, skipping zombie cleanup",
			tag.Queue(dag.ProcGroup()),
			slog.String("status", fullStatus.Status.String()),
		)
		delete(z.staleCounters, st.DAGRunID)
		return nil
	}

	// Confirmed zombie — update status to Failed
	logger.Info(ctx, "Confirmed zombie DAG run, updating to error status",
		tag.Queue(dag.ProcGroup()),
		slog.Int("stale_count", count),
	)

	fullStatus.Status = core.Failed
	fullStatus.FinishedAt = time.Now().Format(time.RFC3339)

	// If the process was killed before writing node data (e.g., SIGKILL before
	// the initial 100ms status write), populate nodes from the DAG definition
	// so the UI shows step names instead of "0/0 Log".
	if len(fullStatus.Nodes) == 0 {
		fullStatus.Nodes = exec.NewNodesFromSteps(dag.Steps)
	}

	for _, n := range fullStatus.Nodes {
		if n.Status == core.NodeRunning || n.Status == core.NodeNotStarted {
			n.Status = core.NodeFailed
			n.Error = "process terminated unexpectedly - zombie process detected"
		}
	}

	if err := z.updateStatus(ctx, attempt, *fullStatus); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Clean up stale proc files after successfully persisting Failed status
	_ = z.procStore.CleanStaleFiles(ctx, dag.ProcGroup())

	// Clean up counter entry
	delete(z.staleCounters, st.DAGRunID)

	return nil
}

// updateStatus updates the status of a DAG run attempt
func (z *ZombieDetector) updateStatus(ctx context.Context,
	attempt exec.DAGRunAttempt, status exec.DAGRunStatus) error {
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
