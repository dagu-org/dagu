package models

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

func makeStep(cmd string) *dag.Step {
	step := &dag.Step{
		Name: "test step",
	}
	step.Command, step.Args = utils.SplitCommand(cmd, false)
	return step
}

func TestFromNodes(t *testing.T) {
	g := testRunSteps(
		t,
		makeStep("true"),
		makeStep("false"),
	)

	ret := FromNodes(g.Nodes())

	require.Equal(t, 2, len(ret))
	require.NotEqual(t, "", ret[1].Error)
}

func TestToNode(t *testing.T) {
	g := testRunSteps(
		t,
		makeStep("true"),
		makeStep("true"),
	)
	orig := g.Nodes()
	for _, n := range orig {
		require.Equal(t, scheduler.NodeStatus_Success, n.Status)
	}
	nodes := FromNodes(orig)
	for i := range nodes {
		n := nodes[i].ToNode()
		require.Equal(t, n.Step, orig[i].Step)
		require.Equal(t, n.NodeState, orig[i].NodeState)
	}
}

func testRunSteps(t *testing.T, steps ...*dag.Step) *scheduler.ExecutionGraph {
	g, err := scheduler.NewExecutionGraph(steps...)
	require.NoError(t, err)
	for _, n := range g.Nodes() {
		if err := n.Execute(); err != nil {
			n.Status = scheduler.NodeStatus_Error
		} else {
			n.Status = scheduler.NodeStatus_Success
		}
	}
	return g
}
