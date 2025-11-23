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

func TestQueueProcessor_StrictFIFO(t *testing.T) {
	th := test.Setup(t)

	// Enable queues
	th.Config.Queues.Enabled = true
	th.Config.Queues.Config = []config.QueueConfig{
		{Name: "test-dag", MaxActiveRuns: 1},
	}

	// Reduce backoff for testing
	scheduler.InitialBackoffInterval = 10 * time.Millisecond
	scheduler.MaxBackoffInterval = 50 * time.Millisecond
	scheduler.MaxBackoffRetries = 2

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

	// Create QueueProcessor
	processor := scheduler.NewQueueProcessor(
		th.QueueStore,
		th.DAGRunStore,
		th.ProcStore,
		dagExecutor,
		th.Config.Queues,
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
		if item.Data().ID == "run-1" {
			foundRun1 = true
		}
		if item.Data().ID == "run-2" {
			foundRun2 = true
		}
	}
	require.True(t, foundRun1, "run-1 should still be in queue")
	require.True(t, foundRun2, "run-2 should still be in queue (strict FIFO - not processed)")
}
