package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// TestParallelExecution_MinimalRetry tests the minimal case of parallel execution with retry
func TestParallelExecution_MinimalRetry(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	
	// Create a simple child DAG that always fails
	testDir := test.TestdataPath(t, "integration")
	childDagFile := filepath.Join(testDir, "child-fail.yaml")
	childDagContent := `name: child-fail
steps:
  - name: fail
    command: exit 1
`
	err := os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })
	
	// Parent DAG with retry but NO continueOn
	parentDagFile := filepath.Join(testDir, "test-parallel-minimal.yaml")
	parentDagContent := `name: test-parallel-minimal
steps:
  - name: parallel-execution
    run: child-fail
    parallel:
      items:
        - "item1"
    retryPolicy:
      limit: 1
      intervalSec: 1
    output: RESULTS
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })
	
	// Run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-minimal.yaml"))
	agent := dag.Agent()
	err = agent.Run(agent.Context)
	require.Error(t, err, "DAG should fail")
	
	// Get status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	
	// Find the parallel node
	var parallelNode *models.Node
	for _, node := range status.Nodes {
		if node.Step.Name == "parallel-execution" {
			parallelNode = node
			break
		}
	}
	require.NotNil(t, parallelNode)
	
	t.Logf("Node status: %v (expected %v)", parallelNode.Status, scheduler.NodeStatusError)
	t.Logf("Retry count: %v", parallelNode.RetryCount)
	t.Logf("Error: %v", parallelNode.Error)
	
	// Should be marked as error (not success)
	require.Equal(t, scheduler.NodeStatusError, parallelNode.Status)
	require.Equal(t, 1, parallelNode.RetryCount)
}

// TestParallelExecution_RetryAndContinueOn tests both retry and continueOn together (the main issue)
func TestParallelExecution_RetryAndContinueOn(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	
	// Create a simple child DAG that always fails
	testDir := test.TestdataPath(t, "integration")
	childDagFile := filepath.Join(testDir, "child-fail-both.yaml")
	childDagContent := `name: child-fail-both
steps:
  - name: fail
    command: exit 1
`
	err := os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })
	
	// Parent DAG with BOTH retry and continueOn.failure (like the original failing test)
	parentDagFile := filepath.Join(testDir, "test-parallel-both.yaml")
	parentDagContent := `name: test-parallel-both
steps:
  - name: parallel-execution
    run: child-fail-both
    parallel:
      items:
        - "item1"
    retryPolicy:
      limit: 1
      intervalSec: 1
    continueOn:
      failure: true
    output: RESULTS
  - name: next-step
    command: echo "This should run"
    depends: parallel-execution
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })
	
	// Run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-both.yaml"))
	agent := dag.Agent()
	err = agent.Run(agent.Context)
	require.Error(t, err, "DAG should still fail overall")
	
	// Get status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	
	// Find nodes
	var parallelNode *models.Node
	var nextNode *models.Node
	for _, node := range status.Nodes {
		if node.Step.Name == "parallel-execution" {
			parallelNode = node
		}
		if node.Step.Name == "next-step" {
			nextNode = node
		}
	}
	require.NotNil(t, parallelNode)
	require.NotNil(t, nextNode)
	
	t.Logf("Parallel node status: %v (expected %v)", parallelNode.Status, scheduler.NodeStatusError)
	t.Logf("Retry count: %v", parallelNode.RetryCount)
	t.Logf("Next node status: %v", nextNode.Status)
	
	// THE KEY TEST: With retry AND continueOn.failure, should still be marked as error
	require.Equal(t, scheduler.NodeStatusError, parallelNode.Status, "Node should be marked as error, not success")
	require.Equal(t, 1, parallelNode.RetryCount)
	require.Equal(t, scheduler.NodeStatusSuccess, nextNode.Status)
	
	// Check if output was captured despite the error
	if parallelNode.OutputVariables != nil {
		if value, ok := parallelNode.OutputVariables.Load("RESULTS"); ok {
			results := value.(string)
			t.Logf("Captured output: %s", results)
			require.Contains(t, results, "RESULTS=")
		} else {
			t.Log("No output captured - this might be the bug we're fixing")
		}
	} else {
		t.Log("OutputVariables is nil - this might be the bug we're fixing")
	}
}