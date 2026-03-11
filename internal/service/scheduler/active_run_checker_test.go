// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/dagu-org/dagu/internal/persis/fileproc"
	"github.com/dagu-org/dagu/internal/persis/filequeue"
	"github.com/stretchr/testify/require"
)

func TestActiveRunChecker_QueuedBlockingPolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	candidateTime := now.Add(-10 * time.Minute)

	t.Run("manual queued run does not block", func(t *testing.T) {
		t.Parallel()
		checker, dag, ctx, dagRunStore, queueStore := newActiveRunCheckerFixture(t, now)

		writeQueuedRun(t, ctx, dagRunStore, queueStore, dag, "manual-run", true, func(status *exec.DAGRunStatus) {
			status.TriggerType = core.TriggerTypeManual
			status.ScheduledTime = candidateTime.Format(time.RFC3339)
			status.QueuedAt = candidateTime.Format(time.RFC3339)
		})

		running, err := checker.IsRunning(ctx, dag, core.TriggerTypeScheduler, candidateTime)
		require.NoError(t, err)
		require.False(t, running)
	})

	t.Run("older or same scheduled queued run blocks", func(t *testing.T) {
		t.Parallel()
		checker, dag, ctx, dagRunStore, queueStore := newActiveRunCheckerFixture(t, now)

		writeQueuedRun(t, ctx, dagRunStore, queueStore, dag, "scheduler-run", true, func(status *exec.DAGRunStatus) {
			status.TriggerType = core.TriggerTypeScheduler
			status.ScheduledTime = candidateTime.Add(-time.Minute).Format(time.RFC3339)
			status.QueuedAt = candidateTime.Add(-time.Minute).Format(time.RFC3339)
		})

		running, err := checker.IsRunning(ctx, dag, core.TriggerTypeScheduler, candidateTime)
		require.NoError(t, err)
		require.True(t, running)
	})

	t.Run("newer scheduled queued run does not block older candidate", func(t *testing.T) {
		t.Parallel()
		checker, dag, ctx, dagRunStore, queueStore := newActiveRunCheckerFixture(t, now)

		writeQueuedRun(t, ctx, dagRunStore, queueStore, dag, "catchup-run", true, func(status *exec.DAGRunStatus) {
			status.TriggerType = core.TriggerTypeCatchUp
			status.ScheduledTime = candidateTime.Add(time.Minute).Format(time.RFC3339)
			status.QueuedAt = candidateTime.Add(time.Minute).Format(time.RFC3339)
		})

		running, err := checker.IsRunning(ctx, dag, core.TriggerTypeScheduler, candidateTime)
		require.NoError(t, err)
		require.False(t, running)
	})

	t.Run("legacy queued run without scheduled time falls back to queuedAt", func(t *testing.T) {
		t.Parallel()
		checker, dag, ctx, dagRunStore, queueStore := newActiveRunCheckerFixture(t, now)

		writeQueuedRun(t, ctx, dagRunStore, queueStore, dag, "legacy-run", true, func(status *exec.DAGRunStatus) {
			status.TriggerType = core.TriggerTypeScheduler
			status.ScheduledTime = ""
			status.QueuedAt = candidateTime.Add(-time.Minute).Format(time.RFC3339)
		})

		running, err := checker.IsRunning(ctx, dag, core.TriggerTypeScheduler, candidateTime)
		require.NoError(t, err)
		require.True(t, running)
	})

	t.Run("stale queued run is ignored", func(t *testing.T) {
		t.Parallel()
		checker, dag, ctx, dagRunStore, queueStore := newActiveRunCheckerFixture(t, now)

		writeQueuedRun(t, ctx, dagRunStore, queueStore, dag, "stale-run", true, func(status *exec.DAGRunStatus) {
			status.TriggerType = core.TriggerTypeScheduler
			status.ScheduledTime = now.Add(-queuedRunBlockingTTL - time.Minute).Format(time.RFC3339)
			status.QueuedAt = now.Add(-queuedRunBlockingTTL - time.Minute).Format(time.RFC3339)
		})

		running, err := checker.IsRunning(ctx, dag, core.TriggerTypeScheduler, candidateTime)
		require.NoError(t, err)
		require.False(t, running)
	})

	t.Run("orphan queued status without queue item does not block", func(t *testing.T) {
		t.Parallel()
		checker, dag, ctx, dagRunStore, queueStore := newActiveRunCheckerFixture(t, now)

		writeQueuedRun(t, ctx, dagRunStore, queueStore, dag, "orphan-run", false, func(status *exec.DAGRunStatus) {
			status.TriggerType = core.TriggerTypeScheduler
			status.ScheduledTime = candidateTime.Format(time.RFC3339)
			status.QueuedAt = candidateTime.Format(time.RFC3339)
		})

		running, err := checker.IsRunning(ctx, dag, core.TriggerTypeScheduler, candidateTime)
		require.NoError(t, err)
		require.False(t, running)
	})
}

func newActiveRunCheckerFixture(
	t *testing.T,
	now time.Time,
) (*activeRunChecker, *core.DAG, context.Context, exec.DAGRunStore, exec.QueueStore) {
	t.Helper()

	tmpDir := t.TempDir()
	dagRunStore := filedagrun.New(filepath.Join(tmpDir, "dag-runs"))
	queueStore := filequeue.New(filepath.Join(tmpDir, "queue"))
	procStore := fileproc.New(filepath.Join(tmpDir, "proc"))
	dag := &core.DAG{
		Name:     "critical-dag",
		Location: filepath.Join(tmpDir, "critical-dag.yaml"),
		YamlData: []byte("name: critical-dag\nsteps:\n  - name: step\n    command: echo ok\n"),
		Steps: []core.Step{{
			Name:    "step",
			Command: "echo ok",
		}},
	}

	return newActiveRunChecker(procStore, queueStore, dagRunStore, func() time.Time {
		return now
	}), dag, context.Background(), dagRunStore, queueStore
}

func writeQueuedRun(
	t *testing.T,
	ctx context.Context,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	dag *core.DAG,
	runID string,
	enqueue bool,
	mutate func(*exec.DAGRunStatus),
) {
	t.Helper()

	attempt, err := dagRunStore.CreateAttempt(ctx, dag, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)
	require.NoError(t, attempt.Open(ctx))

	status := exec.InitialStatus(dag)
	status.DAGRunID = runID
	status.Status = core.Queued
	mutate(&status)

	require.NoError(t, attempt.Write(ctx, status))
	require.NoError(t, attempt.Close(ctx))

	if enqueue {
		require.NoError(t, queueStore.Enqueue(ctx, dag.ProcGroup(), exec.QueuePriorityHigh, exec.NewDAGRunRef(dag.Name, runID)))
	}
}
