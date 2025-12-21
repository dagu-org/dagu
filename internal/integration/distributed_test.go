package integration_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// 3. Worker executes directly as ordered by the scheduler (no re-enqueue)
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

		// Wait for completion (executed locally)
		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			return err == nil && status.Status == core.Succeeded
		}, 2*time.Second, 50*time.Millisecond, "DAG should complete successfully")

		// Verify the DAG was NOT enqueued (executed locally)
		queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Len(t, queueItems, 0, "DAG should NOT be enqueued (dagu start runs locally)")

		if len(queueItems) > 0 {
			data, err := queueItems[0].Data()
			require.NoError(t, err, "Should be able to get queue item data")
			t.Logf("DAG enqueued: dag=%s runId=%s", data.Name, data.ID)
		}

		// Verify the DAG status is "succeeded" (executed locally)
		latest, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, latest.Status, "DAG status should be succeeded")
	})

	t.Run("StartCommand_WorkerSelector_ShouldExecuteLocally", func(t *testing.T) {
		// Verify that dagu start always executes locally
		// even when workerSelector exists

		yamlContent := `
name: local-start-dag
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

		// Build start command
		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		startSpec := subCmdBuilder.Start(dagWrapper.DAG, runtime.StartOptions{
			Quiet: true,
		})

		err := runtime.Start(ctx, startSpec)
		require.NoError(t, err)

		// Should NOT be enqueued (executed directly)
		queueItems, err := coord.QueueStore.ListByDAGName(ctx, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Len(t, queueItems, 0, "DAG should NOT be enqueued (dagu start runs locally)")

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

	// Execute the start command (runs locally now)
	err := runtime.Start(coord.Context, startSpec)
	require.NoError(t, err, "Start command should succeed (process started)")

	// Wait for completion (executed locally)
	require.Eventually(t, func() bool {
		status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		return err == nil && status.Status == core.Failed
	}, 5*time.Second, 100*time.Millisecond, "DAG should fail")

	// Should NOT be enqueued
	queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
	require.NoError(t, err)
	require.Len(t, queueItems, 0, "DAG should NOT be enqueued (dagu start runs locally)")

	status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
	require.NoError(t, err)
	dagRunID := status.DAGRunID
	t.Logf("DAG failed: dag=%s runId=%s", status.Name, status.DAGRunID)

	// Now retry the DAG - it should run locally
	retrySpec := subCmdBuilder.Retry(dagWrapper.DAG, dagRunID, "")
	err = runtime.Start(coord.Context, retrySpec)
	require.NoError(t, err, "Retry command should succeed (process started)")

	// Wait for completion
	require.Eventually(t, func() bool {
		status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		return err == nil && status.Status == core.Failed
	}, 5*time.Second, 100*time.Millisecond, "Retry should fail")

	// Should NOT be enqueued
	queueItems, err = coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
	require.NoError(t, err)
	require.Len(t, queueItems, 0, "Retry should NOT be enqueued (dagu retry runs locally)")
}
