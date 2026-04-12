// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package distr_test

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/stringutil"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecution_StatusPushing(t *testing.T) {
	t.Run("statusUpdatesPersistedToCoordinatorStore", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: status-push-test
worker_selector:
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
worker_selector:
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
		command := `      for i in $(seq 1 2000); do
        echo "Line $i: This is a test line to generate large output that exceeds the 64KB buffer size used in log streaming"
      done`
		if runtime.GOOS == "windows" {
			command = `      1..2000 | ForEach-Object {
        Write-Output ("Line {0}: This is a test line to generate large output that exceeds the 64KB buffer size used in log streaming" -f $_)
      }`
		}

		f := newTestFixture(t, `
name: large-output-test
worker_selector:
  test: "true"
steps:
  - name: big-output
    command: |
`+command+`
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
worker_selector:
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
worker_selector:
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

func TestExecution_TagsPropagation(t *testing.T) {
	t.Run("tagsPreservedThroughCoordinator", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: tags-propagation-test
worker_selector:
  test: "true"
steps:
  - name: step1
    command: echo "tagged run"
`)
		defer f.cleanup()

		f.startScheduler(30 * time.Second)

		require.NoError(t, f.startWithTags("env=prod,team=backend"))

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Contains(t, status.Tags, "env=prod")
		require.Contains(t, status.Tags, "team=backend")
	})

	t.Run("tagsPreservedThroughCoordinator_SharedFS", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: tags-sharedfs-test
worker_selector:
  test: "true"
steps:
  - name: step1
    command: echo "tagged sharedfs run"
`, withWorkerMode(sharedFSMode))
		defer f.cleanup()

		f.startScheduler(30 * time.Second)

		require.NoError(t, f.startWithTags("region=us-east-1"))

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		require.Contains(t, status.Tags, "region=us-east-1")
	})
}

func TestExecution_SharedFSMode(t *testing.T) {
	t.Run("statusWrittenToSharedFilesystem", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: sharedfs-status-test
worker_selector:
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
worker_selector:
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
worker_selector:
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
worker_selector:
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
worker_selector:
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

func TestExecution_WorkDir(t *testing.T) {
	t.Run("sharedNothingWorkDir", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: workdir-shared-nothing-test
worker_selector:
  test: "true"
steps:
  - name: write-to-workdir
    command: echo "hello" > "${DAG_RUN_WORK_DIR}/test.txt"
  - name: read-from-workdir
    command: cat "${DAG_RUN_WORK_DIR}/test.txt"
    depends: [write-to-workdir]
`, withLogPersistence())
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		f.assertAllNodesSucceeded(status)
		assertLogContains(t, f.logDir(), f.dagWrapper.Name, status.DAGRunID, "read-from-workdir", "hello")
	})

	t.Run("sharedFSWorkDir", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: workdir-sharedfs-test
worker_selector:
  test: "true"
steps:
  - name: write-to-workdir
    command: echo "world" > "${DAG_RUN_WORK_DIR}/data.txt"
  - name: read-from-workdir
    command: cat "${DAG_RUN_WORK_DIR}/data.txt"
    depends: [write-to-workdir]
`, withWorkerMode(sharedFSMode), withLogPersistence())
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
		f.assertAllNodesSucceeded(status)
	})
}

func TestExecution_QueueLifecycle(t *testing.T) {
	t.Run("queueItemRemovedAfterSuccess", func(t *testing.T) {
		f := newTestFixture(t, `
name: queue-cleanup-test
worker_selector:
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
worker_selector:
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

func TestExecution_QueuedCatchupHappyPath(t *testing.T) {
	t.Run("sharedNothingPreservesCatchupMetadata", func(t *testing.T) {
		scheduleTime := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
		expectedOutput := "distributed-catchup-remote"

		f := newTestFixture(t, `
name: distributed-catchup-remote-test
worker_selector:
  test: "true"
steps:
  - name: echo-step
    command: echo "`+expectedOutput+`"
`, withLogPersistence())
		defer f.cleanup()

		runID, err := f.enqueueCatchup(scheduleTime)
		require.NoError(t, err)

		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, runID, status.DAGRunID)
		require.Equal(t, core.TriggerTypeCatchUp, status.TriggerType)
		require.Equal(t, stringutil.FormatTime(scheduleTime), status.ScheduleTime)
		require.NotEmpty(t, status.Log)
		f.assertWorkerID(status, "worker-1")
		f.assertAllNodesSucceeded(status)
		assertLogContains(t, f.logDir(), f.dagWrapper.Name, status.DAGRunID, "echo-step", expectedOutput)
	})

	t.Run("sharedFSPreservesCatchupMetadata", func(t *testing.T) {
		scheduleTime := time.Date(2026, 3, 13, 11, 0, 0, 0, time.UTC)
		expectedOutput := "distributed-catchup-sharedfs"

		f := newTestFixture(t, `
name: distributed-catchup-sharedfs-test
worker_selector:
  test: "true"
steps:
  - name: echo-step
    command: echo "`+expectedOutput+`"
`, withWorkerMode(sharedFSMode))
		defer f.cleanup()

		runID, err := f.enqueueCatchup(scheduleTime)
		require.NoError(t, err)

		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, runID, status.DAGRunID)
		require.Equal(t, core.TriggerTypeCatchUp, status.TriggerType)
		require.Equal(t, stringutil.FormatTime(scheduleTime), status.ScheduleTime)
		require.NotEmpty(t, status.Log)
		require.FileExists(t, status.Log)
		require.Len(t, status.Nodes, 1)
		require.NotEmpty(t, status.Nodes[0].Stdout)
		require.FileExists(t, status.Nodes[0].Stdout)
		f.assertWorkerID(status, "worker-1")
		f.assertAllNodesSucceeded(status)
		content, err := os.ReadFile(status.Nodes[0].Stdout)
		require.NoError(t, err)
		assert.Contains(t, string(content), expectedOutput)
	})
}
