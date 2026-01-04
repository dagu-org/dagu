package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

// helper to count total dependencies in a plan
func totalDeps(p *runtime.Plan) int {
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

func TestPlan_Cyclic(t *testing.T) {
	step1 := core.Step{Name: "1", Depends: []string{"2"}}
	step2 := core.Step{Name: "2", Depends: []string{"1"}}
	_, err := runtime.NewPlan(step1, step2)
	require.Error(t, err)
	require.ErrorIs(t, err, runtime.ErrCyclicPlan)
}

func TestPlan_NodeByName(t *testing.T) {
	steps := []core.Step{{Name: "a"}, {Name: "b", Depends: []string{"a"}}}
	p, err := runtime.NewPlan(steps...)
	require.NoError(t, err)
	require.NotNil(t, p.GetNodeByName("a"))
	require.NotNil(t, p.GetNodeByName("b"))
	require.Nil(t, p.GetNodeByName("c"))
}

func TestPlan_DependencyStructures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		steps             []core.Step
		wantTotalDeps     int
		wantOutgoingCount int // nodes with dependents
		wantIncomingCount int // nodes with dependencies
	}{
		{
			name: "basic",
			steps: []core.Step{
				{Name: "step1", Commands: []core.CommandEntry{{Command: "echo", Args: []string{"1"}}}},
				{Name: "step2", Commands: []core.CommandEntry{{Command: "echo", Args: []string{"2"}}}, Depends: []string{"step1"}},
				{Name: "step3", Commands: []core.CommandEntry{{Command: "echo", Args: []string{"3"}}}, Depends: []string{"step2", "step1"}},
			},
			wantTotalDeps:     3, // 1->2,1->3,2->3
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
			wantTotalDeps:     2,
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
			wantTotalDeps:     3, // dl->process, extract->process, process->cleanup
			wantOutgoingCount: 3, // download, extract, process
			wantIncomingCount: 2, // process, cleanup
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := runtime.NewPlan(tt.steps...)
			require.NoError(t, err)
			require.Equal(t, tt.wantTotalDeps, totalDeps(p))

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

func TestRetryPlan(t *testing.T) {
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
	p, err := runtime.CreateRetryPlan(ctx, dag, nodes...)
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

func TestStepRetryPlan(t *testing.T) {
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

func TestPlan_Timing(t *testing.T) {
	steps := []core.Step{{Name: "a"}}
	p, err := runtime.NewPlan(steps...)
	require.NoError(t, err)
	require.True(t, p.IsStarted())
	require.False(t, p.IsFinished())
	require.True(t, p.Duration() >= 0)
	p.Finish()
	require.True(t, p.IsFinished())
	finish := p.FinishAt()
	require.WithinDuration(t, time.Now(), finish, time.Second)
}

func TestPlan_HasActivelyRunningNodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		nodes    []*runtime.Node
		expected bool
	}{
		{
			name: "no nodes running",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeSucceeded),
				makeNode("b", core.NodeSucceeded, "a"),
			},
			expected: false,
		},
		{
			name: "one node running",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeSucceeded),
				makeNode("b", core.NodeRunning, "a"),
			},
			expected: true,
		},
		{
			name: "only not started nodes",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeNotStarted),
				makeNode("b", core.NodeNotStarted, "a"),
			},
			expected: false,
		},
		{
			name: "waiting node with not started dependents",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeWaiting),
				makeNode("b", core.NodeNotStarted, "a"),
			},
			expected: false,
		},
		{
			name: "mix of completed and waiting",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeSucceeded),
				makeNode("b", core.NodeWaiting, "a"),
				makeNode("c", core.NodeNotStarted, "b"),
			},
			expected: false,
		},
		{
			name: "running node alongside waiting",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeRunning),
				makeNode("b", core.NodeWaiting),
				makeNode("c", core.NodeNotStarted, "b"),
			},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := &core.DAG{}
			for _, n := range tt.nodes {
				dag.Steps = append(dag.Steps, n.Step())
			}
			p, err := runtime.CreateRetryPlan(context.Background(), dag, tt.nodes...)
			require.NoError(t, err)
			require.Equal(t, tt.expected, p.HasActivelyRunningNodes())
		})
	}
}

func TestPlan_GetNodeStatusSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		nodes             []*runtime.Node
		wantHasRunning    bool
		wantHasWaiting    bool
		wantHasNotStarted bool
		wantWaitingCount  int
	}{
		{
			name: "all succeeded",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeSucceeded),
				makeNode("b", core.NodeSucceeded, "a"),
			},
			wantHasRunning:    false,
			wantHasWaiting:    false,
			wantHasNotStarted: false,
			wantWaitingCount:  0,
		},
		{
			name: "one running",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeSucceeded),
				makeNode("b", core.NodeRunning, "a"),
			},
			wantHasRunning:    true,
			wantHasWaiting:    false,
			wantHasNotStarted: false,
			wantWaitingCount:  0,
		},
		{
			name: "one waiting with blocked dependents",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeWaiting),
				makeNode("b", core.NodeNotStarted, "a"),
			},
			wantHasRunning:    false,
			wantHasWaiting:    true,
			wantHasNotStarted: true,
			wantWaitingCount:  1,
		},
		{
			name: "multiple waiting nodes",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeWaiting),
				makeNode("b", core.NodeWaiting),
				makeNode("c", core.NodeNotStarted, "a", "b"),
			},
			wantHasRunning:    false,
			wantHasWaiting:    true,
			wantHasNotStarted: true,
			wantWaitingCount:  2,
		},
		{
			name: "mix of all states",
			nodes: []*runtime.Node{
				makeNode("a", core.NodeRunning),
				makeNode("b", core.NodeWaiting),
				makeNode("c", core.NodeNotStarted, "b"),
				makeNode("d", core.NodeSucceeded),
			},
			wantHasRunning:    true,
			wantHasWaiting:    true,
			wantHasNotStarted: true,
			wantWaitingCount:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := &core.DAG{}
			for _, n := range tt.nodes {
				dag.Steps = append(dag.Steps, n.Step())
			}
			p, err := runtime.CreateRetryPlan(context.Background(), dag, tt.nodes...)
			require.NoError(t, err)

			summary := p.GetNodeStatusSummary()
			require.Equal(t, tt.wantHasRunning, summary.HasRunning, "HasRunning mismatch")
			require.Equal(t, tt.wantHasWaiting, summary.HasWaiting, "HasWaiting mismatch")
			require.Equal(t, tt.wantHasNotStarted, summary.HasNotStarted, "HasNotStarted mismatch")
			require.Len(t, summary.WaitingNodes, tt.wantWaitingCount, "WaitingNodes count mismatch")
		})
	}
}
