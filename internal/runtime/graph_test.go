package runtime_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

func TestCycleDetection(t *testing.T) {
	step1 := core.Step{}
	step1.Name = "1"
	step1.Depends = []string{"2"}

	step2 := core.Step{}
	step2.Name = "2"
	step2.Depends = []string{"1"}

	_, err := runtime.NewExecutionGraph(step1, step2)

	if err == nil {
		t.Fatal("cycle detection should be detected.")
	}
}

func TestRetryExecution(t *testing.T) {
	dag := &core.DAG{
		Steps: []core.Step{
			{Name: "1", Command: "true"},
			{Name: "2", Command: "true", Depends: []string{"1"}},
			{Name: "3", Command: "true", Depends: []string{"2"}},
			{Name: "4", Command: "true", Depends: []string{}},
			{Name: "5", Command: "true", Depends: []string{"4"}},
			{Name: "6", Command: "true", Depends: []string{"5"}},
			{Name: "7", Command: "true", Depends: []string{"6"}},
		},
	}

	nodes := []*runtime.Node{
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "1", Command: "true"},
				State: runtime.NodeState{
					Status: core.NodeSucceeded,
				},
			}),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "2", Command: "true", Depends: []string{"1"}},
				State: runtime.NodeState{
					Status: core.NodeFailed,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "3", Command: "true", Depends: []string{"2"}},
				State: runtime.NodeState{
					Status: core.NodeCanceled,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "4", Command: "true", Depends: []string{}},
				State: runtime.NodeState{
					Status: core.NodeSkipped,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "5", Command: "true", Depends: []string{"4"}},
				State: runtime.NodeState{
					Status: core.NodeFailed,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "6", Command: "true", Depends: []string{"5"}},
				State: runtime.NodeState{
					Status: core.NodeSucceeded,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "7", Command: "true", Depends: []string{"6"}},
				State: runtime.NodeState{
					Status: core.NodeSkipped,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "8", Command: "true", Depends: []string{}},
				State: runtime.NodeState{
					Status: core.NodeSkipped,
				},
			},
		),
	}
	ctx := context.Background()
	_, err := runtime.CreateRetryExecutionGraph(ctx, dag, nodes...)
	require.NoError(t, err)
	require.Equal(t, core.NodeSucceeded, nodes[0].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[1].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[2].State().Status)
	require.Equal(t, core.NodeSkipped, nodes[3].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[4].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[5].State().Status)
	require.Equal(t, core.NodeNotStarted, nodes[6].State().Status)
	require.Equal(t, core.NodeSkipped, nodes[7].State().Status)
}

func TestStepRetryExecution(t *testing.T) {
	dag := &core.DAG{
		Steps: []core.Step{
			{Name: "1", Command: "true"},
			{Name: "2", Command: "true", Depends: []string{"1"}},
			{Name: "3", Command: "true", Depends: []string{"2"}},
			{Name: "4", Command: "true", Depends: []string{}},
			{Name: "5", Command: "true", Depends: []string{"4"}},
			{Name: "6", Command: "true", Depends: []string{"5"}},
			{Name: "7", Command: "true", Depends: []string{"6"}},
		},
	}

	nodes := []*runtime.Node{
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "1", Command: "true"},
				State: runtime.NodeState{
					Status: core.NodeSucceeded,
				},
			}),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "2", Command: "true", Depends: []string{"1"}},
				State: runtime.NodeState{
					Status: core.NodeFailed,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "3", Command: "true", Depends: []string{"2"}},
				State: runtime.NodeState{
					Status: core.NodeCanceled,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "4", Command: "true", Depends: []string{}},
				State: runtime.NodeState{
					Status: core.NodeSkipped,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "5", Command: "true", Depends: []string{"4"}},
				State: runtime.NodeState{
					Status: core.NodeFailed,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "6", Command: "true", Depends: []string{"5"}},
				State: runtime.NodeState{
					Status: core.NodeSucceeded,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "7", Command: "true", Depends: []string{"6"}},
				State: runtime.NodeState{
					Status: core.NodeSkipped,
				},
			},
		),
	}
	_, err := runtime.CreateStepRetryGraph(dag, nodes, "2")
	require.NoError(t, err)
	// Only step 2 should be reset to NodeStatusNone, downstream steps remain untouched
	require.Equal(t, core.NodeSucceeded, nodes[0].State().Status)  // 1 (unchanged)
	require.Equal(t, core.NodeNotStarted, nodes[1].State().Status) // 2 (reset)
	require.Equal(t, core.NodeCanceled, nodes[2].State().Status)   // 3 (unchanged)
	require.Equal(t, core.NodeSkipped, nodes[3].State().Status)    // 4 (unchanged)
	require.Equal(t, core.NodeFailed, nodes[4].State().Status)     // 5 (unchanged)
	require.Equal(t, core.NodeSucceeded, nodes[5].State().Status)  // 6 (unchanged)
	require.Equal(t, core.NodeSkipped, nodes[6].State().Status)    // 7 (unchanged)
}

func TestStepRetryExecutionForSuccessfulStep(t *testing.T) {
	// Test that we can retry a successful step
	dag := &core.DAG{
		Steps: []core.Step{
			{Name: "step1", Command: "echo 1"},
			{Name: "step2", Command: "echo 2", Depends: []string{"step1"}},
			{Name: "step3", Command: "echo 3", Depends: []string{"step2"}},
		},
	}

	// All nodes are successful
	nodes := []*runtime.Node{
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "step1", Command: "echo 1"},
				State: runtime.NodeState{
					Status: core.NodeSucceeded,
				},
			}),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "step2", Command: "echo 2", Depends: []string{"step1"}},
				State: runtime.NodeState{
					Status: core.NodeSucceeded,
				},
			},
		),
		runtime.NodeWithData(
			runtime.NodeData{
				Step: core.Step{Name: "step3", Command: "echo 3", Depends: []string{"step2"}},
				State: runtime.NodeState{
					Status: core.NodeSucceeded,
				},
			},
		),
	}

	// Test retrying a successful step in the middle
	graph, err := runtime.CreateStepRetryGraph(dag, nodes, "step2")
	require.NoError(t, err)
	require.NotNil(t, graph)

	// Only step2 should be reset, others remain unchanged
	require.Equal(t, core.NodeSucceeded, nodes[0].State().Status)  // step1 (unchanged)
	require.Equal(t, core.NodeNotStarted, nodes[1].State().Status) // step2 (reset)
	require.Equal(t, core.NodeSucceeded, nodes[2].State().Status)  // step3 (unchanged)

	// Test retrying the first successful step
	// Reset nodes to original state
	nodes[1].SetStatus(core.NodeSucceeded)

	graph, err = runtime.CreateStepRetryGraph(dag, nodes, "step1")
	require.NoError(t, err)
	require.NotNil(t, graph)

	// Only step1 should be reset
	require.Equal(t, core.NodeNotStarted, nodes[0].State().Status) // step1 (reset)
	require.Equal(t, core.NodeSucceeded, nodes[1].State().Status)  // step2 (unchanged)
	require.Equal(t, core.NodeSucceeded, nodes[2].State().Status)  // step3 (unchanged)

	// Test retrying the last successful step
	// Reset nodes to original state
	nodes[0].SetStatus(core.NodeSucceeded)

	graph, err = runtime.CreateStepRetryGraph(dag, nodes, "step3")
	require.NoError(t, err)
	require.NotNil(t, graph)

	// Only step3 should be reset
	require.Equal(t, core.NodeSucceeded, nodes[0].State().Status)  // step1 (unchanged)
	require.Equal(t, core.NodeSucceeded, nodes[1].State().Status)  // step2 (unchanged)
	require.Equal(t, core.NodeNotStarted, nodes[2].State().Status) // step3 (reset)
}

func TestExecutionGraphDependencies(t *testing.T) {
	t.Parallel()

	t.Run("BasicDependencies", func(t *testing.T) {
		// Create a DAG where IDs have been resolved to names (as done by builder)
		steps := []core.Step{
			{Name: "step1", ID: "first", Command: "echo 1"},
			{Name: "step2", ID: "second", Command: "echo 2", Depends: []string{"step1"}}, // ID resolved to name
			{Name: "step3", Command: "echo 3", Depends: []string{"step2", "step1"}},      // ID resolved to name
		}

		// Create execution graph
		graph, err := runtime.NewExecutionGraph(steps...)
		require.NoError(t, err)

		// Verify the graph was set up correctly
		require.NotNil(t, graph)
		require.Len(t, graph.From, 2) // step1 and step2 have outgoing edges
		require.Len(t, graph.To, 2)   // step2 and step3 have incoming edges
	})

	t.Run("ResolvedDependencies", func(t *testing.T) {
		// Test with dependencies already resolved by builder
		// In this case, the builder would have resolved "init" to "setup" based on ID
		steps := []core.Step{
			{Name: "setup", ID: "init", Command: "echo setup"},
			{Name: "init", Command: "echo init-by-name"},
			{Name: "process", Command: "echo process", Depends: []string{"setup"}}, // Resolved from ID to name
		}

		// Create execution graph
		graph, err := runtime.NewExecutionGraph(steps...)
		require.NoError(t, err)
		require.NotNil(t, graph)

		// Verify by checking the structure:
		// - graph should have edges in From and To maps
		// - there should be some connections
		require.NotEmpty(t, graph.From)
		require.NotEmpty(t, graph.To)

		// Check that we have the expected number of edges
		// setup -> process (1 edge)
		edgeCount := 0
		for _, targets := range graph.From {
			edgeCount += len(targets)
		}
		require.Equal(t, 1, edgeCount, "Should have exactly one edge: setup -> process")
	})
}

func TestGraphWithMixedDependencies(t *testing.T) {
	t.Parallel()

	// Dependencies have been resolved from IDs to names by the builder
	steps := []core.Step{
		{Name: "download", ID: "dl", Command: "wget file"},
		{Name: "extract", Command: "tar xf file"},
		{Name: "process", ID: "proc", Command: "process data", Depends: []string{"download", "extract"}}, // IDs resolved to names
		{Name: "cleanup", Command: "rm temp", Depends: []string{"process"}},                              // ID resolved to name
	}

	graph, err := runtime.NewExecutionGraph(steps...)
	require.NoError(t, err)
	require.NotNil(t, graph)

	// Verify correct dependency resolution
	// Expected edges:
	// download -> process
	// extract -> process
	// process -> cleanup
	// Total: 3 edges

	// Count total edges
	edgeCount := 0
	for _, targets := range graph.From {
		edgeCount += len(targets)
	}
	require.Equal(t, 3, edgeCount, "Should have exactly 3 edges")

	// Verify we have the right number of nodes with outgoing edges
	require.Len(t, graph.From, 3, "Three nodes should have outgoing edges: download, extract, process")

	// Verify we have the right number of nodes with incoming edges
	require.Len(t, graph.To, 2, "Two nodes should have incoming edges: process, cleanup")
}
