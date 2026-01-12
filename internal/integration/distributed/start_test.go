package distributed_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Start Command Tests
// =============================================================================
// These tests verify the behavior of `dagu start` command with workerSelector.
// When a DAG has workerSelector, start command should dispatch to coordinator.

func TestStartCommand_WithWorkerSelector(t *testing.T) {
	t.Run("dispatchesToCoordinatorAndWaitsForCompletion", func(t *testing.T) {
		yamlContent := `
name: start-cmd-test
workerSelector:
  environment: production
  component: batch-worker
steps:
  - name: task-on-worker
    command: echo "Running on worker"
  - name: task-2
    command: echo "Task 2 done"
    depends:
      - task-on-worker
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		coordinatorClient := coord.GetCoordinatorClient(t)

		setupSharedNothingWorker(t, coord, "test-worker-1", map[string]string{
			"environment": "production",
			"component":   "batch-worker",
		})

		dagWrapper := coord.DAG(t, yamlContent)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err, "Enqueue command should succeed")

		require.Eventually(t, func() bool {
			queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return err == nil && len(queueItems) == 1
		}, 2*time.Second, 50*time.Millisecond, "DAG should be enqueued")

		queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Len(t, queueItems, 1, "DAG should be enqueued once")

		latest, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Queued, latest.Status, "DAG status should be queued")

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)

		schedulerDone := make(chan error, 1)
		go func() {
			schedulerDone <- schedulerInst.Start(schedulerCtx)
		}()

		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}

			if !status.Status.IsActive() && status.Status != core.Succeeded && status.Status != core.Queued {
				t.Fatalf("DAG execution failed with status %s", status.Status)
			}

			if status.Status != core.Succeeded {
				return false
			}

			queueItems, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return err == nil && len(queueItems) == 0
		}, 25*time.Second, 500*time.Millisecond, "Timeout waiting for DAG execution and queue cleanup")

		schedulerCancel()

		select {
		case <-schedulerDone:
		case <-time.After(5 * time.Second):
		}

		finalStatus, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, finalStatus.Status, "DAG should have succeeded")
		require.Len(t, finalStatus.Nodes, 2, "Should have 2 task nodes")
		assertAllNodesSucceeded(t, finalStatus)

		queueItems, err = coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
		require.NoError(t, err)
		require.Empty(t, queueItems, "Queue should be empty after execution")
	})
}

func TestStartCommand_DirectExecution(t *testing.T) {
	t.Run("directStartCommandExecution", func(t *testing.T) {
		yamlContent := `
name: direct-start-test
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "step1 output"
  - name: step2
    command: echo "step2 output"
    depends: [step1]
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		err := executeStartCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err, "Start command should succeed")

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 20*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		assertAllNodesSucceeded(t, status)
	})
}

