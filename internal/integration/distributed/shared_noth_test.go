package distributed_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Shared-Nothing Mode Tests
// =============================================================================
// These tests verify distributed execution where workers have NO filesystem access
// to the coordinator. Workers use RemoteTaskHandler to:
// - Push status updates to coordinator via gRPC
// - Stream logs to coordinator via gRPC

func TestSharedNothing_StatusPushing(t *testing.T) {
	t.Run("statusUpdatesPersistedToCoordinatorStore", func(t *testing.T) {
		yamlContent := `
name: status-push-test
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

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"test": "true"})

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
		assertWorkerID(t, status, "worker-1")

		assertAllNodesSucceeded(t, status)
	})
}

func TestSharedNothing_LogStreaming(t *testing.T) {
	t.Run("logsStreamedToCoordinatorFilesystem", func(t *testing.T) {
		expectedOutput := "EXPECTED_OUTPUT_12345"
		yamlContent := `
name: log-stream-test
workerSelector:
  test: "true"
steps:
  - name: echo-step
    command: echo "` + expectedOutput + `"
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence(), test.WithLogPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"test": "true"})

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
		assertLogContains(t, coord.LogDir(), dagWrapper.Name, status.DAGRunID, "echo-step", expectedOutput)
	})
}

func TestSharedNothing_FullExecution(t *testing.T) {
	t.Run("fullRemoteExecutionWithStatusAndLogs", func(t *testing.T) {
		yamlContent := `
name: full-remote-test
workerSelector:
  env: prod
steps:
  - name: task1
    command: echo "task1 output"
  - name: task2
    command: echo "task2 output"
    depends: [task1]
`
		coord := test.SetupCoordinator(t,
			test.WithStatusPersistence(),
			test.WithLogPersistence(),
		)
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"env": "prod"})

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
		require.Len(t, status.Nodes, 2)
		assertWorkerID(t, status, "worker-1")
		assertAllNodesSucceeded(t, status)

		assertLogContains(t, coord.LogDir(), dagWrapper.Name, status.DAGRunID, "task1", "task1 output")
		assertLogContains(t, coord.LogDir(), dagWrapper.Name, status.DAGRunID, "task2", "task2 output")
	})
}

func TestSharedNothing_LargeOutput(t *testing.T) {
	t.Run("largeOutputStreamedCorrectly", func(t *testing.T) {
		yamlContent := `
name: large-output-test
workerSelector:
  test: "true"
steps:
  - name: big-output
    command: |
      for i in $(seq 1 2000); do
        echo "Line $i: This is a test line to generate large output that exceeds the 64KB buffer size used in log streaming"
      done
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence(), test.WithLogPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		err := executeEnqueueCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 60*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 45*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)

		logPath := assertLogExists(t, coord.LogDir(), dagWrapper.Name, status.DAGRunID, "big-output")

		fileInfo, err := os.Stat(logPath)
		require.NoError(t, err)
		assert.Greater(t, fileInfo.Size(), int64(64*1024), "log file should exceed 64KB")

		content := getLogContent(t, logPath)
		assert.Contains(t, content, "Line 1:")
		assert.Contains(t, content, "Line 2000:")

		lineCount := strings.Count(content, "\n")
		assert.GreaterOrEqual(t, lineCount, 2000, "should have at least 2000 lines")
	})
}

func TestSharedNothing_PreviousStatusPassing(t *testing.T) {
	t.Run("queuedDAGReceivesPreviousStatusOnDispatch", func(t *testing.T) {
		yamlContent := `
name: queue-status-test
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "step1 executed"
  - name: step2
    command: echo "step2 executed"
    depends: [step1]
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		setupSharedNothingWorker(t, coord, "worker-1", map[string]string{"test": "true"})

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
