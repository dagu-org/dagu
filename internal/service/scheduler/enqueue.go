// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/transform"
)

// EnqueueCatchupRun enqueues a catchup run for a DAG.
//
// The function is idempotent: if a run with the same ID already exists
// (checked via FindAttempt), it returns nil without creating a duplicate.
//
// On failure after CreateAttempt but before Enqueue, the orphaned attempt
// record is cleaned up via RemoveDAGRun.
//
// The DAG is shallow-copied to avoid mutating the shared planner entry
// (Location is cleared to prevent unix pipe conflicts for concurrent runs).
func EnqueueCatchupRun(
	ctx context.Context,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	dag *core.DAG,
	runID string,
	triggerType core.TriggerType,
	scheduleTime time.Time,
) error {
	dagRun := exec.NewDAGRunRef(dag.Name, runID)

	// Idempotency: skip if a run with this ID already exists.
	if _, err := dagRunStore.FindAttempt(ctx, dagRun); err == nil {
		logger.Info(ctx, "Catchup run already exists; skipping",
			tag.DAG(dag.Name),
			tag.RunID(runID),
		)
		return nil
	}

	// Clone to avoid mutating the shared planner entry.
	// Location is cleared to prevent unix pipe conflicts for concurrent runs
	// (same as cmd/enqueue.go:87).
	dagCopy := dag.Clone()
	dagCopy.Location = ""

	att, err := dagRunStore.CreateAttempt(ctx, dagCopy, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
	if err != nil {
		return fmt.Errorf("failed to create catchup attempt: %w", err)
	}

	// Rollback the attempt on any failure after creation. Without this,
	// an orphaned attempt would block all future retries for this run ID
	// because FindAttempt would find it and skip.
	committed := false
	defer func() {
		if committed {
			return
		}
		if rmErr := dagRunStore.RemoveDAGRun(ctx, dagRun); rmErr != nil {
			logger.Error(ctx, "Failed to rollback catchup attempt",
				tag.DAG(dag.Name),
				tag.RunID(runID),
				tag.Error(rmErr),
			)
		}
	}()

	opts := []transform.StatusOption{
		transform.WithAttemptID(att.ID()),
		transform.WithPreconditions(dagCopy.Preconditions),
		transform.WithQueuedAt(stringutil.FormatTime(time.Now())),
		transform.WithHierarchyRefs(dagRun, exec.DAGRunRef{}),
		transform.WithTriggerType(triggerType),
		transform.WithScheduleTime(stringutil.FormatTime(scheduleTime)),
	}

	dagStatus := transform.NewStatusBuilder(dagCopy).Create(runID, core.Queued, 0, time.Time{}, opts...)

	if err := att.Open(ctx); err != nil {
		return fmt.Errorf("failed to open catchup attempt: %w", err)
	}

	if err := att.Write(ctx, dagStatus); err != nil {
		_ = att.Close(ctx)
		return fmt.Errorf("failed to write catchup status: %w", err)
	}

	if err := att.Close(ctx); err != nil {
		return fmt.Errorf("failed to close catchup attempt: %w", err)
	}

	if err := queueStore.Enqueue(ctx, dagCopy.ProcGroup(), exec.QueuePriorityLow, dagRun); err != nil {
		return fmt.Errorf("failed to enqueue catchup run: %w", err)
	}

	committed = true

	logger.Info(ctx, "Catchup run enqueued",
		tag.DAG(dag.Name),
		tag.RunID(runID),
	)

	return nil
}
