package queue_test

import (
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
