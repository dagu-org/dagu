package scheduler_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

func TestCycleDetection(t *testing.T) {
	step1 := digraph.Step{}
	step1.Name = "1"
	step1.Depends = []string{"2"}

	step2 := digraph.Step{}
	step2.Name = "2"
	step2.Depends = []string{"1"}

	_, err := scheduler.NewExecutionGraph(step1, step2)

	if err == nil {
		t.Fatal("cycle detection should be detected.")
	}
}

func TestRetryExecution(t *testing.T) {
	nodes := []*scheduler.Node{
		scheduler.NodeWithData(
			scheduler.NodeData{
				Step: digraph.Step{Name: "1", Command: "true"},
				State: scheduler.NodeState{
					Status: scheduler.NodeStatusSuccess,
				},
			}),
		scheduler.NodeWithData(
			scheduler.NodeData{
				Step: digraph.Step{Name: "2", Command: "true", Depends: []string{"1"}},
				State: scheduler.NodeState{
					Status: scheduler.NodeStatusError,
				},
			},
		),
		scheduler.NodeWithData(
			scheduler.NodeData{
				Step: digraph.Step{Name: "3", Command: "true", Depends: []string{"2"}},
				State: scheduler.NodeState{
					Status: scheduler.NodeStatusCancel,
				},
			},
		),
		scheduler.NodeWithData(
			scheduler.NodeData{
				Step: digraph.Step{Name: "4", Command: "true", Depends: []string{}},
				State: scheduler.NodeState{
					Status: scheduler.NodeStatusSkipped,
				},
			},
		),
		scheduler.NodeWithData(
			scheduler.NodeData{
				Step: digraph.Step{Name: "5", Command: "true", Depends: []string{"4"}},
				State: scheduler.NodeState{
					Status: scheduler.NodeStatusError,
				},
			},
		),
		scheduler.NodeWithData(
			scheduler.NodeData{
				Step: digraph.Step{Name: "6", Command: "true", Depends: []string{"5"}},
				State: scheduler.NodeState{
					Status: scheduler.NodeStatusSuccess,
				},
			},
		),
		scheduler.NodeWithData(
			scheduler.NodeData{
				Step: digraph.Step{Name: "7", Command: "true", Depends: []string{"6"}},
				State: scheduler.NodeState{
					Status: scheduler.NodeStatusSkipped,
				},
			},
		),
		scheduler.NodeWithData(
			scheduler.NodeData{
				Step: digraph.Step{Name: "8", Command: "true", Depends: []string{}},
				State: scheduler.NodeState{
					Status: scheduler.NodeStatusSkipped,
				},
			},
		),
	}
	ctx := context.Background()
	_, err := scheduler.CreateRetryExecutionGraph(ctx, nodes...)
	require.NoError(t, err)
	require.Equal(t, scheduler.NodeStatusSuccess, nodes[0].State().Status)
	require.Equal(t, scheduler.NodeStatusNone, nodes[1].State().Status)
	require.Equal(t, scheduler.NodeStatusNone, nodes[2].State().Status)
	require.Equal(t, scheduler.NodeStatusSkipped, nodes[3].State().Status)
	require.Equal(t, scheduler.NodeStatusNone, nodes[4].State().Status)
	require.Equal(t, scheduler.NodeStatusNone, nodes[5].State().Status)
	require.Equal(t, scheduler.NodeStatusNone, nodes[6].State().Status)
	require.Equal(t, scheduler.NodeStatusSkipped, nodes[7].State().Status)
}
