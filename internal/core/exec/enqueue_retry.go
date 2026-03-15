// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
)

// EnqueueRetryOptions control how queued retry metadata is persisted.
type EnqueueRetryOptions struct {
	// AutoRetry marks scheduler-issued DAG auto-retries. These consume the
	// DAG-level retry budget at enqueue time.
	AutoRetry bool
}

// EnqueueRetryOutcome describes the final state of a retry enqueue attempt.
type EnqueueRetryOutcome string

const (
	EnqueueRetryOutcomeQueued        EnqueueRetryOutcome = "queued"
	EnqueueRetryOutcomeAlreadyQueued EnqueueRetryOutcome = "already_queued"
	EnqueueRetryOutcomeStaleLatest   EnqueueRetryOutcome = "stale_latest"
)

// EnqueueRetryResult reports the persisted status observed at the end of an
// enqueue attempt.
type EnqueueRetryResult struct {
	Outcome EnqueueRetryOutcome
	Status  *DAGRunStatus
}

// EnqueueRetry enqueues a DAG run for retry and persists the Queued status.
// It persists the Queued status first, then enqueues, so the queue processor
// always sees the correct status when it picks up the item. If enqueue fails,
// the status is rolled back. Retries respect global queue capacity because
// the queue processor picks them up when capacity is available.
func EnqueueRetry(
	ctx context.Context,
	dagRunStore DAGRunStore,
	queueStore QueueStore,
	dag *core.DAG,
	status *DAGRunStatus,
	opts EnqueueRetryOptions,
) (EnqueueRetryResult, error) {
	if status.Status == core.Queued {
		// Already queued (e.g. duplicate retry), treat as success
		return EnqueueRetryResult{
			Outcome: EnqueueRetryOutcomeAlreadyQueued,
			Status:  status,
		}, nil
	}

	updatedStatus, swapped, err := dagRunStore.CompareAndSwapLatestAttemptStatus(
		ctx,
		status.DAGRun(),
		status.AttemptID,
		status.Status,
		func(latest *DAGRunStatus) error {
			latest.Status = core.Queued
			latest.QueuedAt = stringutil.FormatTime(time.Now())
			latest.TriggerType = core.TriggerTypeRetry
			if opts.AutoRetry {
				latest.AutoRetryCount++
			}
			return nil
		},
	)
	if err != nil {
		return EnqueueRetryResult{}, fmt.Errorf("persist queued retry status: %w", err)
	}
	if !swapped {
		// Another actor changed the latest attempt first. Treat an already queued
		// retry as success; everything else is a no-op.
		if updatedStatus != nil && updatedStatus.Status == core.Queued {
			return EnqueueRetryResult{
				Outcome: EnqueueRetryOutcomeAlreadyQueued,
				Status:  updatedStatus,
			}, nil
		}
		return EnqueueRetryResult{
			Outcome: EnqueueRetryOutcomeStaleLatest,
			Status:  updatedStatus,
		}, nil
	}

	// Enqueue after status is persisted. If this fails, roll back the status.
	dagRun := status.DAGRun()
	if err := queueStore.Enqueue(ctx, dag.ProcGroup(), QueuePriorityLow, dagRun); err != nil {
		_, _, _ = dagRunStore.CompareAndSwapLatestAttemptStatus(
			ctx,
			dagRun,
			updatedStatus.AttemptID,
			core.Queued,
			func(latest *DAGRunStatus) error {
				latest.Status = status.Status
				latest.QueuedAt = status.QueuedAt
				latest.TriggerType = status.TriggerType
				latest.AutoRetryCount = status.AutoRetryCount
				return nil
			},
		)
		return EnqueueRetryResult{}, fmt.Errorf("enqueue retry: %w", err)
	}

	return EnqueueRetryResult{
		Outcome: EnqueueRetryOutcomeQueued,
		Status:  updatedStatus,
	}, nil
}
