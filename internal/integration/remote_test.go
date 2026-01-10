package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemote_StatusPushing verifies status flows: Worker → Coordinator → DAGRunStore
func TestRemote_StatusPushing(t *testing.T) {
	t.Run("status updates persisted to coordinator store", func(t *testing.T) {
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
		// Setup coordinator with status persistence enabled
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		// Setup worker with remote task handler for shared-nothing mode
		setupRemoteWorker(t, coord, "worker-1", 10, map[string]string{"test": "true"})

		// Load and enqueue the DAG
		dagWrapper := coord.DAG(t, yamlContent)

		// Get coordinator client for scheduler
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Enqueue the DAG
		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		enqueueSpec := subCmdBuilder.Enqueue(dagWrapper.DAG, runtime.EnqueueOptions{Quiet: true})
		err := runtime.Start(coord.Context, enqueueSpec)
		require.NoError(t, err, "enqueue should succeed")

		// Wait for enqueue to complete
		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond, "DAG should be enqueued")

		// Start scheduler
		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		// Wait for completion
		status := waitForStatus(t, coord, dagWrapper.DAG, core.Succeeded, 20*time.Second)
		schedulerCancel()

		// Verify status details
		require.Equal(t, core.Succeeded, status.Status, "DAG should succeed")
		require.Len(t, status.Nodes, 2, "should have 2 nodes")
		assert.Equal(t, "worker-1", status.WorkerID, "worker ID should be recorded")

		for _, node := range status.Nodes {
			assert.Equal(t, core.NodeSucceeded, node.Status, "node %s should succeed", node.Step.Name)
		}
	})
}

// TestRemote_LogStreaming verifies logs flow: Worker → Coordinator → Filesystem
func TestRemote_LogStreaming(t *testing.T) {
	t.Run("logs streamed to coordinator filesystem", func(t *testing.T) {
		expectedOutput := "EXPECTED_OUTPUT_12345"
		yamlContent := `
name: log-stream-test
workerSelector:
  test: "true"
steps:
  - name: echo-step
    command: echo "` + expectedOutput + `"
`
		// Setup coordinator with both status and log persistence enabled
		coord := test.SetupCoordinator(t, test.WithStatusPersistence(), test.WithLogPersistence())
		coord.Config.Queues.Enabled = true

		// Setup worker with remote task handler for shared-nothing mode
		setupRemoteWorker(t, coord, "worker-1", 10, map[string]string{"test": "true"})

		// Load and enqueue the DAG
		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		enqueueSpec := subCmdBuilder.Enqueue(dagWrapper.DAG, runtime.EnqueueOptions{Quiet: true})
		err := runtime.Start(coord.Context, enqueueSpec)
		require.NoError(t, err)

		// Wait for enqueue
		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		// Start scheduler
		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		// Wait for completion
		status := waitForStatus(t, coord, dagWrapper.DAG, core.Succeeded, 20*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)

		// Verify log file exists and contains expected output
		assertLogFileContains(t, coord.LogDir(), dagWrapper.Name, status.DAGRunID, "echo-step", expectedOutput)
	})
}

// TestRemote_FullExecution verifies complete shared-nothing execution
func TestRemote_FullExecution(t *testing.T) {
	t.Run("full remote execution with status and logs", func(t *testing.T) {
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
		// Setup coordinator with both status and log persistence
		coord := test.SetupCoordinator(t,
			test.WithStatusPersistence(),
			test.WithLogPersistence(),
		)
		coord.Config.Queues.Enabled = true

		// Setup worker with remote task handler for shared-nothing mode
		setupRemoteWorker(t, coord, "worker-1", 10, map[string]string{"env": "prod"})

		// Load and enqueue the DAG
		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		enqueueSpec := subCmdBuilder.Enqueue(dagWrapper.DAG, runtime.EnqueueOptions{Quiet: true})
		err := runtime.Start(coord.Context, enqueueSpec)
		require.NoError(t, err)

		// Wait for enqueue
		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		// Start scheduler
		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		// Wait for completion
		status := waitForStatus(t, coord, dagWrapper.DAG, core.Succeeded, 20*time.Second)
		schedulerCancel()

		// Verify status
		require.Equal(t, core.Succeeded, status.Status)
		require.Len(t, status.Nodes, 2)
		assert.Equal(t, "worker-1", status.WorkerID)

		// Verify all nodes succeeded
		for _, node := range status.Nodes {
			assert.Equal(t, core.NodeSucceeded, node.Status)
		}

		// Verify logs for both tasks
		assertLogFileContains(t, coord.LogDir(), dagWrapper.Name, status.DAGRunID, "task1", "task1 output")
		assertLogFileContains(t, coord.LogDir(), dagWrapper.Name, status.DAGRunID, "task2", "task2 output")
	})
}

// TestRemote_SubDAG verifies sub-DAG status propagation through coordinator
func TestRemote_SubDAG(t *testing.T) {
	t.Run("sub-DAG execution via coordinator", func(t *testing.T) {
		// Multi-document YAML with parent and child DAGs
		// Both parent and child have workerSelector so they run on workers
		yamlContent := `
name: parent-remote
workerSelector:
  type: parent
steps:
  - run: child-remote
---
name: child-remote
workerSelector:
  type: child
steps:
  - name: child-step
    command: echo "child executed"
`
		// Setup coordinator with status persistence
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		// Setup workers - one for parent and one for child
		setupRemoteWorker(t, coord, "parent-worker", 10, map[string]string{"type": "parent"})
		setupRemoteWorker(t, coord, "child-worker", 10, map[string]string{"type": "child"})

		// Load the DAG
		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Enqueue the parent DAG to coordinator (runs on worker with coordinator client)
		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		enqueueSpec := subCmdBuilder.Enqueue(dagWrapper.DAG, runtime.EnqueueOptions{Quiet: true})
		err := runtime.Start(coord.Context, enqueueSpec)
		require.NoError(t, err)

		// Verify the DAG was queued
		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		// Start scheduler to handle DAG dispatch
		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		// Wait for parent to complete (which means child also completed)
		status := waitForStatus(t, coord, dagWrapper.DAG, core.Succeeded, 25*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status, "parent DAG should succeed")
	})
}

// TestRemote_LargeOutput verifies buffer flushing for outputs exceeding 64KB
func TestRemote_LargeOutput(t *testing.T) {
	t.Run("large output streamed correctly", func(t *testing.T) {
		// Generate output exceeding 64KB buffer
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

		// Setup worker with remote task handler for shared-nothing mode
		setupRemoteWorker(t, coord, "worker-1", 10, map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		enqueueSpec := subCmdBuilder.Enqueue(dagWrapper.DAG, runtime.EnqueueOptions{Quiet: true})
		err := runtime.Start(coord.Context, enqueueSpec)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 60*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		// Wait for completion (may take longer due to large output)
		status := waitForStatus(t, coord, dagWrapper.DAG, core.Succeeded, 45*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status)

		// Verify log file exists and has significant size
		logPath := assertLogFileExists(t, coord.LogDir(), dagWrapper.Name, status.DAGRunID, "big-output")

		fileInfo, err := os.Stat(logPath)
		require.NoError(t, err)
		assert.Greater(t, fileInfo.Size(), int64(64*1024), "log file should exceed 64KB")

		// Verify content integrity
		content := getLogFileContent(t, logPath)
		assert.Contains(t, content, "Line 1:")
		assert.Contains(t, content, "Line 2000:")

		// Count lines to verify no truncation
		lineCount := strings.Count(content, "\n")
		assert.GreaterOrEqual(t, lineCount, 2000, "should have at least 2000 lines")
	})
}

// TestRemote_QueueDispatchWithPreviousStatus verifies that queued DAGs receive PreviousStatus
// when dispatched to workers in shared-nothing mode.
// This test ensures the fix for "retry requires either previous_status in task or local dagRunStore"
// is working correctly.
func TestRemote_QueueDispatchWithPreviousStatus(t *testing.T) {
	t.Run("queued DAG receives previous status on dispatch", func(t *testing.T) {
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
		// Setup coordinator with status persistence (required for shared-nothing mode)
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		// Setup worker with remote task handler (shared-nothing mode)
		// This worker has NO access to local dagRunStore - it relies on PreviousStatus from task
		setupRemoteWorker(t, coord, "worker-1", 10, map[string]string{"test": "true"})

		// Load and enqueue the DAG
		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		enqueueSpec := subCmdBuilder.Enqueue(dagWrapper.DAG, runtime.EnqueueOptions{Quiet: true})
		err := runtime.Start(coord.Context, enqueueSpec)
		require.NoError(t, err, "enqueue should succeed")

		// Verify the DAG was queued
		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond, "DAG should be enqueued")

		// Start scheduler - this triggers queue processing which dispatches with PreviousStatus
		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		// Wait for completion - if PreviousStatus is not passed, this would fail with:
		// "retry requires either previous_status in task or local dagRunStore"
		status := waitForStatus(t, coord, dagWrapper.DAG, core.Succeeded, 20*time.Second)
		schedulerCancel()

		// Verify execution succeeded
		require.Equal(t, core.Succeeded, status.Status, "DAG should succeed")
		require.Len(t, status.Nodes, 2, "should have 2 nodes")

		// Verify both steps executed
		for _, node := range status.Nodes {
			assert.Equal(t, core.NodeSucceeded, node.Status, "node %s should succeed", node.Step.Name)
		}
	})
}

// TestRemote_Cancellation verifies cancellation propagates to workers
func TestRemote_Cancellation(t *testing.T) {
	t.Run("cancellation propagates to remote worker", func(t *testing.T) {
		yamlContent := `
name: cancel-test
workerSelector:
  test: "true"
steps:
  - name: long-task
    command: sleep 60
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		// Setup worker with remote task handler for shared-nothing mode
		setupRemoteWorker(t, coord, "worker-1", 10, map[string]string{"test": "true"})

		dagWrapper := coord.DAG(t, yamlContent)
		coordinatorClient := coord.GetCoordinatorClient(t)

		subCmdBuilder := runtime.NewSubCmdBuilder(coord.Config)
		enqueueSpec := subCmdBuilder.Enqueue(dagWrapper.DAG, runtime.EnqueueOptions{Quiet: true})
		err := runtime.Start(coord.Context, enqueueSpec)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			items, _ := coord.QueueStore.ListByDAGName(coord.Context, dagWrapper.ProcGroup(), dagWrapper.Name)
			return len(items) == 1
		}, 2*time.Second, 50*time.Millisecond)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupSchedulerWithCoordinator(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		// Wait for the DAG to start running
		var dagRunID string
		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			if status.Status == core.Running {
				dagRunID = status.DAGRunID
				t.Logf("DAG is running with ID: %s", dagRunID)
				return true
			}
			return false
		}, 10*time.Second, 200*time.Millisecond, "DAG should start running")

		// Send cancellation signal
		t.Log("Sending cancellation signal...")
		startTime := time.Now()
		err = coord.DAGRunMgr.Stop(coord.Context, dagWrapper.DAG, dagRunID)
		require.NoError(t, err, "stop should succeed")

		// Wait for cancellation to take effect
		status := waitForStatusIn(t, coord, dagWrapper.DAG, []core.Status{core.Aborted, core.Failed}, 15*time.Second)
		schedulerCancel()

		elapsed := time.Since(startTime)

		// Verify cancellation was quick (not waiting 60 seconds)
		assert.Less(t, elapsed, 10*time.Second, "cancellation should be quick")
		assert.Contains(t, []core.Status{core.Aborted, core.Failed}, status.Status, "DAG should be aborted or failed")
	})
}
