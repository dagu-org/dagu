package exec

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
)

// EnqueueRetry enqueues a DAG run for retry and persists the Queued status.
// It enqueues first, then persists status, so a failed enqueue never leaves
// an orphaned Queued status. Retries respect global queue capacity because
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

	// Enqueue first; if this fails, no status change is persisted
	dagRun := NewDAGRunRef(dag.Name, dagRunID)
	if err := queueStore.Enqueue(ctx, dag.ProcGroup(), QueuePriorityLow, dagRun); err != nil {
		return fmt.Errorf("enqueue retry: %w", err)
	}

	// Only after successful enqueue, persist the Queued status
	status.Status = core.Queued
	status.QueuedAt = stringutil.FormatTime(time.Now())
	status.TriggerType = core.TriggerTypeRetry
	if err := attempt.Write(ctx, *status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}
	return nil
}
