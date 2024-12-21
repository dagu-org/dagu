// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/require"
)

func TestCycleDetection(t *testing.T) {
	step1 := digraph.Step{}
	step1.Name = "1"
	step1.Depends = []string{"2"}

	step2 := digraph.Step{}
	step2.Name = "2"
	step2.Depends = []string{"1"}

	_, err := NewExecutionGraph(step1, step2)

	if err == nil {
		t.Fatal("cycle detection should be detected.")
	}
}

func TestRetryExecution(t *testing.T) {
	nodes := []*Node{
		{
			data: NodeData{
				Step: digraph.Step{Name: "1", Command: "true"},
				State: NodeState{
					Status: NodeStatusSuccess,
				},
			},
		},
		{
			data: NodeData{
				Step: digraph.Step{Name: "2", Command: "true", Depends: []string{"1"}},
				State: NodeState{
					Status: NodeStatusError,
				},
			},
		},
		{
			data: NodeData{
				Step: digraph.Step{Name: "3", Command: "true", Depends: []string{"2"}},
				State: NodeState{
					Status: NodeStatusCancel,
				},
			},
		},
		{
			data: NodeData{
				Step: digraph.Step{Name: "4", Command: "true", Depends: []string{}},
				State: NodeState{
					Status: NodeStatusSkipped,
				},
			},
		},
		{
			data: NodeData{
				Step: digraph.Step{Name: "5", Command: "true", Depends: []string{"4"}},
				State: NodeState{
					Status: NodeStatusError,
				},
			},
		},
		{
			data: NodeData{
				Step: digraph.Step{Name: "6", Command: "true", Depends: []string{"5"}},
				State: NodeState{
					Status: NodeStatusSuccess,
				},
			},
		},
		{
			data: NodeData{
				Step: digraph.Step{Name: "7", Command: "true", Depends: []string{"6"}},
				State: NodeState{
					Status: NodeStatusSkipped,
				},
			},
		},
		{
			data: NodeData{
				Step: digraph.Step{Name: "8", Command: "true", Depends: []string{}},
				State: NodeState{
					Status: NodeStatusSkipped,
				},
			},
		},
	}
	ctx := context.Background()
	_, err := CreateRetryExecutionGraph(ctx, nodes...)
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
