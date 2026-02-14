package distr_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCancellation_SingleTask(t *testing.T) {
	t.Run("cancellationPropagatesToRemoteWorker", func(t *testing.T) {
		f := newTestFixture(t, `
name: cancel-test
worker_selector:
  test: "true"
steps:
  - name: long-task
    command: sleep 60
`)
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		var dagRunID string
		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
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
		require.NoError(t, f.stop(dagRunID))

		status := f.waitForStatusIn([]core.Status{core.Aborted, core.Failed}, 15*time.Second)

		elapsed := time.Since(startTime)
		assert.Less(t, elapsed, 10*time.Second, "cancellation should be quick")
		assert.Contains(t, []core.Status{core.Aborted, core.Failed}, status.Status)
	})
}

func TestCancellation_SubDAG(t *testing.T) {
	t.Run("parentCancelPropagatesToChildOnWorker", func(t *testing.T) {
		f := newTestFixture(t, `
steps:
  - call: dotest
params:
  - URL: default_value
---
name: dotest
worker_selector:
  foo: bar
steps:
  - name: long-sleep
    command: sleep 30
`, withLabels(map[string]string{"foo": "bar"}))
		defer f.cleanup()

		require.NoError(t, f.start())
		f.startScheduler(30 * time.Second)

		var dagRunID string
		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
			if err != nil {
				return false
			}
			if status.Status == core.Running {
				dagRunID = status.DAGRunID
				return true
			}
			return false
		}, 10*time.Second, 200*time.Millisecond, "Timeout waiting for DAG to start running")

		require.NoError(t, f.stop(dagRunID))

		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
			if err != nil {
				return false
			}
			return status.Status == core.Aborted || status.Status == core.Failed
		}, 15*time.Second, 500*time.Millisecond, "Timeout waiting for DAG to be cancelled")

		finalStatus, err := f.latestStatus()
		require.NoError(t, err)
		require.Contains(t, []core.Status{core.Aborted, core.Failed}, finalStatus.Status)
	})

	t.Run("cancelPropagatesToSubDAGOnWorker", func(t *testing.T) {
		f := newTestFixture(t, `
steps:
  - name: run-local-on-worker
    call: local-sub
    output: RESULT

---
name: local-sub
worker_selector:
  type: test-worker
steps:
  - name: worker-task
    command: sleep 1000
    output: MESSAGE
`, withLabels(map[string]string{"type": "test-worker"}))

		runID := uuid.New().String()
		agent := f.dagWrapper.Agent(test.WithDAGRunID(runID))

		done := make(chan struct{})
		go func() {
			agent.RunCancel(t)
			close(done)
		}()

		f.dagWrapper.AssertLatestStatus(t, core.Running)

		require.NoError(t, f.stop(runID))

		f.dagWrapper.AssertLatestStatus(t, core.Aborted)

		<-done
	})
}

func TestCancellation_ConcurrentWorkers(t *testing.T) {
	t.Run("cancellationWithHighConcurrency", func(t *testing.T) {
		tmpDir := t.TempDir()
		f := newTestFixture(t, `
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
      max_concurrent: 2

---
name: child-task
worker_selector:
  type: test-worker
steps:
  - name: process
    command: |
      echo "Starting task $1"
      sleep 0.3
      echo "Completed task $1"
`, withWorkerCount(3), withLabels(map[string]string{"type": "test-worker"}),
			withDAGsDir(tmpDir), withLogPersistence())

		agent := f.dagWrapper.Agent()

		done := make(chan struct{})
		go func() {
			agent.Context = f.coord.Context
			_ = agent.Run(agent.Context)
			close(done)
		}()

		require.Eventually(t, func() bool {
			st, err := f.latestStatus()
			if err != nil || !st.Status.IsActive() || len(st.Nodes) == 0 {
				return false
			}
			concurrentNode := st.Nodes[0]
			return concurrentNode.Status == core.NodeRunning && len(concurrentNode.SubRuns) >= 2
		}, 10*time.Second, 100*time.Millisecond)

		agent.Signal(f.coord.Context, os.Signal(syscall.SIGTERM))

		<-done

		st, err := f.latestStatus()
		require.NoError(t, err)
		require.NotNil(t, st)

		require.GreaterOrEqual(t, len(st.Nodes), 1)
		concurrentNode := st.Nodes[0]
		require.Equal(t, "high-concurrency", concurrentNode.Step.Name)

		require.Contains(t, []core.NodeStatus{core.NodePartiallySucceeded, core.NodeAborted}, concurrentNode.Status)
	})
}

func TestCancellation_GracefulShutdown(t *testing.T) {
	t.Run("gracefulShutdownOnSIGTERM", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: graceful-cancel-test
worker_selector:
  test: "true"
steps:
  - name: task1
    command: sleep 30
  - name: task2
    command: echo "should not run"
    depends: [task1]
`)
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Running, 10*time.Second)

		require.NoError(t, f.stop(status.DAGRunID))

		finalStatus := f.waitForStatusIn([]core.Status{core.Aborted, core.Failed}, 15*time.Second)

		require.Contains(t, []core.Status{core.Aborted, core.Failed}, finalStatus.Status)

		for _, node := range finalStatus.Nodes {
			if node.Step.Name == "task2" {
				require.NotEqual(t, core.NodeSucceeded, node.Status, "task2 should not have succeeded")
			}
		}
	})
}

func TestCancellation_ParallelItems(t *testing.T) {
	t.Run("cancelParallelExecutionOnWorkers", func(t *testing.T) {
		tmpDir := t.TempDir()
		f := newTestFixture(t, `
steps:
  - name: process-items
    call: child-sleep
    parallel:
      items:
        - "100"
        - "101"
        - "102"
        - "103"
      max_concurrent: 2

---
name: child-sleep
worker_selector:
  type: test-worker
steps:
  - name: sleep
    command: sleep $1
`, withWorkerCount(2), withLabels(map[string]string{"type": "test-worker"}),
			withDAGsDir(tmpDir), withLogPersistence())

		agent := f.dagWrapper.Agent()
		done := make(chan struct{})

		go func() {
			agent.Context = f.coord.Context
			_ = agent.Run(agent.Context)
			close(done)
		}()

		require.Eventually(t, func() bool {
			st, err := f.latestStatus()
			if err != nil || !st.Status.IsActive() {
				return false
			}
			if len(st.Nodes) == 0 {
				return false
			}
			parallelNode := st.Nodes[0]
			return parallelNode.Status == core.NodeRunning
		}, 5*time.Second, 100*time.Millisecond)

		require.Eventually(t, func() bool {
			workerInfo, err := f.coordinatorClient.GetWorkers(f.coord.Context)
			require.NoError(t, err)
			var runningTasks int
			for _, w := range workerInfo {
				runningTasks += len(w.RunningTasks)
			}
			return runningTasks > 0
		}, 5*time.Second, 100*time.Millisecond)

		agent.Signal(f.coord.Context, os.Signal(syscall.SIGINT))

		<-done

		st, err := f.latestStatus()
		require.NoError(t, err)
		require.NotNil(t, st)

		require.GreaterOrEqual(t, len(st.Nodes), 1)
		parallelNode := st.Nodes[0]
		require.Equal(t, "process-items", parallelNode.Step.Name)
		require.Equal(t, core.NodeAborted, parallelNode.Status)
		require.NotEmpty(t, parallelNode.SubRuns)
	})
}

func TestRetry_WithWorkerSelector(t *testing.T) {
	t.Run("retryDispatchesToCoordinator", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: retry-cmd-test
worker_selector:
  test: "true"
steps:
  - name: task1
    command: echo "task1 executed"
  - name: task2
    command: echo "task2 executed"
    depends: [task1]
`)
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)
		dagRunID := status.DAGRunID
		f.cleanup()

		f.startScheduler(30 * time.Second)

		require.NoError(t, f.retry(dagRunID))

		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
			if err != nil {
				return false
			}
			return status.Status == core.Succeeded && status.DAGRunID == dagRunID
		}, 25*time.Second, 200*time.Millisecond, "Retry should complete successfully")

		finalStatus, err := f.latestStatus()
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, finalStatus.Status)
		f.assertAllNodesSucceeded(finalStatus)
	})

	t.Run("retryDispatchesToCoordinator_NoNameField", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
worker_selector:
  test: "true"
steps:
  - name: task1
    command: echo "task1 executed"
  - name: task2
    command: echo "task2 executed"
    depends: [task1]
`)
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 20*time.Second)
		dagRunID := status.DAGRunID
		f.cleanup()

		f.startScheduler(30 * time.Second)

		require.NoError(t, f.retry(dagRunID))

		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
			if err != nil {
				return false
			}
			return status.Status == core.Succeeded && status.DAGRunID == dagRunID
		}, 25*time.Second, 200*time.Millisecond, "Retry should complete successfully")

		finalStatus, err := f.latestStatus()
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, finalStatus.Status)
		f.assertAllNodesSucceeded(finalStatus)
	})
}

func TestRetry_PartialRetry(t *testing.T) {
	t.Run("retryReusesSameRunID", func(t *testing.T) {
		f := newTestFixture(t, `
type: graph
name: partial-retry-test
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
		originalRunID := status.DAGRunID
		f.cleanup()

		f.startScheduler(30 * time.Second)

		require.NoError(t, f.retry(originalRunID))

		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
			if err != nil {
				return false
			}
			return status.Status == core.Succeeded && status.DAGRunID == originalRunID
		}, 25*time.Second, 200*time.Millisecond, "Retry should complete with same run ID")

		finalStatus, err := f.latestStatus()
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, finalStatus.Status)
		require.Equal(t, originalRunID, finalStatus.DAGRunID, "retry should maintain the same run ID")
	})
}

func TestRetry_SharedFSMode(t *testing.T) {
	t.Run("retryWorksWithSharedFSWorker", func(t *testing.T) {
		f := newTestFixture(t, `
name: retry-sharedfs-test
worker_selector:
  test: "true"
steps:
  - name: task1
    command: echo "sharedfs task1"
`, withWorkerMode(sharedFSMode))
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 25*time.Second)
		dagRunID := status.DAGRunID
		f.cleanup()

		ctx, cancel := context.WithTimeout(f.coord.Context, 30*time.Second)
		defer cancel()

		f.schedulerCtx = ctx
		f.schedulerCancel = cancel
		f.startScheduler(30 * time.Second)

		require.NoError(t, f.retry(dagRunID))

		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
			if err != nil {
				return false
			}
			return status.Status == core.Succeeded
		}, 25*time.Second, 200*time.Millisecond)
	})

	t.Run("retryWorksWithSharedFSWorker_NoNameField", func(t *testing.T) {
		f := newTestFixture(t, `
worker_selector:
  test: "true"
steps:
  - name: task1
    command: echo "sharedfs task1"
`, withWorkerMode(sharedFSMode))
		defer f.cleanup()

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 25*time.Second)
		dagRunID := status.DAGRunID
		f.cleanup()

		ctx, cancel := context.WithTimeout(f.coord.Context, 30*time.Second)
		defer cancel()

		f.schedulerCtx = ctx
		f.schedulerCancel = cancel
		f.startScheduler(30 * time.Second)

		require.NoError(t, f.retry(dagRunID))

		require.Eventually(t, func() bool {
			status, err := f.latestStatus()
			if err != nil {
				return false
			}
			return status.Status == core.Succeeded
		}, 25*time.Second, 200*time.Millisecond)
	})
}
