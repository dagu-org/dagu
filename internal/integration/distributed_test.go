package integration_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// TestStartCommandWithWorkerSelector tests that the start command enqueues
// DAGs with workerSelector instead of executing them locally, and that the
// scheduler dispatches them to workers correctly.
//
// This is the integration test for the distributed execution fix where:
// 1. start command checks for workerSelector → enqueues (instead of executing)
// 2. Scheduler queue handler picks it up → dispatches to coordinator
// 3. Worker executes with --no-queue flag → executes directly (no re-enqueue)
func TestStartCommandWithWorkerSelector(t *testing.T) {
	t.Run("StartCommand_WithWorkerSelector_ShouldEnqueue", func(t *testing.T) {
		// This test verifies that when a DAG has workerSelector,
		// the start command enqueues it instead of executing locally

		yamlContent := `
name: toplevel-worker-dag
workerSelector:
  environment: production
  component: batch-worker
steps:
  - name: task-on-worker
    command: echo "Running on worker"
  - name: task-2
    command: echo "Task 2"
    depends:
      - task-on-worker
`
		coord := test.SetupCoordinator(t)
		coord.Config.Queues.Enabled = true

		// Load the DAG
		dagWrapper := coord.DAG(t, yamlContent)

		// Build the start command
		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		startSpec := subCmdBuilder.Start(dagWrapper.DAG, runtime.StartOptions{
			Quiet: true,
		})

		// Execute the start command (spawns subprocess)
		err := runtime.Start(coord.Context, startSpec)
		require.NoError(t, err, "Start command should succeed")

		// Wait for the DAG to be enqueued
		require.Eventually(t, func() bool {
			queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return err == nil && len(queueItems) == 1
		}, 2*time.Second, 50*time.Millisecond, "DAG should be enqueued")

		// Verify the DAG was enqueued (not executed locally)
		queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Len(t, queueItems, 1, "DAG should be enqueued once")

		if len(queueItems) > 0 {
			data := queueItems[0].Data()
			t.Logf("DAG enqueued: dag=%s runId=%s", data.Name, data.ID)
		}

		// Verify the DAG status is "queued" (not started/running)
		latest, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Queued, latest.Status, "DAG status should be queued")
	})

	t.Run("StartCommand_WithNoQueueFlag_ShouldExecuteDirectly", func(t *testing.T) {
		// Verify that --no-queue flag bypasses enqueueing
		// even when workerSelector exists

		yamlContent := `
name: no-queue-dag
workerSelector:
  test: value
steps:
  - name: task
    command: echo "Direct execution"
`
		coord := test.SetupCoordinator(t)
		ctx := coord.Context

		// Load the DAG
		dagWrapper := coord.DAG(t, yamlContent)

		// Build start command WITH --no-queue flag
		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		startSpec := subCmdBuilder.Start(dagWrapper.DAG, runtime.StartOptions{
			Quiet:   true,
			NoQueue: true, // This bypasses enqueueing
		})

		err := runtime.Start(ctx, startSpec)
		require.NoError(t, err)

		// Should NOT be enqueued (executed directly)
		queueItems, err := coord.QueueStore.ListByDAGName(ctx, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Len(t, queueItems, 0, "DAG should NOT be enqueued when --no-queue is set")

		// Verify it succeeded (executed locally)
		dagWrapper.AssertLatestStatus(t, core.Succeeded)
	})
}

// TestRetryCommandWithWorkerSelector tests that the retry command enqueues
// DAGs with workerSelector instead of executing them locally.
func TestRetryCommandWithWorkerSelector(t *testing.T) {
	// This test verifies that when retrying a DAG with workerSelector,
	// the retry command enqueues it instead of executing locally

	yamlContent := `
name: retry-worker-dag
workerSelector:
  environment: production
steps:
  - name: failing-task
    command: exit 1
`
	coord := test.SetupCoordinator(t)
	coord.Config.Queues.Enabled = true

	// Load the DAG
	dagWrapper := coord.DAG(t, yamlContent)

	// First, start the DAG (it will fail)
	subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
	startSpec := subCmdBuilder.Start(dagWrapper.DAG, runtime.StartOptions{
		Quiet: true,
	})

	err := runtime.Start(coord.Context, startSpec)
	require.NoError(t, err, "Start command should succeed")

	// Wait for the DAG to be enqueued
	require.Eventually(t, func() bool {
		queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
		return err == nil && len(queueItems) == 1
	}, 2*time.Second, 50*time.Millisecond, "DAG should be enqueued")

	// Verify the DAG was enqueued
	queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
	require.NoError(t, err)
	require.Len(t, queueItems, 1, "DAG should be enqueued once")

	var dagRunID string
	var dagRun execution.DAGRunRef
	if len(queueItems) > 0 {
		data := queueItems[0].Data()
		dagRunID = data.ID
		dagRun = data
		t.Logf("DAG enqueued: dag=%s runId=%s", data.Name, data.ID)
	}

	// Dequeue it to simulate processing
	_, err = coord.QueueStore.DequeueByDAGRunID(coord.Context, dagWrapper.ProcGroup(), dagRun)
	require.NoError(t, err)

	// Now retry the DAG - it should be enqueued again
	retrySpec := subCmdBuilder.Retry(dagWrapper.DAG, dagRunID, "", false)
	err = runtime.Run(coord.Context, retrySpec)
	require.NoError(t, err, "Retry command should succeed")

	// Wait for the retry to be enqueued
	require.Eventually(t, func() bool {
		queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
		return err == nil && len(queueItems) == 1
	}, 2*time.Second, 50*time.Millisecond, "Retry should be enqueued")

	// Verify the retry was enqueued
	queueItems, err = coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
	require.NoError(t, err)
	require.Len(t, queueItems, 1, "Retry should be enqueued once")

	if len(queueItems) > 0 {
		data := queueItems[0].Data()
		require.Equal(t, dagRunID, data.ID, "Should have same DAG run ID")
		t.Logf("Retry enqueued: dag=%s runId=%s", data.Name, data.ID)
	}
}
