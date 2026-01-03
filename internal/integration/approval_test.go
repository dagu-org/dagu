package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestWaitStepApproval(t *testing.T) {
	t.Run("WaitStepEntersWaitStatus", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: wait-step
    executor:
      type: wait
      config:
        prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: echo "approved"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Wait)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 2)
		require.Equal(t, "wait-step", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[0].Status)
		require.Equal(t, "after-wait", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[1].Status)
	})

	t.Run("WaitStepBlocksDependentNodes", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: before-wait
    command: echo "before"
  - name: wait-step
    depends: [before-wait]
    executor:
      type: wait
  - name: after-wait-1
    depends: [wait-step]
    command: echo "after1"
  - name: after-wait-2
    depends: [wait-step]
    command: echo "after2"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Wait)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 4)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status, "before-wait should succeed")
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[1].Status, "wait-step should be waiting")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[2].Status, "after-wait-1 should not start")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[3].Status, "after-wait-2 should not start")
	})

	t.Run("ParallelBranchWithWaitStep", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: branch-a-1
    command: echo "a1"
  - name: branch-a-2
    depends: [branch-a-1]
    command: echo "a2"
  - name: wait-branch
    executor:
      type: wait
  - name: after-wait
    depends: [wait-branch]
    command: echo "after wait"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		testDAG.AssertLatestStatus(t, core.Wait)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 4)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status, "branch-a-1 should succeed")
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[1].Status, "branch-a-2 should succeed")
		require.Equal(t, core.NodeWaiting, dagRunStatus.Nodes[2].Status, "wait-branch should be waiting")
		require.Equal(t, core.NodeNotStarted, dagRunStatus.Nodes[3].Status, "after-wait should not start")
	})
}
