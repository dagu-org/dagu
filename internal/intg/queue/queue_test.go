package queue_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
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
maxActiveRuns: 1
steps:
  - name: sleep
    command: sleep 1
`, WithQueue("global-queue"), WithGlobalQueue("global-queue", 3)).
		Enqueue(3).StartScheduler(30 * time.Second)

	f.WaitDrain(15 * time.Second)
	f.Stop()
	f.AssertConcurrent(2 * time.Second)
}

func TestDAGMaxActiveRuns(t *testing.T) {
	start := time.Now()
	f := newFixture(t, `
name: batch-dag
maxActiveRuns: 3
steps:
  - name: sleep
    command: sleep 2
`).Enqueue(3).StartScheduler(30 * time.Second)

	f.WaitDrain(20 * time.Second)
	f.Stop()
	f.AssertConcurrent(2 * time.Second)
	require.Less(t, time.Since(start), 8*time.Second)
}

func TestPriorityOrdering(t *testing.T) {
	f := newFixture(t, `
name: priority-dag
maxActiveRuns: 1
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
