// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package queue_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicProcessing(t *testing.T) {
	f := newFixture(t, `
name: echo-dag
steps:
  - name: echo
    command: echo hello
`).Enqueue(3).StartScheduler(30 * time.Second)

	f.WaitDrain(25 * time.Second)
	f.Stop()

	items, err := f.th.QueueStore.List(f.th.Context, f.queue)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestGlobalConcurrency(t *testing.T) {
	// This test uses a global queue with maxConcurrency=3 to verify concurrent execution.
	// Note: maxActiveRuns at DAG level is deprecated and intentionally omitted.
	f := newFixture(t, `
name: sleep-dag
queue: global-queue
steps:
  - name: sleep
    command: sleep 1
`, WithQueue("global-queue"), WithGlobalQueue("global-queue", 3)).
		Enqueue(3).StartScheduler(30 * time.Second)

	f.WaitDrain(25 * time.Second)
	f.Stop()
	f.AssertConcurrent(2 * time.Second)
}

func TestLocalQueueFIFOProcessing(t *testing.T) {
	// Local queues always use maxConcurrency=1 (FIFO), ignoring DAG's maxActiveRuns.
	// This verifies that even with maxActiveRuns: 3, local queues process sequentially.
	f := newFixture(t, `
name: batch-dag
max_active_runs: 3
steps:
  - name: sleep
    command: sleep 1
`).Enqueue(3).StartScheduler(30 * time.Second)

	f.WaitDrain(20 * time.Second)
	f.Stop()

	// Verify sequential processing: start times should be at least 1 second apart
	times := f.collectStartTimes()
	require.Len(t, times, 3)
	for i := 1; i < len(times); i++ {
		diff := times[i].Sub(times[i-1])
		require.GreaterOrEqual(t, diff, 900*time.Millisecond,
			"Local queue should process sequentially (FIFO), not concurrently")
	}
}

func TestPriorityOrdering(t *testing.T) {
	f := newFixture(t, `
name: priority-dag
max_active_runs: 1
steps:
  - name: echo
    command: echo done
`).
		EnqueueWithPriority(exec.QueuePriorityLow).
		EnqueueWithPriority(exec.QueuePriorityLow).
		EnqueueWithPriority(exec.QueuePriorityHigh).
		EnqueueWithPriority(exec.QueuePriorityHigh).
		StartScheduler(30 * time.Second)

	f.WaitDrain(25 * time.Second)
	f.Stop()

	// Verify high priority runs started before low priority runs
	times := f.collectStartTimes()
	require.Len(t, times, 4)
	// High priority (index 2,3) should start before low priority (index 0,1)
	highPriorityStart := times[2]
	if times[3].Before(highPriorityStart) {
		highPriorityStart = times[3]
	}
	lowPriorityStart := times[0]
	if times[1].Before(lowPriorityStart) {
		lowPriorityStart = times[1]
	}
	require.True(t, highPriorityStart.Before(lowPriorityStart) || highPriorityStart.Equal(lowPriorityStart),
		"High priority runs should start before or equal to low priority runs")
}

func TestRetryEnqueue(t *testing.T) {
	// Verify EnqueueRetry works with real file-based stores:
	// a failed run transitions to Queued with correct metadata and appears in the queue.
	// Scheduler processing of queued items is already covered by TestBasicProcessing
	// and TestGlobalConcurrency — retry-enqueued items are identical from the
	// scheduler's perspective.
	f := newFixture(t, `
name: retry-dag
queue: retry-queue
steps:
  - name: echo
    command: echo retried
`, WithQueue("retry-queue"), WithGlobalQueue("retry-queue", 1)).
		FailedRun()

	runID := f.runIDs[0]

	// Verify the run is in Failed status before retry
	ref := exec.NewDAGRunRef(f.dag.Name, runID)
	att, err := f.th.DAGRunStore.FindAttempt(f.th.Context, ref)
	require.NoError(t, err)
	status, err := att.ReadStatus(f.th.Context)
	require.NoError(t, err)
	require.Equal(t, core.Failed, status.Status)

	// Enqueue the retry — status transitions from Failed to Queued
	f.RetryEnqueue(runID)

	// Verify status persisted as Queued with correct metadata
	att, err = f.th.DAGRunStore.FindAttempt(f.th.Context, ref)
	require.NoError(t, err)
	status, err = att.ReadStatus(f.th.Context)
	require.NoError(t, err)
	assert.Equal(t, core.Queued, status.Status)
	assert.NotEmpty(t, status.QueuedAt)
	assert.Equal(t, core.TriggerTypeRetry, status.TriggerType)

	// Verify item is in the queue
	items, err := f.th.QueueStore.List(f.th.Context, "retry-queue")
	require.NoError(t, err)
	require.Len(t, items, 1)
	data, err := items[0].Data()
	require.NoError(t, err)
	assert.Equal(t, f.dag.Name, data.Name)
	assert.Equal(t, runID, data.ID)
}

func TestSchedulerRetryScanner(t *testing.T) {
	t.Run("EligibleFailedRunRetries", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "failure.marker")
		f := newFixture(t, fmt.Sprintf(`
type: graph
name: retry-dag
queue: retry-queue
retry_policy:
  limit: 1
  interval_sec: 1
  backoff: 1.0
  max_interval_sec: 1
handler_on:
  failure:
    command: "printf failed > %s"
steps:
  - id: retry_step
    command: echo retried
`, markerPath), WithQueue("retry-queue"), WithGlobalQueue("retry-queue", 1))

		failedAt := time.Now().UTC().Add(-30 * time.Second)
		runID := f.FailedRunWithMetadata(runStatusOptions{
			StartedAt:    failedAt.Add(-5 * time.Second),
			FinishedAt:   failedAt,
			ScheduleTime: failedAt.Add(-time.Minute),
			TriggerType:  core.TriggerTypeScheduler,
		})
		markerModTime := prepareFailureMarker(t, markerPath)
		originalAttemptID := f.Status(runID).AttemptID

		f.StartScheduler(40 * time.Second)
		defer f.Stop()

		require.Eventually(t, func() bool {
			status := f.Status(runID)
			return status.Status == core.Succeeded &&
				status.AttemptID != originalAttemptID &&
				status.RetryCount == 1
		}, 25*time.Second, 250*time.Millisecond)

		f.WaitDrain(5 * time.Second)

		latest := f.Status(runID)
		assert.Equal(t, core.Succeeded, latest.Status)
		assert.NotEqual(t, originalAttemptID, latest.AttemptID)
		assert.Equal(t, 1, latest.RetryCount)
		assert.Equal(t, markerModTime, readMarkerModTime(t, markerPath))
	})

	t.Run("NewerScheduledRunSuppressesRetry", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "failure.marker")
		f := newFixture(t, fmt.Sprintf(`
type: graph
name: retry-dag
queue: retry-queue
retry_policy:
  limit: 1
  interval_sec: 1
  backoff: 1.0
  max_interval_sec: 1
handler_on:
  failure:
    command: "printf failed > %s"
steps:
  - id: retry_step
    command: echo retried
`, markerPath), WithQueue("retry-queue"), WithGlobalQueue("retry-queue", 1), WithRetryWindow(48*time.Hour))

		now := time.Now().UTC()
		midnight := retryScanReferenceMidnight(now)
		failedFinishedAt := midnight.Add(2 * time.Minute)
		failedScheduleTime := midnight.Add(-10 * time.Minute)
		newerScheduleTime := midnight.Add(-time.Minute)

		runID := f.FailedRunWithMetadata(runStatusOptions{
			StartedAt:    failedFinishedAt.Add(-5 * time.Second),
			FinishedAt:   failedFinishedAt,
			ScheduleTime: failedScheduleTime,
			TriggerType:  core.TriggerTypeScheduler,
		})
		_ = f.RunningRunWithMetadata(runStatusOptions{
			StartedAt:    now.Add(-time.Minute),
			ScheduleTime: newerScheduleTime,
			TriggerType:  core.TriggerTypeScheduler,
		})
		markerModTime := prepareFailureMarker(t, markerPath)
		originalAttemptID := f.Status(runID).AttemptID

		f.StartScheduler(35 * time.Second)
		defer f.Stop()

		assertRunRemainsFailed(t, f, runID, originalAttemptID, 20*time.Second)

		latest := f.Status(runID)
		assert.Equal(t, core.Failed, latest.Status)
		assert.Equal(t, originalAttemptID, latest.AttemptID)
		assert.Equal(t, markerModTime, readMarkerModTime(t, markerPath))

		items, err := f.th.QueueStore.List(f.th.Context, f.queue)
		require.NoError(t, err)
		require.Empty(t, items)
	})
}

func prepareFailureMarker(t *testing.T, markerPath string) time.Time {
	t.Helper()

	require.NoError(t, os.WriteFile(markerPath, []byte("failed"), 0644))
	modTime := time.Unix(time.Now().Add(-time.Hour).Unix(), 0).UTC()
	require.NoError(t, os.Chtimes(markerPath, modTime, modTime))
	return readMarkerModTime(t, markerPath)
}

func readMarkerModTime(t *testing.T, markerPath string) time.Time {
	t.Helper()

	info, err := os.Stat(markerPath)
	require.NoError(t, err)
	return info.ModTime()
}

func retryScanReferenceMidnight(now time.Time) time.Time {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if now.Sub(midnight) < 5*time.Minute {
		return midnight.Add(-24 * time.Hour)
	}
	return midnight
}

func assertRunRemainsFailed(t *testing.T, f *fixture, runID, attemptID string, duration time.Duration) {
	t.Helper()

	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		status := f.Status(runID)
		require.Equal(t, core.Failed, status.Status)
		require.Equal(t, attemptID, status.AttemptID)

		items, err := f.th.QueueStore.List(f.th.Context, f.queue)
		require.NoError(t, err)
		require.Empty(t, items)

		time.Sleep(250 * time.Millisecond)
	}
}
