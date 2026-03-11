// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

const queuedRunBlockingTTL = 24 * time.Hour

type activeRunChecker struct {
	procStore   exec.ProcStore
	queueStore  exec.QueueStore
	dagRunStore exec.DAGRunStore
	clock       Clock
}

func newActiveRunChecker(
	procStore exec.ProcStore,
	queueStore exec.QueueStore,
	dagRunStore exec.DAGRunStore,
	clock Clock,
) *activeRunChecker {
	checker := &activeRunChecker{
		procStore:   procStore,
		queueStore:  queueStore,
		dagRunStore: dagRunStore,
	}
	checker.SetClock(clock)
	return checker
}

func (c *activeRunChecker) SetClock(clock Clock) {
	if clock == nil {
		c.clock = time.Now
		return
	}
	c.clock = clock
}

func (c *activeRunChecker) IsRunning(
	ctx context.Context,
	dag *core.DAG,
	_ core.TriggerType,
	scheduledTime time.Time,
) (bool, error) {
	count, err := c.procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	items, err := c.queueStore.ListByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return false, fmt.Errorf("failed to list queued runs: %w", err)
	}

	now := c.clock()
	for _, item := range items {
		ref, err := item.Data()
		if err != nil {
			return false, fmt.Errorf("failed to read queued run reference: %w", err)
		}

		attempt, err := c.dagRunStore.FindAttempt(ctx, *ref)
		if errors.Is(err, exec.ErrDAGRunIDNotFound) {
			logger.Warn(ctx, "Ignoring queued item without dag-run attempt",
				tag.DAG(dag.Name),
				tag.RunID(ref.ID),
			)
			continue
		}
		if err != nil {
			return false, fmt.Errorf("failed to find queued attempt %s: %w", ref.String(), err)
		}

		status, err := attempt.ReadStatus(ctx)
		if errors.Is(err, exec.ErrCorruptedStatusFile) {
			logger.Warn(ctx, "Ignoring queued item with corrupted status",
				tag.DAG(dag.Name),
				tag.RunID(ref.ID),
				tag.Error(err),
			)
			continue
		}
		if err != nil {
			return false, fmt.Errorf("failed to read queued status %s: %w", ref.String(), err)
		}
		if status.Status != core.Queued {
			continue
		}
		if queuedStatusBlocks(status, scheduledTime, now) {
			return true, nil
		}
	}

	return false, nil
}

func queuedStatusBlocks(status *exec.DAGRunStatus, scheduledTime, now time.Time) bool {
	if status == nil {
		return false
	}
	if status.TriggerType != core.TriggerTypeScheduler &&
		status.TriggerType != core.TriggerTypeCatchUp {
		return false
	}

	if candidate, ok := blockingReferenceTime(status, now); ok {
		if scheduledTime.IsZero() {
			return true
		}
		return !candidate.After(scheduledTime)
	}

	return false
}

func blockingReferenceTime(status *exec.DAGRunStatus, now time.Time) (time.Time, bool) {
	if status.ScheduledTime != "" {
		if scheduledAt, err := stringutil.ParseTime(status.ScheduledTime); err == nil {
			if scheduledAt.Before(now.Add(-queuedRunBlockingTTL)) {
				return time.Time{}, false
			}
			return scheduledAt, true
		}
	}

	if status.QueuedAt == "" {
		return time.Time{}, false
	}

	queuedAt, err := stringutil.ParseTime(status.QueuedAt)
	if err != nil {
		return time.Time{}, false
	}
	if queuedAt.Before(now.Add(-queuedRunBlockingTTL)) {
		return time.Time{}, false
	}

	return queuedAt, true
}
