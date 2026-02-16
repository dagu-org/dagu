package exec

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
)

// EnqueueRetry enqueues a DAG run for retry and persists the Queued status.
// It persists the Queued status first, then enqueues, so the queue processor
// always sees the correct status when it picks up the item. If enqueue fails,
// the status is rolled back. Retries respect global queue capacity because
// the queue processor picks them up when capacity is available.
func EnqueueRetry(
	ctx context.Context,
	queueStore QueueStore,
	attempt DAGRunAttempt,
	dag *core.DAG,
	status *DAGRunStatus,
	dagRunID string,
) error {
	if status.Status == core.Queued {
		// Already queued (e.g. duplicate retry), treat as success
		return nil
	}
	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("open attempt: %w", err)
	}
	defer func() { _ = attempt.Close(ctx) }()

	// Snapshot original values for rollback
	origStatus := status.Status
	origQueuedAt := status.QueuedAt
	origTriggerType := status.TriggerType

	// Persist Queued status FIRST so the queue processor always sees
	// the correct status when it picks up the item.
	status.Status = core.Queued
	status.QueuedAt = stringutil.FormatTime(time.Now())
	status.TriggerType = core.TriggerTypeRetry
	if err := attempt.Write(ctx, *status); err != nil {
		status.Status = origStatus
		status.QueuedAt = origQueuedAt
		status.TriggerType = origTriggerType
		return fmt.Errorf("write status: %w", err)
	}

	// Enqueue after status is persisted. If this fails, roll back the status.
	dagRun := NewDAGRunRef(dag.Name, dagRunID)
	if err := queueStore.Enqueue(ctx, dag.ProcGroup(), QueuePriorityLow, dagRun); err != nil {
		status.Status = origStatus
		status.QueuedAt = origQueuedAt
		status.TriggerType = origTriggerType
		_ = attempt.Write(ctx, *status) // best-effort rollback
		return fmt.Errorf("enqueue retry: %w", err)
	}

	return nil
}
