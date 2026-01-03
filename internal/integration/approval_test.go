package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// findNodeByName returns the node with the given step name, or nil if not found.
func findNodeByName(nodes []*execution.Node, name string) *execution.Node {
	for _, node := range nodes {
		if node.Step.Name == name {
			return node
		}
	}
	return nil
}

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

		waitStepNode := findNodeByName(dagRunStatus.Nodes, "wait-step")
		require.NotNil(t, waitStepNode, "wait-step node should exist")
		require.Equal(t, core.NodeWaiting, waitStepNode.Status)

		afterWaitNode := findNodeByName(dagRunStatus.Nodes, "after-wait")
		require.NotNil(t, afterWaitNode, "after-wait node should exist")
		require.Equal(t, core.NodeNotStarted, afterWaitNode.Status)
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

		beforeWaitNode := findNodeByName(dagRunStatus.Nodes, "before-wait")
		require.NotNil(t, beforeWaitNode, "before-wait node should exist")
		require.Equal(t, core.NodeSucceeded, beforeWaitNode.Status, "before-wait should succeed")

		waitStepNode := findNodeByName(dagRunStatus.Nodes, "wait-step")
		require.NotNil(t, waitStepNode, "wait-step node should exist")
		require.Equal(t, core.NodeWaiting, waitStepNode.Status, "wait-step should be waiting")

		afterWait1Node := findNodeByName(dagRunStatus.Nodes, "after-wait-1")
		require.NotNil(t, afterWait1Node, "after-wait-1 node should exist")
		require.Equal(t, core.NodeNotStarted, afterWait1Node.Status, "after-wait-1 should not start")

		afterWait2Node := findNodeByName(dagRunStatus.Nodes, "after-wait-2")
		require.NotNil(t, afterWait2Node, "after-wait-2 node should exist")
		require.Equal(t, core.NodeNotStarted, afterWait2Node.Status, "after-wait-2 should not start")
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

		branchA1Node := findNodeByName(dagRunStatus.Nodes, "branch-a-1")
		require.NotNil(t, branchA1Node, "branch-a-1 node should exist")
		require.Equal(t, core.NodeSucceeded, branchA1Node.Status, "branch-a-1 should succeed")

		branchA2Node := findNodeByName(dagRunStatus.Nodes, "branch-a-2")
		require.NotNil(t, branchA2Node, "branch-a-2 node should exist")
		require.Equal(t, core.NodeSucceeded, branchA2Node.Status, "branch-a-2 should succeed")

		waitBranchNode := findNodeByName(dagRunStatus.Nodes, "wait-branch")
		require.NotNil(t, waitBranchNode, "wait-branch node should exist")
		require.Equal(t, core.NodeWaiting, waitBranchNode.Status, "wait-branch should be waiting")

		afterWaitNode := findNodeByName(dagRunStatus.Nodes, "after-wait")
		require.NotNil(t, afterWaitNode, "after-wait node should exist")
		require.Equal(t, core.NodeNotStarted, afterWaitNode.Status, "after-wait should not start")
	})

	// Note: SubDAGWithWaitStep test is not included because propagating wait status
	// from sub-DAG to parent node is not yet implemented in the sub-DAG executor.
	// When a sub-DAG enters Wait status, it currently fails instead of setting
	// NodeWaiting on the parent call step.
}
