package distr_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/require"
)

func TestSubDAG_LocalCallsDistributed(t *testing.T) {
	t.Run("localParentCallsDistributedChild", func(t *testing.T) {
		f := newTestFixture(t, `
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
`, withLabels(map[string]string{"type": "test-worker"}))

		agent := f.dagWrapper.Agent()
		agent.RunSuccess(t)
		f.dagWrapper.AssertLatestStatus(t, core.Succeeded)
	})
}

func TestSubDAG_FailurePropagation(t *testing.T) {
	t.Run("childFailurePropagatesToParent", func(t *testing.T) {
		f := newTestFixture(t, `
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
`, withLabels(map[string]string{"type": "test-worker"}))

		agent := f.dagWrapper.Agent()

		err := agent.Run(agent.Context)
		require.Error(t, err)

		f.dagWrapper.AssertLatestStatus(t, core.Failed)

		st, statusErr := f.latestStatus()
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
		f := newTestFixture(t, `
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
`, withWorkerCount(0))

		agent := f.dagWrapper.Agent()

		ctx, cancel := context.WithTimeout(f.coord.Context, 5*time.Second)
		defer cancel()
		err := agent.Run(ctx)
		require.Error(t, err)

		st := agent.Status(f.coord.Context)
		require.NotEqual(t, core.Succeeded, st.Status)
	})
}

func TestSubDAG_DifferentWorkers(t *testing.T) {
	t.Run("parentAndChildOnDifferentWorkers", func(t *testing.T) {
		childYAML := `
name: child-remote
workerSelector:
  type: child
steps:
  - name: child-step
    command: echo "child executed"
`
		f := newTestFixture(t, `
name: parent-remote
workerSelector:
  type: parent
steps:
  - call: child-remote
`, withLabels(map[string]string{"type": "parent"}))
		defer f.cleanup()

		f.coord.CreateDAGFile(t, f.coord.Config.Paths.DAGsDir, "child-remote", []byte(childYAML))

		childWorker := f.setupSharedNothingWorker("child-worker", map[string]string{"type": "child"})
		_ = childWorker

		require.NoError(t, f.enqueue())
		f.waitForQueued()
		f.startScheduler(30 * time.Second)

		status := f.waitForStatus(core.Succeeded, 25*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
	})
}

func TestSubDAG_InSameFile(t *testing.T) {
	t.Run("parentAndChildInSameYAMLFile", func(t *testing.T) {
		f := newTestFixture(t, `
steps:
  - call: dotest
params:
  - URL: default_value
---
name: dotest
workerSelector:
  foo: bar
steps:
  - name: task
    command: echo "Sub-DAG executed"
`, withLabels(map[string]string{"foo": "bar"}))
		defer f.cleanup()

		f.startScheduler(30 * time.Second)

		require.NoError(t, f.start())

		status := f.waitForStatus(core.Succeeded, 20*time.Second)

		require.Equal(t, core.Succeeded, status.Status)
	})
}
