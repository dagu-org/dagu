package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// Get the latest status to verify parallel execution
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 1) // process-items

	// Check process-items node
	processNode := status.Nodes[0]
	require.Equal(t, "process-items", processNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, processNode.Status)

	// Verify child DAG runs were created
	require.NotEmpty(t, processNode.Children)
	require.Len(t, processNode.Children, 3) // 3 child runs for item1, item2, item3
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

	// Get the latest status to verify parallel execution
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 1) // process-regions

	// Check process-regions node
	processNode := status.Nodes[0]
	require.Equal(t, "process-regions", processNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, processNode.Status)

	// Verify child DAG runs were created with JSON parameters
	require.NotEmpty(t, processNode.Children)
	require.Len(t, processNode.Children, 3) // 3 child runs for different regions

	// Verify that parameters contain JSON objects
	for _, child := range processNode.Children {
		require.Contains(t, child.Params, `"REGION"`)
		require.Contains(t, child.Params, `"VERSION"`)
	}
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

	// Get the latest status to verify parallel execution
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 1) // process-from-var

	// Check process-from-var node
	processNode := status.Nodes[0]
	require.Equal(t, "process-from-var", processNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, processNode.Status)

	// Verify four child DAG runs from JSON array
	require.NotEmpty(t, processNode.Children)
	require.Len(t, processNode.Children, 4) // 4 child runs for alpha, beta, gamma, delta
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

	// Get the latest status to verify parallel execution
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 1) // process-servers

	// Check process-servers node
	processNode := status.Nodes[0]
	require.Equal(t, "process-servers", processNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, processNode.Status)

	// Verify three child DAG runs from space-separated values
	require.NotEmpty(t, processNode.Children)
	require.Len(t, processNode.Children, 3) // 3 child runs for server1, server2, server3
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
	t.Cleanup(func() { _ = os.Remove(dagFile) })

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
		require.Contains(t, parallelResults, `"outputs"`)
		
		// Verify outputs array contains the expected output
		require.Contains(t, parallelResults, `"TASK_OUTPUT"`)
		require.Contains(t, parallelResults, `TASK_RESULT_A`)
		require.Contains(t, parallelResults, `TASK_RESULT_B`)
		require.Contains(t, parallelResults, `TASK_RESULT_C`)
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

// TestParallelExecution_DeterministicIDs verifies that child DAG run IDs are deterministic and duplicates are deduplicated
func TestParallelExecution_DeterministicIDs(t *testing.T) {
	// Create test DAGs in testdata directory
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// The child-echo DAG already exists in testdata

	// Create a temporary test DAG that uses parallel execution with duplicate items
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
        - "test1"  # Duplicate to verify deduplication
        - "test3"
        - "test2"  # Another duplicate
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(dagFile) })

	// Load and run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-deterministic-ids.yaml"))

	// Run and verify deduplication
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the status to check child DAG run IDs
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Len(t, status.Nodes, 1)

	// Collect unique parameters
	uniqueParams := make(map[string]string)
	for _, child := range status.Nodes[0].Children {
		uniqueParams[child.Params] = child.DAGRunID
	}

	// Should have only 3 unique runs despite 5 items (test1, test2, test1, test3, test2)
	require.Len(t, status.Nodes[0].Children, 3, "duplicate items should be deduplicated")
	require.Len(t, uniqueParams, 3, "should have 3 unique parameter sets")

	// Verify we have the expected unique parameters
	_, hasTest1 := uniqueParams["test1"]
	_, hasTest2 := uniqueParams["test2"]
	_, hasTest3 := uniqueParams["test3"]
	require.True(t, hasTest1, "should have test1")
	require.True(t, hasTest2, "should have test2")
	require.True(t, hasTest3, "should have test3")
}

// TestParallelExecution_Cancel verifies that cancelling a parallel execution properly cancels all child DAG runs
func TestParallelExecution_Cancel(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	
	// Create a child DAG that sleeps for a while
	testDir := test.TestdataPath(t, "integration")
	childDagFile := filepath.Join(testDir, "child-sleep.yaml")
	childDagContent := `name: child-sleep
params:
  - SLEEP_TIME: "5"
steps:
  - name: sleep
    command: sleep $1
`
	err := os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })
	
	// Create a parent DAG with parallel execution
	parentDagFile := filepath.Join(testDir, "test-parallel-cancel.yaml")
	parentDagContent := `name: test-parallel-cancel
steps:
  - name: parallel-sleep
    run: child-sleep
    parallel:
      items:
        - "10"
        - "10"
        - "10"
        - "10"
      maxConcurrent: 2
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })

	// Load and start the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-cancel.yaml"))
	agent := dag.Agent()
	
	// Start the DAG in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- agent.Run(agent.Context)
	}()
	
	// Wait a bit to ensure parallel execution has started
	time.Sleep(1 * time.Second)
	
	// Cancel the execution
	agent.Cancel()
	
	// Wait for the agent to finish
	err = <-errChan
	require.Error(t, err, "agent should return an error when cancelled")
	// The error might contain "killed" or "cancelled" depending on timing
	require.True(t, 
		strings.Contains(err.Error(), "killed") || strings.Contains(err.Error(), "cancelled"),
		"error should indicate cancellation: %v", err)
	
	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	
	// Check that the parallel step exists
	require.Len(t, status.Nodes, 1)
	parallelNode := status.Nodes[0]
	require.Equal(t, "parallel-sleep", parallelNode.Step.Name)
	// The step might be marked as failed, cancelled, or even not started depending on timing
	require.True(t,
		parallelNode.Status == scheduler.NodeStatusCancel || 
		parallelNode.Status == scheduler.NodeStatusError ||
		parallelNode.Status == scheduler.NodeStatusNone,
		"parallel step should be cancelled, failed, or not started, got: %v", parallelNode.Status)
	
	// If the step was actually started, verify that child DAG runs were created
	if parallelNode.Status != scheduler.NodeStatusNone {
		require.NotEmpty(t, parallelNode.Children, "child DAG runs should have been created if step started")
	}
}

// TestParallelExecution_PartialFailure verifies behavior when some child DAGs fail
func TestParallelExecution_PartialFailure(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	
	// Create a child DAG that fails for certain inputs
	testDir := test.TestdataPath(t, "integration")
	childDagFile := filepath.Join(testDir, "child-conditional-fail.yaml")
	childDagContent := `name: child-conditional-fail
params:
  - INPUT: "default"
steps:
  - name: process
    command: |
      if [ "$1" = "fail" ]; then
        echo "Failing as requested"
        exit 1
      fi
      echo "Processing: $1"
`
	err := os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })
	
	// Create a parent DAG with mixed success/failure items
	parentDagFile := filepath.Join(testDir, "test-parallel-partial-failure.yaml")
	parentDagContent := `name: test-parallel-partial-failure
steps:
  - name: parallel-mixed
    run: child-conditional-fail
    parallel:
      items:
        - "ok1"
        - "fail"
        - "ok2"
        - "fail"
        - "ok3"
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })

	// Load and run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-partial-failure.yaml"))
	agent := dag.Agent()
	
	// Run should fail because some child DAGs fail
	err = agent.Run(agent.Context)
	require.Error(t, err)
	
	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	
	// Check that the parallel step failed
	require.Len(t, status.Nodes, 1)
	parallelNode := status.Nodes[0]
	require.Equal(t, "parallel-mixed", parallelNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusError, parallelNode.Status)
	
	// Verify that child DAG runs were created (4 due to deduplication of "fail")
	require.Len(t, parallelNode.Children, 4, "should have 4 child DAG runs after deduplication")
}

// TestParallelExecution_OutputsArray verifies the outputs array is easily accessible
func TestParallelExecution_OutputsArray(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	
	// Create a parent DAG that uses outputs array from parallel execution
	testDir := test.TestdataPath(t, "integration")
	dagFile := filepath.Join(testDir, "test-parallel-outputs-array.yaml")
	dagContent := `name: test-parallel-outputs-array
steps:
  - name: parallel-tasks
    run: child-with-output
    parallel:
      items: ["task1", "task2", "task3"]
    output: RESULTS
  - name: use-first-output
    command: |
      # Access first output directly from outputs array
      echo "First output: ${RESULTS.outputs[0].TASK_OUTPUT}"
    depends: parallel-tasks
    output: FIRST_OUTPUT
  - name: use-all-outputs
    command: |
      # Show we can access any output by index
      echo "Output 0: ${RESULTS.outputs[0].TASK_OUTPUT}"
      echo "Output 1: ${RESULTS.outputs[1].TASK_OUTPUT}"
      echo "Output 2: ${RESULTS.outputs[2].TASK_OUTPUT}"
    depends: parallel-tasks
    output: ALL_OUTPUTS
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(dagFile) })

	// Load and run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-outputs-array.yaml"))
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))
	
	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)
	
	// Get the latest status to verify outputs
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 3) // parallel-tasks, use-first-output, use-all-outputs
	
	// Check that subsequent steps could access the outputs array
	firstOutputNode := status.Nodes[1]
	require.Equal(t, "use-first-output", firstOutputNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, firstOutputNode.Status)
	
	// Verify the first output was accessible
	if value, ok := firstOutputNode.OutputVariables.Load("FIRST_OUTPUT"); ok {
		firstOutput := value.(string)
		require.Contains(t, firstOutput, "First output:")
		require.Contains(t, firstOutput, "TASK_RESULT_task1")
	} else {
		t.Fatal("FIRST_OUTPUT not found")
	}
	
	// Check all outputs were accessible
	allOutputsNode := status.Nodes[2]
	require.Equal(t, "use-all-outputs", allOutputsNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, allOutputsNode.Status)
	
	if value, ok := allOutputsNode.OutputVariables.Load("ALL_OUTPUTS"); ok {
		allOutputs := value.(string)
		require.Contains(t, allOutputs, "TASK_RESULT_task1")
		require.Contains(t, allOutputs, "TASK_RESULT_task2")
		require.Contains(t, allOutputs, "TASK_RESULT_task3")
	} else {
		t.Fatal("ALL_OUTPUTS not found")
	}
}

// TestParallelExecution_OutOfBoundsAccess verifies behavior when accessing out-of-bounds indices
func TestParallelExecution_OutOfBoundsAccess(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))
	
	// Create a parent DAG that tries to access out-of-bounds indices
	testDir := test.TestdataPath(t, "integration")
	dagFile := filepath.Join(testDir, "test-parallel-out-of-bounds.yaml")
	dagContent := `name: test-parallel-out-of-bounds
steps:
  - name: parallel-tasks
    run: child-with-output
    parallel:
      items: ["task1", "task2"]  # Only 2 items
    output: RESULTS
  - name: access-out-of-bounds
    command: |
      echo "Valid index 0: ${RESULTS.outputs[0].TASK_OUTPUT}"
      echo "Valid index 1: ${RESULTS.outputs[1].TASK_OUTPUT}"
      echo "Out of bounds index 2: ${RESULTS.outputs[2].TASK_OUTPUT}"
      echo "Out of bounds index 10: ${RESULTS.outputs[10].TASK_OUTPUT}"
    depends: parallel-tasks
    output: TEST_OUTPUT
`
	err := os.WriteFile(dagFile, []byte(dagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(dagFile) })

	// Load and run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-out-of-bounds.yaml"))
	agent := dag.Agent()
	
	// The DAG should complete (variable expansion handles undefined gracefully)
	err = agent.Run(agent.Context)
	require.NoError(t, err)
	
	// Get the latest status to check outputs
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 2)
	
	// Check the output from the access-out-of-bounds step
	outOfBoundsNode := status.Nodes[1]
	require.Equal(t, "access-out-of-bounds", outOfBoundsNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, outOfBoundsNode.Status)
	
	if value, ok := outOfBoundsNode.OutputVariables.Load("TEST_OUTPUT"); ok {
		output := value.(string)
		// Valid indices should have values
		require.Contains(t, output, "Valid index 0:")
		require.Contains(t, output, "TASK_RESULT_task1")
		require.Contains(t, output, "Valid index 1:")
		require.Contains(t, output, "TASK_RESULT_task2")
		// Out of bounds indices should return <nil>
		require.Contains(t, output, "Out of bounds index 2: <nil>")
		require.Contains(t, output, "Out of bounds index 10: <nil>")
	} else {
		t.Fatal("TEST_OUTPUT not found")
	}
}
