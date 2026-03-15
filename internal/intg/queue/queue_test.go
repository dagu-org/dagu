// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package queue_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
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
	f := newFixture(t, `
name: batch-dag
max_active_runs: 3
steps:
  - name: sleep
    command: sleep 1
`).Enqueue(3).StartScheduler(30 * time.Second)

	f.WaitDrain(20 * time.Second)
	f.Stop()

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

	times := f.collectStartTimes()
	require.Len(t, times, 4)
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
	f := newFixture(t, `
name: retry-dag
queue: retry-queue
steps:
  - name: echo
    command: echo retried
`, WithQueue("retry-queue"), WithGlobalQueue("retry-queue", 1)).
		FailedRun()

	runID := f.runIDs[0]

	ref := exec.NewDAGRunRef(f.dag.Name, runID)
	att, err := f.th.DAGRunStore.FindAttempt(f.th.Context, ref)
	require.NoError(t, err)
	status, err := att.ReadStatus(f.th.Context)
	require.NoError(t, err)
	require.Equal(t, core.Failed, status.Status)

	f.RetryEnqueue(runID)

	att, err = f.th.DAGRunStore.FindAttempt(f.th.Context, ref)
	require.NoError(t, err)
	status, err = att.ReadStatus(f.th.Context)
	require.NoError(t, err)
	assert.Equal(t, core.Queued, status.Status)
	assert.NotEmpty(t, status.QueuedAt)
	assert.Equal(t, core.TriggerTypeRetry, status.TriggerType)
	assert.Zero(t, status.AutoRetryCount)

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
  backoff: false
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
		originalAttemptID := f.MustStatus(runID).AttemptID

		f.StartScheduler(40 * time.Second)
		defer f.Stop()

		require.Eventually(t, func() bool {
			status := f.MustStatus(runID)
			return status.Status == core.Succeeded &&
				status.AttemptID != originalAttemptID &&
				status.AutoRetryCount == 1
		}, 25*time.Second, 250*time.Millisecond)

		f.WaitDrain(5 * time.Second)

		latest := f.MustStatus(runID)
		assert.Equal(t, core.Succeeded, latest.Status)
		assert.NotEqual(t, originalAttemptID, latest.AttemptID)
		assert.Equal(t, 1, latest.AutoRetryCount)
		assert.Equal(t, markerModTime, readMarkerModTime(t, markerPath))
	})

	t.Run("MissingFinishedAtStillRetriesViaCreatedAt", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "failure.marker")
		f := newFixture(t, fmt.Sprintf(`
type: graph
name: retry-dag
queue: retry-queue
retry_policy:
  limit: 1
  interval_sec: 1
  backoff: false
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
			CreatedAt:    failedAt,
			StartedAt:    failedAt.Add(-5 * time.Second),
			ScheduleTime: failedAt.Add(-time.Minute),
			TriggerType:  core.TriggerTypeScheduler,
		})
		originalAttemptID := f.MustStatus(runID).AttemptID

		f.StartScheduler(40 * time.Second)
		defer f.Stop()

		require.Eventually(t, func() bool {
			status := f.MustStatus(runID)
			return status.Status == core.Succeeded &&
				status.AttemptID != originalAttemptID &&
				status.AutoRetryCount == 1
		}, 25*time.Second, 250*time.Millisecond)

		latest := f.MustStatus(runID)
		assert.Equal(t, core.Succeeded, latest.Status)
		assert.NotEqual(t, originalAttemptID, latest.AttemptID)
		assert.Equal(t, 1, latest.AutoRetryCount)
	})

	t.Run("NewerScheduledRunDoesNotSuppressRetry", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "failure.marker")
		f := newFixture(t, fmt.Sprintf(`
type: graph
name: retry-dag
queue: retry-queue
retry_policy:
  limit: 1
  interval_sec: 1
  backoff: false
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
		originalAttemptID := f.MustStatus(runID).AttemptID

		f.StartScheduler(35 * time.Second)
		defer f.Stop()

		require.Eventually(t, func() bool {
			status := f.MustStatus(runID)
			return status.Status == core.Succeeded &&
				status.AttemptID != originalAttemptID &&
				status.AutoRetryCount == 1
		}, 25*time.Second, 250*time.Millisecond)

		latest := f.MustStatus(runID)
		assert.Equal(t, core.Succeeded, latest.Status)
		assert.NotEqual(t, originalAttemptID, latest.AttemptID)
		assert.Equal(t, 1, latest.AutoRetryCount)
		assert.Equal(t, markerModTime, readMarkerModTime(t, markerPath))
	})
}

func TestCatchupQueuedHappyPath(t *testing.T) {
	scheduleTime := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)

	f := newFixture(t, `
name: catchup-local-test
steps:
  - name: echo-step
    command: echo catchup-local
`)

	runID := f.enqueueCatchup(scheduleTime)
	f.StartScheduler(30 * time.Second)
	defer f.Stop()

	f.WaitDrain(25 * time.Second)
	status := f.waitForRecentStatus(25*time.Second, func(st exec.DAGRunStatus) bool {
		return st.DAGRunID == runID && st.Status == core.Succeeded
	})

	require.Equal(t, core.TriggerTypeCatchUp, status.TriggerType)
	require.Equal(t, stringutil.FormatTime(scheduleTime), status.ScheduleTime)
	require.NotEmpty(t, status.Log)
	require.FileExists(t, status.Log)

	items, err := f.th.QueueStore.List(f.th.Context, f.queue)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestSchedulerCatchupFromPersistedWatermark(t *testing.T) {
	scheduledTime := stableCurrentMinute(t)

	f := newFixture(t, `
name: catchup-watermark-test
schedule: "* * * * *"
catchup_window: "2m"
steps:
  - name: echo-step
    command: echo catchup-from-watermark
`)

	f.seedWatermark(scheduledTime.Add(-time.Minute), scheduledTime.Add(-time.Minute))
	f.StartScheduler(45 * time.Second)
	defer f.Stop()

	f.WaitDrain(30 * time.Second)
	status := f.waitForRecentStatus(30*time.Second, func(st exec.DAGRunStatus) bool {
		return st.Status == core.Succeeded &&
			st.TriggerType == core.TriggerTypeCatchUp &&
			st.ScheduleTime == stringutil.FormatTime(scheduledTime)
	})

	require.Equal(t, core.TriggerTypeCatchUp, status.TriggerType)
	require.Equal(t, stringutil.FormatTime(scheduledTime), status.ScheduleTime)
	require.NotEmpty(t, status.Log)
	require.FileExists(t, status.Log)
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

func stableCurrentMinute(t *testing.T) time.Time {
	t.Helper()

	now := time.Now().UTC()
	if now.Second() >= 50 {
		nextSafe := now.Truncate(time.Minute).Add(time.Minute + 2*time.Second)
		time.Sleep(time.Until(nextSafe))
	}

	return time.Now().UTC().Truncate(time.Minute)
}
