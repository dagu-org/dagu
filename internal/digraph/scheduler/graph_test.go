// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package scheduler

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/stretchr/testify/require"
)

func TestCycleDetection(t *testing.T) {
	step1 := digraph.Step{}
	step1.Name = "1"
	step1.Depends = []string{"2"}

	step2 := digraph.Step{}
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
