package distr_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRetryCommand_WithWorkerSelector(t *testing.T) {
	t.Run("retryDispatchesToCoordinator", func(t *testing.T) {
		yamlContent := `
name: retry-cmd-test
workerSelector:
  test: "true"
steps:
  - name: task1
    command: echo "task1 executed"
  - name: task2
    command: echo "task2 executed"
    depends: [task1]
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "retry-worker-1", map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err, "enqueue should succeed")

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 20*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)
		dagRunID := status.DAGRunID

		schedulerCtx2, schedulerCancel2 := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel2()

		schedulerInst2 := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst2.Start(schedulerCtx2) }()

		err = executeRetryCommand(t, coord, dagWrapper.DAG, dagRunID)
		require.NoError(t, err, "retry command should succeed")

		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			return status.Status == core.Succeeded && status.DAGRunID == dagRunID
		}, 25*time.Second, 200*time.Millisecond, "Retry should complete successfully")

		schedulerCancel2()

		finalStatus, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, finalStatus.Status)
		assertAllNodesSucceeded(t, finalStatus)
	})
}

func TestRetryCommand_PartialRetry(t *testing.T) {
	t.Run("retryReusesSameRunID", func(t *testing.T) {
		yamlContent := `
name: partial-retry-test
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "step1"
  - name: step2
    command: echo "step2"
    depends: [step1]
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "partial-retry-worker", map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 20*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)
		originalRunID := status.DAGRunID

		schedulerCtx2, schedulerCancel2 := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel2()

		schedulerInst2 := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst2.Start(schedulerCtx2) }()

		err = executeRetryCommand(t, coord, dagWrapper.DAG, originalRunID)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			return status.Status == core.Succeeded && status.DAGRunID == originalRunID
		}, 25*time.Second, 200*time.Millisecond, "Retry should complete with same run ID")

		schedulerCancel2()

		finalStatus, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, finalStatus.Status)
		require.Equal(t, originalRunID, finalStatus.DAGRunID, "retry should maintain the same run ID")
	})
}

func TestRetryCommand_SharedFSMode(t *testing.T) {
	t.Run("retryWorksWithSharedFSWorker", func(t *testing.T) {
		yamlContent := `
name: retry-sharedfs-test
workerSelector:
  test: "true"
steps:
  - name: task1
    command: echo "sharedfs task1"
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedFSWorker(t, coord, "retry-sharedfs-worker", map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 25*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)
		dagRunID := status.DAGRunID

		schedulerCtx2, schedulerCancel2 := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel2()

		schedulerInst2 := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst2.Start(schedulerCtx2) }()

		err = executeRetryCommand(t, coord, dagWrapper.DAG, dagRunID)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			return status.Status == core.Succeeded
		}, 25*time.Second, 200*time.Millisecond)

		schedulerCancel2()
	})
}
