package distr_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func setupSharedFSWorker(t *testing.T, coord *test.Coordinator, workerID string, labels map[string]string) *worker.Worker {
	t.Helper()

	coordinatorClient := coord.GetCoordinatorClient(t)

	workerInst := worker.NewWorker(workerID, 10, coordinatorClient, labels, coord.Config)
	workerInst.SetHandler(worker.NewTaskHandler(coord.Config))

	return startAndCleanupWorkerWithID(t, coord, workerInst, workerID)
}

func TestSharedFS_StatusPersistence(t *testing.T) {
	t.Run("statusWrittenToSharedFilesystem", func(t *testing.T) {
		yamlContent := `
name: sharedfs-status-test
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

		setupSharedFSWorker(t, coord, "sharedfs-worker-1", map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err, "enqueue should succeed")

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond, "DAG should be enqueued")

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 20*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status, "DAG should succeed")
		require.Len(t, status.Nodes, 2, "should have 2 nodes")
		assertAllNodesSucceeded(t, status)
	})
}

func TestSharedFS_LogFilePersistence(t *testing.T) {
	t.Run("logsWrittenToSharedFilesystem", func(t *testing.T) {
		yamlContent := `
name: sharedfs-log-test
workerSelector:
  test: "true"
steps:
  - name: echo-step
    command: echo "test output"
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedFSWorker(t, coord, "sharedfs-worker-1", map[string]string{"test": "true"})

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

		require.Len(t, status.Nodes, 1, "should have 1 node")
		node := status.Nodes[0]
		require.Equal(t, "echo-step", node.Step.Name)
		require.NotEmpty(t, node.Stdout, "node should have stdout log file path set")
	})
}

func TestSharedFS_SubprocessExecution(t *testing.T) {
	t.Run("subprocessExecutesDAGCorrectly", func(t *testing.T) {
		yamlContent := `
name: sharedfs-subprocess-test
workerSelector:
  env: test
steps:
  - name: task1
    command: echo "subprocess task1"
  - name: task2
    command: echo "subprocess task2"
    depends: [task1]
  - name: task3
    command: echo "subprocess task3"
    depends: [task2]
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedFSWorker(t, coord, "subprocess-worker", map[string]string{"env": "test"})

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
		require.Len(t, status.Nodes, 3, "should have 3 nodes")
		assertAllNodesSucceeded(t, status)

		for _, node := range status.Nodes {
			require.NotEmpty(t, node.StartedAt, "node %s should have started", node.Step.Name)
			require.NotEmpty(t, node.FinishedAt, "node %s should have finished", node.Step.Name)
		}
	})
}

func TestSharedFS_QueueCleanup(t *testing.T) {
	t.Run("queueItemRemovedAfterSubprocessCompletion", func(t *testing.T) {
		yamlContent := `
name: sharedfs-queue-cleanup-test
workerSelector:
  test: "true"
steps:
  - name: cleanup-task
    command: echo "cleanup test"
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedFSWorker(t, coord, "cleanup-worker", map[string]string{"test": "true"})

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

		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil || status.Status != core.Succeeded {
				return false
			}

			items, err := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return err == nil && len(items) == 0
		}, 25*time.Second, 200*time.Millisecond, "Queue should be empty after subprocess completion")

		schedulerCancel()
	})
}
