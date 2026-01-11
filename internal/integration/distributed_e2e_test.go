package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// TestStartCommandE2E_WithWorkerSelector tests the complete end-to-end flow:
// 1. start command enqueues a top-level DAG with workerSelector
// 2. scheduler picks it up from the queue
// 3. scheduler dispatches it to coordinator
// 4. worker executes it successfully
func TestStartCommandE2E_WithWorkerSelector(t *testing.T) {
	t.Run("E2E_StartCommand_Queue_Scheduler_Worker", func(t *testing.T) {
		// Create test DAG with workerSelector
		yamlContent := `
name: e2e-worker-dag
workerSelector:
  environment: production
  component: batch-worker
steps:
  - name: task-on-worker
    command: echo "Running on worker"
  - name: task-2
    command: sleep 2 && echo "Task 2 done"
    depends:
      - task-on-worker
`

		// Setup coordinator
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Create and start worker with matching labels
		setupWorker(t, coord, "test-worker-1", 10, map[string]string{
			"environment": "production",
			"component":   "batch-worker",
		})

		// Load the DAG
		dagWrapper := coord.DAG(t, yamlContent)

		// Build the enqueue command spec
		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		enqueueSpec := subCmdBuilder.Enqueue(dagWrapper.DAG, runtime.EnqueueOptions{
			Quiet: true,
		})

		// Execute the enqueue command (spawns subprocess)
		err := runtime.Start(coord.Context, enqueueSpec)
		require.NoError(t, err, "Enqueue command should succeed")

		// Wait for the subprocess to complete enqueueing
		require.Eventually(t, func() bool {
			queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return err == nil && len(queueItems) == 1
		}, 2*time.Second, 50*time.Millisecond, "DAG should be enqueued")

		// Verify the DAG was enqueued (not executed locally)
		queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Len(t, queueItems, 1, "DAG should be enqueued once")

		if len(queueItems) > 0 {
			data, err := queueItems[0].Data()
			require.NoError(t, err, "Should be able to get queue item data")
			t.Logf("DAG enqueued: dag=%s runId=%s", data.Name, data.ID)
		}

		// Verify the DAG status is "queued" (not started/running)
		latest, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Queued, latest.Status, "DAG status should be queued")

		// Now start the scheduler to process the queue
		t.Log("Starting scheduler to process queue...")

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)

		schedulerDone := make(chan error, 1)
		go func() {
			schedulerDone <- schedulerInst.Start(schedulerCtx)
		}()

		// Wait for execution completion
		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}

			t.Logf("DAG status: %s", status.Status)

			if status.Status == core.Failed {
				t.Fatalf("DAG execution failed")
			}

			return status.Status == core.Succeeded
		}, 25*time.Second, 500*time.Millisecond, "Timeout waiting for DAG execution to complete")

		schedulerCancel()

		// Wait for scheduler to stop
		select {
		case <-schedulerDone:
		case <-time.After(5 * time.Second):
		}

		// Verify the final status
		finalStatus, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, finalStatus.Status, "DAG should have succeeded")
		require.Len(t, finalStatus.Nodes, 2, "Should have 2 task nodes")

		// Verify both tasks completed
		for _, node := range finalStatus.Nodes {
			require.Equal(t, core.NodeSucceeded, node.Status, fmt.Sprintf("Task %s should have succeeded", node.Step.Name))
		}

		// Verify the queue is now empty
		queueItems, err = coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Empty(t, queueItems, "Queue should be empty after execution")

		t.Log("E2E test completed successfully!")
	})

	t.Run("E2E_StartCommand_WorkerSelector_ShouldExecuteLocally", func(t *testing.T) {
		// Verify that dagu start always executes locally even when workerSelector exists
		yamlContent := `
name: local-start-dag
workerSelector:
  test: value
steps:
  - name: task
    command: echo "Direct execution"
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
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

		// Wait for execution to complete
		dagWrapper.AssertLatestStatus(t, core.Succeeded)

		// Should NOT be enqueued (executed directly)
		queueItems, err := coord.QueueStore.ListByDAGName(ctx, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Len(t, queueItems, 0, "DAG should NOT be enqueued (dagu start runs locally)")
	})

	t.Run("E2E_DistributedExecution_Cancellation_SubDAG", func(t *testing.T) {
		// This test replicates the user's exact scenario:
		// Parent DAG calls a sub-DAG (in same YAML file with ---)
		// The sub-DAG has workerSelector and runs on a worker
		// When we cancel the parent, the sub-DAG should also be cancelled
		yamlContent := `
steps:
  - run: dotest
params:
  - URL: default_value
---
name: dotest
workerSelector:
  foo: bar
steps:
  - name: long-sleep
    command: sleep 30
`

		// Setup coordinator
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Create and start worker with matching labels (matching the sub-DAG's workerSelector)
		setupWorker(t, coord, "test-worker-cancel", 10, map[string]string{
			"foo": "bar", // Match the sub-DAG's workerSelector
		})

		// Load the DAG
		dagWrapper := coord.DAG(t, yamlContent)

		// Build the start command spec
		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		startSpec := subCmdBuilder.Start(dagWrapper.DAG, runtime.StartOptions{
			Quiet: true,
		})

		// Execute the start command (spawns subprocess)
		err := runtime.Start(coord.Context, startSpec)
		require.NoError(t, err, "Start command should succeed")

		// Start the scheduler
		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)

		schedulerDone := make(chan error, 1)
		go func() {
			schedulerDone <- schedulerInst.Start(schedulerCtx)
		}()

		// Wait for the DAG to be running
		var dagRunID string
		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			t.Logf("DAG status while waiting: %s", status.Status)
			if status.Status == core.Running {
				dagRunID = status.DAGRunID
				t.Logf("DAG is now running with ID: %s", dagRunID)
				return true
			}
			return false
		}, 10*time.Second, 200*time.Millisecond, "Timeout waiting for DAG to start running")

		// Now send the stop signal
		t.Log("Sending stop signal to the running DAG...")
		err = coord.DAGRunMgr.Stop(coord.Context, dagWrapper.DAG, dagRunID)
		require.NoError(t, err, "Stop command should succeed")

		// Wait for the DAG to be cancelled/stopped
		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			t.Logf("DAG status after stop: %s", status.Status)
			return status.Status == core.Aborted || status.Status == core.Failed
		}, 15*time.Second, 500*time.Millisecond, "Timeout waiting for DAG to be cancelled")

		schedulerCancel()

		// Wait for scheduler to stop
		select {
		case <-schedulerDone:
		case <-time.After(5 * time.Second):
		}

		// Verify the final status
		finalStatus, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Contains(t, []core.Status{core.Aborted, core.Failed}, finalStatus.Status,
			"DAG should have been canceled or failed")

		t.Log("Cancellation test completed successfully!")
	})

}
