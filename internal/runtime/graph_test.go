package runtime_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

// helper to count edges in From map
func totalEdges(from map[int][]int) int {
	c := 0
	for _, targets := range from {
		c += len(targets)
	}
	return c
}

// helper to quickly make a Node
func makeNode(name string, status core.NodeStatus, depends ...string) *runtime.Node {
	return runtime.NodeWithData(runtime.NodeData{
		Step:  core.Step{Name: name, Depends: depends},
		State: runtime.NodeState{Status: status},
	})
}

func TestExecutionGraph_CycleDetection(t *testing.T) {
	step1 := core.Step{Name: "1", Depends: []string{"2"}}
	step2 := core.Step{Name: "2", Depends: []string{"1"}}
	_, err := runtime.NewExecutionGraph(step1, step2)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "cycle detected"), "expected cycle detected error")
}

func TestExecutionGraph_NodeByName(t *testing.T) {
	steps := []core.Step{{Name: "a"}, {Name: "b", Depends: []string{"a"}}}
	g, err := runtime.NewExecutionGraph(steps...)
	require.NoError(t, err)
	require.NotNil(t, g.NodeByName("a"))
	require.NotNil(t, g.NodeByName("b"))
	require.Nil(t, g.NodeByName("c"))
}

func TestExecutionGraph_DependencyStructures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		steps             []core.Step
		wantTotalEdges    int
		wantOutgoingCount int // len(From)
		wantIncomingCount int // len(To)
	}{
		{
			name: "basic",
			steps: []core.Step{
				{Name: "step1", Command: "echo 1"},
				{Name: "step2", Command: "echo 2", Depends: []string{"step1"}},
				{Name: "step3", Command: "echo 3", Depends: []string{"step2", "step1"}},
			},
			wantTotalEdges:    3, // 1->2,1->3,2->3
			wantOutgoingCount: 2, // step1, step2
			wantIncomingCount: 2, // step2, step3
		},
		{
			name: "single chain",
			steps: []core.Step{
				{Name: "download"},
				{Name: "process", Depends: []string{"download"}},
				{Name: "cleanup", Depends: []string{"process"}},
			},
			wantTotalEdges:    2,
			wantOutgoingCount: 2,
			wantIncomingCount: 2,
		},
		{
			name: "fan in/out",
			steps: []core.Step{
				{Name: "download"},
				{Name: "extract"},
				{Name: "process", Depends: []string{"download", "extract"}},
				{Name: "cleanup", Depends: []string{"process"}},
			},
			wantTotalEdges:    3, // dl->process, extract->process, process->cleanup
			wantOutgoingCount: 3, // download, extract, process
			wantIncomingCount: 2, // process, cleanup
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			g, err := runtime.NewExecutionGraph(tt.steps...)
			require.NoError(t, err)
			require.Equal(t, tt.wantTotalEdges, totalEdges(g.From))
			require.Len(t, g.From, tt.wantOutgoingCount)
			require.Len(t, g.To, tt.wantIncomingCount)
		})
	}
}

func TestRetryExecutionGraph(t *testing.T) {
	ctx := context.Background()
	dag := &core.DAG{Steps: []core.Step{
		{Name: "1"}, {Name: "2", Depends: []string{"1"}}, {Name: "3", Depends: []string{"2"}},
		{Name: "4"}, {Name: "5", Depends: []string{"4"}}, {Name: "6", Depends: []string{"5"}}, {Name: "7", Depends: []string{"6"}},
		{Name: "8"},
	}}
	nodes := []*runtime.Node{
		makeNode("1", core.NodeSucceeded),
		makeNode("2", core.NodeFailed, "1"),
		makeNode("3", core.NodeAborted, "2"),
		makeNode("4", core.NodeSkipped),
		makeNode("5", core.NodeFailed, "4"),
		makeNode("6", core.NodeSucceeded, "5"),
		makeNode("7", core.NodeSkipped, "6"),
		makeNode("8", core.NodeSkipped),
	}
	g, err := runtime.CreateRetryExecutionGraph(ctx, dag, nodes...)
	require.NoError(t, err)
	require.NotNil(t, g)
	// expectations based on upstream failures and aborted states triggering retry propagation
	require.Equal(t, core.NodeSucceeded, nodes[0].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[1].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[2].State().Status)
	require.Equal(t, core.NodeSkipped, nodes[3].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[4].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[5].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[6].State().Status)
	require.Equal(t, core.NodeSkipped, nodes[7].State().Status)
}

func TestStepRetryExecutionGraph(t *testing.T) {
	dag := &core.DAG{Steps: []core.Step{
		{Name: "1"}, {Name: "2", Depends: []string{"1"}}, {Name: "3", Depends: []string{"2"}},
		{Name: "4"}, {Name: "5", Depends: []string{"4"}}, {Name: "6", Depends: []string{"5"}}, {Name: "7", Depends: []string{"6"}},
	}}
	baseNodes := []*runtime.Node{
		makeNode("1", core.NodeSucceeded),
		makeNode("2", core.NodeFailed, "1"),
		makeNode("3", core.NodeAborted, "2"),
		makeNode("4", core.NodeSkipped),
		makeNode("5", core.NodeFailed, "4"),
		makeNode("6", core.NodeSucceeded, "5"),
		makeNode("7", core.NodeSkipped, "6"),
	}
	tests := []struct {
		name       string
		step       string
		wantStatus map[string]core.NodeStatus
	}{
		{name: "retry failed step", step: "2", wantStatus: map[string]core.NodeStatus{"1": core.NodeSucceeded, "2": core.NodeNotStarted, "3": core.NodeAborted, "4": core.NodeSkipped, "5": core.NodeFailed, "6": core.NodeSucceeded, "7": core.NodeSkipped}},
		{name: "retry succeeded first", step: "1", wantStatus: map[string]core.NodeStatus{"1": core.NodeNotStarted, "2": core.NodeFailed, "3": core.NodeAborted, "4": core.NodeSkipped, "5": core.NodeFailed, "6": core.NodeSucceeded, "7": core.NodeSkipped}},
		{name: "retry succeeded middle", step: "6", wantStatus: map[string]core.NodeStatus{"1": core.NodeSucceeded, "2": core.NodeFailed, "3": core.NodeAborted, "4": core.NodeSkipped, "5": core.NodeFailed, "6": core.NodeNotStarted, "7": core.NodeSkipped}},
		{name: "retry succeeded last", step: "7", wantStatus: map[string]core.NodeStatus{"1": core.NodeSucceeded, "2": core.NodeFailed, "3": core.NodeAborted, "4": core.NodeSkipped, "5": core.NodeFailed, "6": core.NodeSucceeded, "7": core.NodeNotStarted}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// deep copy nodes (statuses) for isolation
			nodes := make([]*runtime.Node, 0, len(baseNodes))
			for _, n := range baseNodes {
				nodes = append(nodes, makeNode(n.Name(), n.State().Status, n.Step().Depends...))
			}
			g, err := runtime.CreateStepRetryGraph(dag, nodes, tt.step)
			require.NoError(t, err)
			require.NotNil(t, g)
			for _, n := range nodes {
				require.Equal(t, tt.wantStatus[n.Name()], n.State().Status, "status mismatch for %s", n.Name())
			}
		})
	}
}

func TestExecutionGraph_Timing(t *testing.T) {
	steps := []core.Step{{Name: "a"}}
	g, err := runtime.NewExecutionGraph(steps...)
	require.NoError(t, err)
	require.True(t, g.IsStarted())
	require.False(t, g.IsFinished())
	require.True(t, g.Duration() >= 0)
	g.Finish()
	require.True(t, g.IsFinished())
	finish := g.FinishAt()
	require.WithinDuration(t, time.Now(), finish, time.Second)
}
