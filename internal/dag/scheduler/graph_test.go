package scheduler

import (
	"testing"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/stretchr/testify/require"
)

func TestCycleDetection(t *testing.T) {
	step1 := dag.Step{}
	step1.Name = "1"
	step1.Depends = []string{"2"}

	step2 := dag.Step{}
	step2.Name = "2"
	step2.Depends = []string{"1"}

	_, err := NewExecutionGraph(logger.Default, step1, step2)

	if err == nil {
		t.Fatal("cycle detection should be detected.")
	}
}

func TestRetryExecution(t *testing.T) {
	nodes := []*Node{
		{
			data: NodeData{
				Step: dag.Step{Name: "1", Command: "true"},
				NodeState: NodeState{
					Status: NodeStatusSuccess,
				},
			},
		},
		{
			data: NodeData{
				Step: dag.Step{Name: "2", Command: "true", Depends: []string{"1"}},
				NodeState: NodeState{
					Status: NodeStatusError,
				},
			},
		},
		{
			data: NodeData{
				Step: dag.Step{Name: "3", Command: "true", Depends: []string{"2"}},
				NodeState: NodeState{
					Status: NodeStatusCancel,
				},
			},
		},
		{
			data: NodeData{
				Step: dag.Step{Name: "4", Command: "true", Depends: []string{}},
				NodeState: NodeState{
					Status: NodeStatusSkipped,
				},
			},
		},
		{
			data: NodeData{
				Step: dag.Step{Name: "5", Command: "true", Depends: []string{"4"}},
				NodeState: NodeState{
					Status: NodeStatusError,
				},
			},
		},
		{
			data: NodeData{
				Step: dag.Step{Name: "6", Command: "true", Depends: []string{"5"}},
				NodeState: NodeState{
					Status: NodeStatusSuccess,
				},
			},
		},
		{
			data: NodeData{
				Step: dag.Step{Name: "7", Command: "true", Depends: []string{"6"}},
				NodeState: NodeState{
					Status: NodeStatusSkipped,
				},
			},
		},
		{
			data: NodeData{
				Step: dag.Step{Name: "8", Command: "true", Depends: []string{}},
				NodeState: NodeState{
					Status: NodeStatusSkipped,
				},
			},
		},
	}
	_, err := NewExecutionGraphForRetry(logger.Default, nodes...)
	require.NoError(t, err)
	require.Equal(t, NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, NodeStatusNone, nodes[1].State().Status)
	require.Equal(t, NodeStatusNone, nodes[2].State().Status)
	require.Equal(t, NodeStatusSkipped, nodes[3].State().Status)
	require.Equal(t, NodeStatusNone, nodes[4].State().Status)
	require.Equal(t, NodeStatusNone, nodes[5].State().Status)
	require.Equal(t, NodeStatusNone, nodes[6].State().Status)
	require.Equal(t, NodeStatusSkipped, nodes[7].State().Status)
}
