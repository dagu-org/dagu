package distributed_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Cancellation Tests
// =============================================================================
// These tests verify cancellation propagation in distributed execution,
// including single tasks, sub-DAGs, and parallel execution.

func TestCancellation_SingleTask(t *testing.T) {
	t.Run("cancellationPropagatesToRemoteWorker", func(t *testing.T) {
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

		var dagRunID string
		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			if status.Status == core.Running {
				dagRunID = status.DAGRunID
				return true
			}
			return false
		}, 10*time.Second, 1000*time.Millisecond, "DAG should start running")

		startTime := time.Now()
		err = coord.DAGRunMgr.Stop(coord.Context, dagWrapper.DAG, dagRunID)
		require.NoError(t, err, "stop should succeed")

		status := waitForStatusIn(t, coord, dagWrapper.DAG, []core.Status{core.Aborted, core.Failed}, 15*time.Second)
		schedulerCancel()

		elapsed := time.Since(startTime)

		assert.Less(t, elapsed, 10*time.Second, "cancellation should be quick")
		assert.Contains(t, []core.Status{core.Aborted, core.Failed}, status.Status, "DAG should be aborted or failed")
	})
}

func TestCancellation_SubDAG(t *testing.T) {
	t.Run("parentCancelPropagatesToChildOnWorker", func(t *testing.T) {
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
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		coordinatorClient := coord.GetCoordinatorClient(t)

		setupSharedNothingWorker(t, coord, "test-worker-cancel", map[string]string{
			"foo": "bar",
		})

		dagWrapper := coord.DAG(t, yamlContent)

		err := executeStartCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err, "Start command should succeed")

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)

		schedulerDone := make(chan error, 1)
		go func() {
			schedulerDone <- schedulerInst.Start(schedulerCtx)
		}()

		var dagRunID string
		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			if status.Status == core.Running {
				dagRunID = status.DAGRunID
				return true
			}
			return false
		}, 10*time.Second, 200*time.Millisecond, "Timeout waiting for DAG to start running")

		err = coord.DAGRunMgr.Stop(coord.Context, dagWrapper.DAG, dagRunID)
		require.NoError(t, err, "Stop command should succeed")

		require.Eventually(t, func() bool {
			status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil {
				return false
			}
			return status.Status == core.Aborted || status.Status == core.Failed
		}, 15*time.Second, 500*time.Millisecond, "Timeout waiting for DAG to be cancelled")

		schedulerCancel()

		select {
		case <-schedulerDone:
		case <-time.After(5 * time.Second):
		}

		finalStatus, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.Contains(t, []core.Status{core.Aborted, core.Failed}, finalStatus.Status,
			"DAG should have been canceled or failed")
	})
}

func TestCancellation_ConcurrentWorkers(t *testing.T) {
	t.Run("cancellationWithHighConcurrency", func(t *testing.T) {
		yamlContent := `
steps:
  - name: high-concurrency
    call: child-task
    parallel:
      items:
        - "task1"
        - "task2"
        - "task3"
        - "task4"
        - "task5"
        - "task6"
      maxConcurrent: 2

---
name: child-task
workerSelector:
  type: test-worker
steps:
  - name: process
    command: |
      echo "Starting task $1"
      sleep 0.3
      echo "Completed task $1"
`
		tmpDir := t.TempDir()
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir), test.WithStatusPersistence(), test.WithLogPersistence())

		setupWorkers(t, coord, 3, SharedNothingMode, map[string]string{"type": "test-worker"})

		dagWrapper := coord.DAG(t, yamlContent)

		agent := dagWrapper.Agent()

		done := make(chan struct{})
		go func() {
			agent.Context = coord.Context
			_ = agent.Run(agent.Context)
			close(done)
		}()

		require.Eventually(t, func() bool {
			st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil || !st.Status.IsActive() || len(st.Nodes) == 0 {
				return false
			}
			concurrentNode := st.Nodes[0]
			return concurrentNode.Status == core.NodeRunning && len(concurrentNode.SubRuns) >= 2
		}, 10*time.Second, 100*time.Millisecond)

		agent.Signal(coord.Context, os.Signal(syscall.SIGTERM))

		<-done

		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)

		require.GreaterOrEqual(t, len(st.Nodes), 1)
		concurrentNode := st.Nodes[0]
		require.Equal(t, "high-concurrency", concurrentNode.Step.Name)

		if len(concurrentNode.SubRuns) > 0 {
			t.Logf("Created %d sub runs before cancellation", len(concurrentNode.SubRuns))
		}

		require.Contains(t, []core.NodeStatus{core.NodePartiallySucceeded, core.NodeAborted}, concurrentNode.Status,
			"expected node to be partially succeeded or aborted, got %v", concurrentNode.Status)
	})
}

func TestCancellation_GracefulShutdown(t *testing.T) {
	t.Run("gracefulShutdownOnSIGTERM", func(t *testing.T) {
		yamlContent := `
name: graceful-cancel-test
workerSelector:
  test: "true"
steps:
  - name: task1
    command: sleep 30
  - name: task2
    command: echo "should not run"
    depends: [task1]
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
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

		status := waitForRunning(t, coord, dagWrapper.DAG, 10*time.Second)

		err = coord.DAGRunMgr.Stop(coord.Context, dagWrapper.DAG, status.DAGRunID)
		require.NoError(t, err)

		finalStatus := waitForStatusIn(t, coord, dagWrapper.DAG, []core.Status{core.Aborted, core.Failed}, 15*time.Second)
		schedulerCancel()

		require.Contains(t, []core.Status{core.Aborted, core.Failed}, finalStatus.Status)

		// Task2 should not have started
		for _, node := range finalStatus.Nodes {
			if node.Step.Name == "task2" {
				require.NotEqual(t, core.NodeSucceeded, node.Status, "task2 should not have succeeded")
			}
		}
	})
}
