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

// helper to count edges in the plan
func totalEdges(p *runtime.ExecutionPlan) int {
	c := 0
	for _, node := range p.Nodes() {
		c += len(p.Dependents(node.ID()))
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

func TestExecutionPlan_CycleDetection(t *testing.T) {
	step1 := core.Step{Name: "1", Depends: []string{"2"}}
	step2 := core.Step{Name: "2", Depends: []string{"1"}}
	_, err := runtime.NewExecutionPlan(step1, step2)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "cycle detected"), "expected cycle detected error")
}

func TestExecutionPlan_NodeByName(t *testing.T) {
	steps := []core.Step{{Name: "a"}, {Name: "b", Depends: []string{"a"}}}
	p, err := runtime.NewExecutionPlan(steps...)
	require.NoError(t, err)
	require.NotNil(t, p.GetNodeByName("a"))
	require.NotNil(t, p.GetNodeByName("b"))
	require.Nil(t, p.GetNodeByName("c"))
}

func TestExecutionPlan_DependencyStructures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		steps             []core.Step
		wantTotalEdges    int
		wantOutgoingCount int // nodes with dependents
		wantIncomingCount int // nodes with dependencies
	}{
		{
			name: "basic",
			steps: []core.Step{
				{Name: "step1", Command: "echo 1"},
				{Name: "step2", Command: "echo 2", Depends: []string{"step1"}},
				{Name: "step3", Command: "echo 3", Depends: []string{"step2", "step1"}},
			},
			wantTotalEdges:    3, // 1->2,1->3,2->3
			wantOutgoingCount: 2, // step1 (has 2,3), step2 (has 3)
			wantIncomingCount: 2, // step2 (has 1), step3 (has 1,2)
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
		t.Run(tt.name, func(t *testing.T) {
			p, err := runtime.NewExecutionPlan(tt.steps...)
			require.NoError(t, err)
			require.Equal(t, tt.wantTotalEdges, totalEdges(p))

			outgoing := 0
			incoming := 0
			for _, n := range p.Nodes() {
				if len(p.Dependents(n.ID())) > 0 {
					outgoing++
				}
				if len(p.Dependencies(n.ID())) > 0 {
					incoming++
				}
			}

			require.Equal(t, tt.wantOutgoingCount, outgoing)
			require.Equal(t, tt.wantIncomingCount, incoming)
		})
	}
}

func TestRetryExecutionPlan(t *testing.T) {
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
	p, err := runtime.CreateRetryExecutionPlan(ctx, dag, nodes...)
	require.NoError(t, err)
	require.NotNil(t, p)
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

func TestStepRetryExecutionPlan(t *testing.T) {
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
		{
			name: "retry failed step",
			step: "2",
			wantStatus: map[string]core.NodeStatus{
				"1": core.NodeSucceeded,
				"2": core.NodeNotStarted,
				"3": core.NodeAborted,
				"4": core.NodeSkipped,
				"5": core.NodeFailed,
				"6": core.NodeSucceeded,
				"7": core.NodeSkipped,
			},
		},
		{
			name: "retry succeeded first",
			step: "1",
			wantStatus: map[string]core.NodeStatus{
				"1": core.NodeNotStarted,
				"2": core.NodeFailed,
				"3": core.NodeAborted,
				"4": core.NodeSkipped,
				"5": core.NodeFailed,
				"6": core.NodeSucceeded,
				"7": core.NodeSkipped,
			},
		},
		{
			name: "retry succeeded middle",
			step: "6",
			wantStatus: map[string]core.NodeStatus{
				"1": core.NodeSucceeded,
				"2": core.NodeFailed,
				"3": core.NodeAborted,
				"4": core.NodeSkipped,
				"5": core.NodeFailed,
				"6": core.NodeNotStarted,
				"7": core.NodeSkipped,
			},
		},
		{
			name: "retry succeeded last",
			step: "7",
			wantStatus: map[string]core.NodeStatus{
				"1": core.NodeSucceeded,
				"2": core.NodeFailed,
				"3": core.NodeAborted,
				"4": core.NodeSkipped,
				"5": core.NodeFailed,
				"6": core.NodeSucceeded,
				"7": core.NodeNotStarted,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// deep copy nodes (statuses) for isolation
			nodes := make([]*runtime.Node, 0, len(baseNodes))
			for _, n := range baseNodes {
				nodes = append(nodes, makeNode(n.Name(), n.State().Status, n.Step().Depends...))
			}
			p, err := runtime.CreateStepRetryPlan(dag, nodes, tt.step)
			require.NoError(t, err)
			require.NotNil(t, p)
			for _, n := range nodes {
				require.Equal(t, tt.wantStatus[n.Name()], n.State().Status, "status mismatch for %s", n.Name())
			}
		})
	}
}

func TestExecutionPlan_Timing(t *testing.T) {
	steps := []core.Step{{Name: "a"}}
	p, err := runtime.NewExecutionPlan(steps...)
	require.NoError(t, err)
	require.True(t, p.IsStarted())
	require.False(t, p.IsFinished())
	require.True(t, p.Duration() >= 0)
	p.Finish()
	require.True(t, p.IsFinished())
	finish := p.FinishAt()
	require.WithinDuration(t, time.Now(), finish, time.Second)
}
