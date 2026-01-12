package distr_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Sub-DAG Execution Tests
// =============================================================================
// These tests verify sub-DAG execution in distributed mode, including:
// - Local parent calling distributed child
// - Failure propagation from child to parent
// - Output collection from sub-DAGs

func TestSubDAG_LocalCallsDistributed(t *testing.T) {
	t.Run("localParentCallsDistributedChild", func(t *testing.T) {
		yamlContent := `
steps:
  - name: run-local-on-worker
    call: local-sub
    output: RESULT

---
name: local-sub
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: echo "Hello from worker"
    output: MESSAGE
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		setupSharedNothingWorker(t, coord, "test-worker-1", map[string]string{"type": "test-worker"})

		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()

		agent.RunSuccess(t)

		dagWrapper.AssertLatestStatus(t, core.Succeeded)
	})
}

func TestSubDAG_FailurePropagation(t *testing.T) {
	t.Run("childFailurePropagatesToParent", func(t *testing.T) {
		yamlContent := `
steps:
  - name: run-local-on-worker
    call: local-sub

---
name: local-sub
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: |
      echo "Start task"
      exit 1
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		setupSharedNothingWorker(t, coord, "test-worker-1", map[string]string{"type": "test-worker"})

		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()

		err := agent.Run(agent.Context)
		require.Error(t, err)

		dagWrapper.AssertLatestStatus(t, core.Failed)

		st, statusErr := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, statusErr)
		require.Len(t, st.Nodes, 1)

		node := st.Nodes[0]
		require.Equal(t, "run-local-on-worker", node.Step.Name)
		require.Equal(t, core.NodeFailed, node.Status)
		require.Len(t, node.SubRuns, 1)
	})
}

func TestSubDAG_NoMatchingWorker(t *testing.T) {
	t.Run("failsWhenNoWorkerMatchesSelector", func(t *testing.T) {
		yamlContent := `
steps:
  - name: run-on-nonexistent-worker
    call: local-sub
    output: RESULT

---

name: local-sub
workerSelector:
  type: nonexistent-worker
steps:
  - name: worker-task
    command: echo "Should not run"
    output: MESSAGE
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()

		ctx, cancel := context.WithTimeout(coord.Context, 5*time.Second)
		defer cancel()
		err := agent.Run(ctx)
		require.Error(t, err)

		st := agent.Status(coord.Context)
		require.NotEqual(t, core.Succeeded, st.Status)
	})
}

func TestSubDAG_Cancellation(t *testing.T) {
	t.Run("cancelPropagatesToSubDAGOnWorker", func(t *testing.T) {
		yamlContent := `
steps:
  - name: run-local-on-worker
    call: local-sub
    output: RESULT

---
name: local-sub
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: sleep 1000
    output: MESSAGE
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		setupSharedNothingWorker(t, coord, "test-worker-1", map[string]string{"type": "test-worker"})

		dagWrapper := coord.DAG(t, yamlContent)

		runID := uuid.New().String()
		agent := dagWrapper.Agent(test.WithDAGRunID(runID))

		done := make(chan struct{})
		go func() {
			agent.RunCancel(t)
			close(done)
		}()

		dagWrapper.AssertLatestStatus(t, core.Running)

		err := coord.DAGRunMgr.Stop(coord.Context, agent.DAG, runID)
		require.NoError(t, err)

		dagWrapper.AssertLatestStatus(t, core.Aborted)

		<-done
	})
}

func TestSubDAG_DifferentWorkers(t *testing.T) {
	t.Run("parentAndChildOnDifferentWorkers", func(t *testing.T) {
		parentYAML := `
name: parent-remote
workerSelector:
  type: parent
steps:
  - run: child-remote
`
		childYAML := `
name: child-remote
workerSelector:
  type: child
steps:
  - name: child-step
    command: echo "child executed"
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		coord.CreateDAGFile(t, coord.Config.Paths.DAGsDir, "child-remote", []byte(childYAML))

		setupSharedNothingWorker(t, coord, "parent-worker", map[string]string{"type": "parent"})
		setupSharedNothingWorker(t, coord, "child-worker", map[string]string{"type": "child"})

		dagWrapper := coord.DAG(t, parentYAML)
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

		require.Equal(t, core.Succeeded, status.Status, "parent DAG should succeed")
	})
}

func TestSubDAG_InSameFile(t *testing.T) {
	t.Run("parentAndChildInSameYAMLFile", func(t *testing.T) {
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
  - name: task
    command: echo "Sub-DAG executed"
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())
		coord.Config.Queues.Enabled = true

		coordinatorClient := coord.GetCoordinatorClient(t)

		setupSharedNothingWorker(t, coord, "test-worker", map[string]string{
			"foo": "bar",
		})

		dagWrapper := coord.DAG(t, yamlContent)

		schedulerCtx, schedulerCancel := context.WithTimeout(coord.Context, 30*time.Second)
		defer schedulerCancel()

		schedulerInst := setupScheduler(t, coord, coordinatorClient)
		go func() { _ = schedulerInst.Start(schedulerCtx) }()

		err := executeStartCommand(t, coord, dagWrapper.DAG)
		require.NoError(t, err, "Start command should succeed")

		status := waitForSucceeded(t, coord, dagWrapper.DAG, 20*time.Second)
		schedulerCancel()

		require.Equal(t, core.Succeeded, status.Status, "DAG should succeed")
	})
}
