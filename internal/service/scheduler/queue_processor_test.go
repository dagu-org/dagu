package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// testBackoffConfig returns a fast backoff configuration for testing.
func testBackoffConfig() scheduler.BackoffConfig {
	return scheduler.BackoffConfig{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		MaxRetries:      2,
	}
}

func TestQueueProcessor_StrictFIFO(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Enable queues
	th.Config.Queues.Enabled = true
	th.Config.Queues.Config = []config.QueueConfig{
		{Name: "test-dag", MaxActiveRuns: 1},
	}

	// Create a simple DAG (local execution, no dispatcher needed)
	dagYaml := []byte("name: test-dag\nsteps:\n  - name: fail\n    command: exit 1")
	dag := &core.DAG{
		Name:          "test-dag",
		MaxActiveRuns: 1,
		YamlData:      dagYaml,
		Steps: []core.Step{
			{Name: "fail", Command: "exit 1"},
		},
	}

	// Store DAG
	err := th.DAGStore.Create(th.Context, "test-dag.yaml", dagYaml)
	require.NoError(t, err)

	// Create 2 DAG runs and initialize them
	run1, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), "run-1", execution.NewDAGRunAttemptOptions{})
	require.NoError(t, err)
	require.NoError(t, run1.Open(th.Context))
	st1 := execution.InitialStatus(dag)
	st1.Status = core.Queued
	require.NoError(t, run1.Write(th.Context, st1))
	require.NoError(t, run1.Close(th.Context))

	run2, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), "run-2", execution.NewDAGRunAttemptOptions{})
	require.NoError(t, err)
	require.NoError(t, run2.Open(th.Context))
	st2 := execution.InitialStatus(dag)
	st2.Status = core.Queued
	require.NoError(t, run2.Write(th.Context, st2))
	require.NoError(t, run2.Close(th.Context))

	// Enqueue items
	err = th.QueueStore.Enqueue(th.Context, "test-dag", execution.QueuePriorityHigh, execution.NewDAGRunRef("test-dag", "run-1"))
	require.NoError(t, err)

	err = th.QueueStore.Enqueue(th.Context, "test-dag", execution.QueuePriorityHigh, execution.NewDAGRunRef("test-dag", "run-2"))
	require.NoError(t, err)

	// Create DAGExecutor (no dispatcher, so it will use local execution)
	dagExecutor := scheduler.NewDAGExecutor(nil, runtime.NewSubCmdBuilder(th.Config))

	// Create QueueProcessor with fast backoff for testing
	processor := scheduler.NewQueueProcessor(
		th.QueueStore,
		th.DAGRunStore,
		th.ProcStore,
		dagExecutor,
		th.Config.Queues,
		scheduler.WithBackoffConfig(testBackoffConfig()),
	)

	// Process the queue
	// Since run-1 will fail and be retried multiple times,
	// run-2 should NOT be processed due to strict FIFO
	processor.ProcessQueueItems(context.Background(), "test-dag")

	// Give the processor enough time to attempt all retries for run-1
	// The backoff is configured to: initial=10ms, max=50ms, retries=2
	// So worst case: 10ms + 20ms + 40ms + overhead ≈ 150ms should be safe
	time.Sleep(150 * time.Millisecond)

	// Verify that run-2 is still in the queue (not processed)
	items, err := th.QueueStore.List(th.Context, "test-dag")
	require.NoError(t, err)
	require.Len(t, items, 2, "Both items should still be in queue since run-1 keeps failing")

	// Verify both items are still there
	foundRun1 := false
	foundRun2 := false
	for _, item := range items {
		data, err := item.Data()
		require.NoError(t, err, "expected no error when getting item data")
		if data.ID == "run-1" {
			foundRun1 = true
		}
		if data.ID == "run-2" {
			foundRun2 = true
		}
	}
	require.True(t, foundRun1, "run-1 should still be in queue")
	require.True(t, foundRun2, "run-2 should still be in queue (strict FIFO - not processed)")
}
