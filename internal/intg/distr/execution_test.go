package distr_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecution_StatusPushing(t *testing.T) {
	t.Run("statusUpdatesPersistedToCoordinatorStore", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: status-push-test
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "step1"
  - name: step2
    command: echo "step2"
    depends: [step1]
`)
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		f.assertWorkerID(status, "worker-1")
		f.assertAllNodesSucceeded(status)
	})
}

func TestExecution_LogStreaming(t *testing.T) {
	t.Run("logsStreamedToCoordinatorFilesystem", func(t *testing.T) {
		expectedOutput := "EXPECTED_OUTPUT_12345"
		f := newTestFixture(t, `
name: log-stream-test
workerSelector:
  test: "true"
steps:
  - name: echo-step
    command: echo "`+expectedOutput+`"
`, withLogPersistence())
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		assertLogContains(t, f.logDir(), f.dagWrapper.Name, status.DAGRunID, "echo-step", expectedOutput)
	})
}

func TestExecution_LargeOutput(t *testing.T) {
	t.Run("largeOutputStreamedCorrectly", func(t *testing.T) {
		f := newTestFixture(t, `
name: large-output-test
workerSelector:
  test: "true"
steps:
  - name: big-output
    command: |
      for i in $(seq 1 2000); do
        echo "Line $i: This is a test line to generate large output that exceeds the 64KB buffer size used in log streaming"
      done
`, withLogPersistence())
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(60 * time.Second)

		status := f.waitForStatus(core.Succeeded, 45*time.Second)

		require.Equal(t, core.Succeeded, status.Status)

		logPath := assertLogExists(t, f.logDir(), f.dagWrapper.Name, status.DAGRunID, "big-output")

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

func TestExecution_StartCommand(t *testing.T) {
	t.Run("directStartCommandExecution", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: direct-start-test
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "step1 output"
  - name: step2
    command: echo "step2 output"
    depends: [step1]
`)
		defer f.cleanup()

		f.startScheduler(30 * time.Second)

		require.NoError(t, f.start())

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		f.assertAllNodesSucceeded(status)
	})

	t.Run("directStartCommandExecution_NoNameField", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "no name field"
  - name: step2
    command: echo "step2 output"
    depends: [step1]
`)
		defer f.cleanup()

		f.startScheduler(30 * time.Second)

		require.NoError(t, f.start())

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		f.assertAllNodesSucceeded(status)
	})
}

func TestExecution_SharedFSMode(t *testing.T) {
	t.Run("statusWrittenToSharedFilesystem", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: sharedfs-status-test
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "step1"
  - name: step2
    command: echo "step2"
    depends: [step1]
`, withWorkerMode(sharedFSMode))
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		f.assertAllNodesSucceeded(status)
	})

	t.Run("logsWrittenToSharedFilesystem", func(t *testing.T) {
		f := newTestFixture(t, `
name: sharedfs-log-test
workerSelector:
  test: "true"
steps:
  - name: echo-step
    command: echo "test output"
`, withWorkerMode(sharedFSMode))
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 1)
		node := status.Nodes[0]
		require.Equal(t, "echo-step", node.Step.Name)
		require.NotEmpty(t, node.Stdout, "node should have stdout log file path set")
	})

	t.Run("subprocessExecutesDAGCorrectly", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
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
`, withWorkerMode(sharedFSMode), withLabels(map[string]string{"env": "test"}))
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 25*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 3)
		f.assertAllNodesSucceeded(status)

		for _, node := range status.Nodes {
			require.NotEmpty(t, node.StartedAt, "node %s should have started", node.Step.Name)
			require.NotEmpty(t, node.FinishedAt, "node %s should have finished", node.Step.Name)
		}
	})

	t.Run("directStartWithSharedFS", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: sharedfs-direct-start-test
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "direct start"
  - name: step2
    command: sleep 0.1
    depends: [step1]
`, withWorkerMode(sharedFSMode))
		defer f.cleanup()

		f.startScheduler(30 * time.Second)
		require.NoError(t, f.start())

		status := f.waitForStatus(core.Succeeded, 20*time.Second)
		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		f.assertAllNodesSucceeded(status)
	})

	t.Run("directStartWithSharedFS_NoNameField", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
workerSelector:
  test: "true"
steps:
  - name: step1
    command: echo "no name field"
  - name: step2
    command: sleep 0.1
    depends: [step1]
`, withWorkerMode(sharedFSMode))
		defer f.cleanup()

		f.startScheduler(30 * time.Second)
		require.NoError(t, f.start())

		status := f.waitForStatus(core.Succeeded, 20*time.Second)
		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		f.assertAllNodesSucceeded(status)
	})
}

func TestExecution_QueueLifecycle(t *testing.T) {
	t.Run("queueItemRemovedAfterSuccess", func(t *testing.T) {
		f := newTestFixture(t, `
name: queue-cleanup-test
workerSelector:
  test: "true"
steps:
  - name: task1
    command: echo "done"
`)
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
			if err != nil || status.Status != core.Succeeded {
				return false
			}

			items, err := f.coord.QueueStore.ListByDAGName(f.coord.Context, f.dagWrapper.ProcGroup(), f.dagWrapper.Name)
			return err == nil && len(items) == 0
		}, 25*time.Second, 200*time.Millisecond, "Queue should be empty after success")
	})

	t.Run("queuedStatusBeforeSchedulerStarts", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: scheduler-process-test
workerSelector:
  env: prod
steps:
  - name: step1
    command: echo "step1"
  - name: step2
    command: echo "step2"
    depends: [step1]
`, withLabels(map[string]string{"env": "prod"}))
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()

		latest, err := f.latestStatus()
		require.NoError(t, err)
		require.Equal(t, core.Queued, latest.Status, "DAG should be in queued state before scheduler starts")

		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		f.assertAllNodesSucceeded(status)
	})
}
