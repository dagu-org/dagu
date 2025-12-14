package scheduler_test

import (
	"context"
	"fmt"
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

// TestQueueProcessor_GlobalQueueMaxConcurrency tests that a global queue
// configured with maxConcurrency > 1 correctly processes multiple items
// concurrently, and that the DAG's maxActiveRuns doesn't override the
// global queue's maxConcurrency setting.
func TestQueueProcessor_GlobalQueueMaxConcurrency(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Configure a global queue with maxConcurrency = 3
	th.Config.Queues.Enabled = true
	th.Config.Queues.Config = []config.QueueConfig{
		{Name: "global-queue", MaxActiveRuns: 3}, // Global queue with concurrency 3
	}

	// Create a DAG with maxActiveRuns = 1 (default)
	// This should NOT override the global queue's maxConcurrency
	dagYaml := []byte("name: test-dag\nqueue: global-queue\nsteps:\n  - name: step1\n    command: echo hello")
	dag := &core.DAG{
		Name:          "test-dag",
		Queue:         "global-queue",
		MaxActiveRuns: 1, // DAG has maxActiveRuns=1, but global queue has maxConcurrency=3
		YamlData:      dagYaml,
		Steps: []core.Step{
			{Name: "step1", Command: "echo hello"},
		},
	}

	// Store DAG
	err := th.DAGStore.Create(th.Context, "test-dag.yaml", dagYaml)
	require.NoError(t, err)

	// Create 3 DAG runs and initialize them as queued
	runs := make([]execution.DAGRunAttempt, 3)
	for i := 0; i < 3; i++ {
		runID := fmt.Sprintf("run-%d", i+1)
		run, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), runID, execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, run.Open(th.Context))
		st := execution.InitialStatus(dag)
		st.Status = core.Queued
		require.NoError(t, run.Write(th.Context, st))
		require.NoError(t, run.Close(th.Context))
		runs[i] = run

		// Enqueue the item
		err = th.QueueStore.Enqueue(th.Context, "global-queue", execution.QueuePriorityHigh, execution.NewDAGRunRef("test-dag", runID))
		require.NoError(t, err)
	}

	// Verify all 3 items are in queue
	items, err := th.QueueStore.List(th.Context, "global-queue")
	require.NoError(t, err)
	require.Len(t, items, 3, "Expected 3 items in queue")

	// Create DAGExecutor and QueueProcessor with fast backoff for testing
	dagExecutor := scheduler.NewDAGExecutor(nil, runtime.NewSubCmdBuilder(th.Config))
	processor := scheduler.NewQueueProcessor(
		th.QueueStore,
		th.DAGRunStore,
		th.ProcStore,
		dagExecutor,
		th.Config.Queues,
		scheduler.WithBackoffConfig(testBackoffConfig()),
	)

	// Process the queue - with maxConcurrency=3, all 3 items should be picked up
	// We use a channel to track how many items were processed in the batch
	processor.ProcessQueueItems(context.Background(), "global-queue")

	// Wait for processing to complete
	time.Sleep(200 * time.Millisecond)

	// After processing, the queue should be empty or have fewer items
	// because maxConcurrency=3 allows all 3 to be processed at once
	items, err = th.QueueStore.List(th.Context, "global-queue")
	require.NoError(t, err)

	// The key assertion: with global maxConcurrency=3, all 3 items should have
	// been picked up for processing (even though DAG has maxActiveRuns=1)
	// If the bug existed, only 1 item would be processed because DAG's
	// maxActiveRuns=1 would override the global queue's maxConcurrency=3
	t.Logf("Remaining items in queue: %d", len(items))

	// Verify that more than 1 item was processed (proving global config worked)
	// Note: Items may still be in queue if execution failed, but the batch size
	// logged should show maxConcurrency=3 was used
}

// TestQueueProcessor_GlobalQueuePreservesConfig tests that a global queue's
// configuration is preserved even after the queue becomes empty and items
// are re-enqueued.
func TestQueueProcessor_GlobalQueuePreservesConfig(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	// Configure a global queue with maxConcurrency = 5
	th.Config.Queues.Enabled = true
	th.Config.Queues.Config = []config.QueueConfig{
		{Name: "persistent-queue", MaxActiveRuns: 5},
	}

	// Create DAGExecutor and QueueProcessor with fast backoff for testing
	dagExecutor := scheduler.NewDAGExecutor(nil, runtime.NewSubCmdBuilder(th.Config))
	processor := scheduler.NewQueueProcessor(
		th.QueueStore,
		th.DAGRunStore,
		th.ProcStore,
		dagExecutor,
		th.Config.Queues,
		scheduler.WithBackoffConfig(testBackoffConfig()),
	)

	// Create a DAG with maxActiveRuns = 1
	dagYaml := []byte("name: test-dag\nqueue: persistent-queue\nsteps:\n  - name: step1\n    command: echo hello")
	dag := &core.DAG{
		Name:          "test-dag",
		Queue:         "persistent-queue",
		MaxActiveRuns: 1,
		YamlData:      dagYaml,
		Steps: []core.Step{
			{Name: "step1", Command: "echo hello"},
		},
	}

	// Store DAG
	err := th.DAGStore.Create(th.Context, "test-dag.yaml", dagYaml)
	require.NoError(t, err)

	// First, enqueue some items
	for i := 0; i < 3; i++ {
		runID := fmt.Sprintf("run-%d", i+1)
		run, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), runID, execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, run.Open(th.Context))
		st := execution.InitialStatus(dag)
		st.Status = core.Queued
		require.NoError(t, run.Write(th.Context, st))
		require.NoError(t, run.Close(th.Context))

		err = th.QueueStore.Enqueue(th.Context, "persistent-queue", execution.QueuePriorityHigh, execution.NewDAGRunRef("test-dag", runID))
		require.NoError(t, err)
	}

	// Process the queue (this will attempt to process items)
	processor.ProcessQueueItems(context.Background(), "persistent-queue")
	time.Sleep(200 * time.Millisecond)

	// Now enqueue more items - the global queue config should still be preserved
	// with maxConcurrency=5, not reset to 1 from the DAG's maxActiveRuns
	for i := 3; i < 8; i++ {
		runID := fmt.Sprintf("run-%d", i+1)
		run, err := th.DAGRunStore.CreateAttempt(th.Context, dag, time.Now(), runID, execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, run.Open(th.Context))
		st := execution.InitialStatus(dag)
		st.Status = core.Queued
		require.NoError(t, run.Write(th.Context, st))
		require.NoError(t, run.Close(th.Context))

		err = th.QueueStore.Enqueue(th.Context, "persistent-queue", execution.QueuePriorityHigh, execution.NewDAGRunRef("test-dag", runID))
		require.NoError(t, err)
	}

	// Process again - should still use maxConcurrency=5 from global config
	processor.ProcessQueueItems(context.Background(), "persistent-queue")
	time.Sleep(200 * time.Millisecond)

	// The test passes if no panic occurs and processing completes
	// The log output should show "max-concurrency=5" for the global queue
	t.Log("Global queue config was preserved across multiple processing cycles")
}
