package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// TestStartCommandE2E_WithWorkerSelector tests the complete end-to-end flow:
// 1. start command enqueues a top-level DAG with workerSelector
// 2. scheduler picks it up from the queue
// 3. scheduler dispatches it to coordinator
// 4. worker executes it successfully
//
// This is the comprehensive integration test for the distributed execution fix.
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
		coord := test.SetupCoordinator(t)
		coord.Config.Queues.Enabled = true

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		ctx, cancel := context.WithCancel(coord.Context)
		t.Cleanup(cancel)

		// Create and start worker with matching labels
		workerInst := worker.NewWorker(
			"test-worker-1",
			10, // maxActiveRuns
			coordinatorClient,
			map[string]string{
				"environment": "production",
				"component":   "batch-worker",
			},
			coord.Config,
		)

		go func(w *worker.Worker) {
			if err := w.Start(ctx); err != nil {
				t.Logf("Worker stopped: %v", err)
			}
		}(workerInst)

		// Give worker time to connect
		time.Sleep(100 * time.Millisecond)

		// Load the DAG
		dagWrapper := coord.DAG(t, yamlContent)

		// Unset DISABLE_DAG_RUN_QUEUE so the subprocess can enqueue
		require.NoError(t, os.Unsetenv("DISABLE_DAG_RUN_QUEUE"))

		// Build the start command spec
		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		startSpec := subCmdBuilder.Start(dagWrapper.DAG, runtime.StartOptions{
			Quiet: true,
		})

		// Execute the start command (spawns subprocess)
		err := runtime.Start(coord.Context, startSpec)
		require.NoError(t, err, "Start command should succeed")

		// Wait for the subprocess to complete enqueueing
		time.Sleep(500 * time.Millisecond)

		// Verify the DAG was enqueued (not executed locally)
		queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.DAG.ProcGroup(), dagWrapper.DAG.Name)
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

		// Now start the scheduler to process the queue
		t.Log("Starting scheduler to process queue...")

		schedulerCtx, schedulerCancel := context.WithTimeout(ctx, 30*time.Second)
		defer schedulerCancel()

		// Create DAGExecutor using coordinator's service registry
		schedulerCoordCli := coordinatorClient // Use the same coordinator client as the worker
		de := scheduler.NewDAGExecutor(schedulerCoordCli, runtime.NewSubCmdBuilder(coord.Config))
		em := scheduler.NewEntryReader(coord.Config.Paths.DAGsDir, coord.DAGStore, coord.DAGRunMgr, de, "")

		// Create scheduler instance
		schedulerInst, err := scheduler.New(
			coord.Config,
			em,
			coord.DAGRunMgr,
			coord.DAGRunStore,
			coord.QueueStore,
			coord.ProcStore,
			coord.ServiceRegistry,
			schedulerCoordCli,
		)
		require.NoError(t, err, "failed to create scheduler")

		schedulerDone := make(chan error, 1)
		go func() {
			schedulerDone <- schedulerInst.Start(schedulerCtx)
		}()

		// Give scheduler time to start
		time.Sleep(200 * time.Millisecond)

		// Poll for execution completion
		var executionSucceeded bool
		timeout := time.After(25 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
				if err != nil {
					continue
				}

				t.Logf("DAG status: %s", status.Status)

				if status.Status == core.Succeeded {
					executionSucceeded = true
					goto DONE
				}

				if status.Status == core.Failed {
					t.Fatalf("DAG execution failed")
				}

			case <-timeout:
				t.Fatal("Timeout waiting for DAG execution to complete")
			}
		}

	DONE:
		schedulerCancel()

		// Wait for scheduler to stop
		select {
		case <-schedulerDone:
		case <-time.After(5 * time.Second):
		}

		require.True(t, executionSucceeded, "DAG should have executed successfully on worker")

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
		queueItems, err = coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.DAG.ProcGroup(), dagWrapper.DAG.Name)
		require.NoError(t, err)
		require.Empty(t, queueItems, "Queue should be empty after execution")

		t.Log("E2E test completed successfully!")
	})

	t.Run("E2E_StartCommand_WithNoQueueFlag_ShouldExecuteDirectly", func(t *testing.T) {
		// Verify that --no-queue flag bypasses enqueueing even when workerSelector exists
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

		// Wait for execution
		time.Sleep(1 * time.Second)

		// Should NOT be enqueued (executed directly)
		queueItems, err := coord.QueueStore.ListByDAGName(ctx, dagWrapper.DAG.ProcGroup(), dagWrapper.DAG.Name)
		require.NoError(t, err)
		require.Len(t, queueItems, 0, "DAG should NOT be enqueued when --no-queue is set")

		// Verify it succeeded (executed locally)
		dagWrapper.AssertLatestStatus(t, core.Succeeded)
	})
}
