package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestParallelExecution_SimpleItems(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load parent DAG with parallel configuration
	dag := th.DAG(t, filepath.Join("integration", "parallel-simple.yaml"))

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// TODO: Once parallel execution is fully implemented, we should verify:
	// - Three child DAG runs were created
	// - They ran with correct parameters
	// - maxConcurrent was respected
}

func TestParallelExecution_ObjectItems(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load parent DAG with parallel object configuration
	dag := th.DAG(t, filepath.Join("integration", "parallel-objects.yaml"))

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// TODO: Once parallel execution is fully implemented, verify:
	// - Three child DAG runs with correct REGION and VERSION parameters
	// - Parameters were passed as JSON objects
}

func TestParallelExecution_VariableReference(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load parent DAG with variable reference
	dag := th.DAG(t, filepath.Join("integration", "parallel-variable.yaml"))

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// TODO: Verify four child DAG runs with items from JSON array
}

func TestParallelExecution_SpaceSeparated(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load parent DAG with space-separated variable
	dag := th.DAG(t, filepath.Join("integration", "parallel-space-separated.yaml"))

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// TODO: Verify three child DAG runs from space-separated values
}

func TestParallelExecution_DirectVariable(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load parent DAG with direct variable reference (not ${ITEMS} but $ITEMS)
	dag := th.DAG(t, filepath.Join("integration", "parallel-direct-variable.yaml"))

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status to verify parallel execution
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 2) // parallel-tasks and aggregate-results
	
	// Check parallel-tasks node
	parallelNode := status.Nodes[0]
	require.Equal(t, "parallel-tasks", parallelNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, parallelNode.Status)
	
	// Verify child DAG runs were created
	require.NotEmpty(t, parallelNode.Children)
	require.Len(t, parallelNode.Children, 3) // 3 child runs from the ITEMS array
}

func TestParallelExecution_WithOutput(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Create a DAG that uses parallel execution output
	testDir := test.TestdataPath(t, "integration")
	dagFile := filepath.Join(testDir, "test-parallel-output.yaml")
	dagContent := `name: test-parallel-output
steps:
  - name: parallel-with-output
    run: child-with-output
    parallel:
      items:
        - "A"
        - "B"
        - "C"
    output: PARALLEL_RESULTS
  - name: use-output
    command: |
      echo "Parallel execution results:"
      echo "${PARALLEL_RESULTS}"
    depends: parallel-with-output
    output: FINAL_OUTPUT
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(dagFile) })

	// Load and run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-output.yaml"))

	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status to verify outputs
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 2) // parallel-with-output and use-output
	
	// Check parallel-with-output node
	parallelNode := status.Nodes[0]
	require.Equal(t, "parallel-with-output", parallelNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, parallelNode.Status)
	require.Len(t, parallelNode.Children, 3) // 3 child runs
	
	// Check that the parallel execution produced output
	require.NotNil(t, parallelNode.OutputVariables)
	if value, ok := parallelNode.OutputVariables.Load("PARALLEL_RESULTS"); ok {
		parallelResults := value.(string)
		require.Contains(t, parallelResults, "PARALLEL_RESULTS=")
		require.Contains(t, parallelResults, `"summary"`)
		require.Contains(t, parallelResults, `"total": 3`) // Note: JSON has spaces after colons
		require.Contains(t, parallelResults, `"succeeded": 3`)
		require.Contains(t, parallelResults, `"results"`)
	} else {
		t.Fatal("PARALLEL_RESULTS output not found")
	}
	
	// Check use-output node
	useOutputNode := status.Nodes[1]
	require.Equal(t, "use-output", useOutputNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, useOutputNode.Status)
}

func TestParallelExecution_InvalidConfiguration(t *testing.T) {
	// Test validation errors by creating invalid DAG files
	t.Run("MissingChildDAG", func(t *testing.T) {
		th := test.Setup(t)
		dagFile := filepath.Join(th.Config.Paths.DAGsDir, "invalid-parallel.yaml")
		err := os.MkdirAll(filepath.Dir(dagFile), 0750)
		require.NoError(t, err)
		
		dagContent := `name: invalid-parallel
steps:
  - name: parallel-without-run
    command: echo "test"
    parallel:
      items: ["a", "b"]
`
		err = os.WriteFile(dagFile, []byte(dagContent), 0600)
		require.NoError(t, err)
		
		// This should fail during DAG loading due to validation
		_, err = digraph.Load(th.Context, dagFile)
		require.Error(t, err)
		require.Contains(t, err.Error(), "parallel execution is only supported for child-DAGs")
	})

	t.Run("NoItemsOrVariable", func(t *testing.T) {
		th := test.Setup(t)
		dagFile := filepath.Join(th.Config.Paths.DAGsDir, "invalid-parallel-empty.yaml")
		err := os.MkdirAll(filepath.Dir(dagFile), 0750)
		require.NoError(t, err)
		
		dagContent := `name: invalid-parallel-empty
steps:
  - name: empty-parallel
    run: child-echo
    parallel:
      maxConcurrent: 2
`
		err = os.WriteFile(dagFile, []byte(dagContent), 0600)
		require.NoError(t, err)
		
		// This should fail during DAG loading
		_, err = digraph.Load(th.Context, dagFile)
		require.Error(t, err)
		require.Contains(t, err.Error(), "parallel must have either items array or variable reference")
	})


	t.Run("InvalidMaxConcurrent", func(t *testing.T) {
		th := test.Setup(t)
		dagFile := filepath.Join(th.Config.Paths.DAGsDir, "invalid-max-concurrent.yaml")
		err := os.MkdirAll(filepath.Dir(dagFile), 0750)
		require.NoError(t, err)
		
		dagContent := `name: invalid-max-concurrent
steps:
  - name: invalid-concurrent
    run: child-echo
    parallel:
      items: ["a", "b"]
      maxConcurrent: 0
`
		err = os.WriteFile(dagFile, []byte(dagContent), 0600)
		require.NoError(t, err)
		
		// This should fail during DAG loading
		_, err = digraph.Load(th.Context, dagFile)
		require.Error(t, err)
		require.Contains(t, err.Error(), "maxConcurrent must be greater than 0")
	})
}

// TestParallelExecution_DeterministicIDs verifies that child DAG run IDs are deterministic
func TestParallelExecution_DeterministicIDs(t *testing.T) {
	// Create test DAGs in testdata directory
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	
	// The child-echo DAG already exists in testdata
	
	// Create a temporary test DAG that uses parallel execution
	testDir := test.TestdataPath(t, "integration")
	dagFile := filepath.Join(testDir, "test-deterministic-ids.yaml")
	dagContent := `name: test-deterministic-ids
steps:
  - name: process
    run: child-echo
    parallel:
      items:
        - "test1"
        - "test2"
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(dagFile) })

	// Load and run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-deterministic-ids.yaml"))

	// Run twice and verify same child DAG IDs are generated
	agent1 := dag.Agent()
	require.NoError(t, agent1.Run(agent1.Context))
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// TODO: Capture child DAG run IDs from first run

	// Run again
	agent2 := dag.Agent()
	require.NoError(t, agent2.Run(agent2.Context))
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// TODO: Verify same child DAG run IDs were generated
}