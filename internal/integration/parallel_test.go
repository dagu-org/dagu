package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
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
		// The first output could be from any of the parallel tasks
		require.Regexp(t, `TASK_RESULT_task[123]`, firstOutput)
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

// TestParallelExecution_OutputCaptureWithFailures verifies output behavior when some child DAGs fail
func TestParallelExecution_OutputCaptureWithFailures(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Create a simple child DAG that outputs data and fails/succeeds based on input
	testDir := test.TestdataPath(t, "integration")
	childDagFile := filepath.Join(testDir, "child-output-fail.yaml")
	childDagContent := `name: child-output-fail
steps:
  - name: process
    command: |
      INPUT="$1"
      echo "Output for ${INPUT}"
      if [ "${INPUT}" = "fail" ]; then
        exit 1
      fi
    output: RESULT
`
	err := os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })

	// Parent DAG with mixed success/failure
	parentDagFile := filepath.Join(testDir, "test-parallel-output-failures.yaml")
	parentDagContent := `name: test-parallel-output-failures
steps:
  - name: parallel-test
    run: child-output-fail
    parallel:
      items:
        - "success"
        - "fail"
    output: RESULTS
    continueOn:
      failure: true
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })

	// Run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-output-failures.yaml"))
	agent := dag.Agent()
	err = agent.Run(agent.Context)
	// Should fail because one child fails
	require.Error(t, err)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 1)

	// Check parallel node
	parallelNode := status.Nodes[0]
	require.Equal(t, "parallel-test", parallelNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusError, parallelNode.Status)

	// Verify output was captured for successful executions only
	require.NotNil(t, parallelNode.OutputVariables)
	if value, ok := parallelNode.OutputVariables.Load("RESULTS"); ok {
		results := value.(string)
		t.Logf("Captured results: %s", results)
		require.Contains(t, results, "RESULTS=")

		// Verify we got JSON output with both success and failure
		require.Contains(t, results, `"total": 2`)
		require.Contains(t, results, `"succeeded": 1`)
		require.Contains(t, results, `"failed": 1`)

		// Only successful output should be captured
		require.Contains(t, results, "Output for success")
		require.NotContains(t, results, "Output for fail")

		// Verify the failed execution has no output in results
		require.Contains(t, results, `"status": "failed"`)
		// Outputs array should only contain the successful output
		outputsSection := results[strings.Index(results, `"outputs": [`):]
		outputsEndIndex := strings.Index(outputsSection, `]`) + 1
		outputsContent := outputsSection[:outputsEndIndex]
		// Should only have 1 output (from the successful execution)
		require.Equal(t, 1, strings.Count(outputsContent, `"RESULT"`), "Outputs array should only contain successful output")
	} else {
		t.Fatal("RESULTS output not found")
	}
}

// TestParallelExecution_OutputCaptureWithRetry verifies output capture when child DAGs retry
func TestParallelExecution_OutputCaptureWithRetry(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Create a simple child DAG that fails first, succeeds on retry
	testDir := test.TestdataPath(t, "integration")

	// Clean up counter file after test
	counterFile := "/tmp/test_retry_counter.txt"
	t.Cleanup(func() { _ = os.Remove(counterFile) })

	childDagFile := filepath.Join(testDir, "child-retry-simple.yaml")
	childDagContent := `name: child-retry-simple
steps:
  - name: retry-step
    command: |
      COUNTER_FILE="/tmp/test_retry_counter.txt"
      if [ ! -f "$COUNTER_FILE" ]; then
        echo "1" > "$COUNTER_FILE"
        echo "First attempt"
        exit 1
      else
        echo "Retry success"
        exit 0
      fi
    output: OUTPUT
    retryPolicy:
      limit: 1
      intervalSec: 0
`
	err := os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })

	// Parent DAG with parallel execution
	parentDagFile := filepath.Join(testDir, "test-parallel-retry.yaml")
	parentDagContent := `name: test-parallel-retry
steps:
  - name: parallel-retry
    run: child-retry-simple
    parallel:
      items:
        - "item1"
    output: RESULTS
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })

	// Run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-retry.yaml"))
	agent := dag.Agent()
	err = agent.Run(agent.Context)
	require.NoError(t, err)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 1)

	// Check parallel node
	parallelNode := status.Nodes[0]
	require.Equal(t, "parallel-retry", parallelNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, parallelNode.Status)

	// Verify output was captured from retry
	require.NotNil(t, parallelNode.OutputVariables)
	if value, ok := parallelNode.OutputVariables.Load("RESULTS"); ok {
		results := value.(string)
		require.Contains(t, results, "RESULTS=")
		require.Contains(t, results, `"succeeded": 1`)
		require.Contains(t, results, `"failed": 0`)

		// Should contain retry output, not first attempt
		require.Contains(t, results, "Retry success")
		require.NotContains(t, results, "First attempt")
	} else {
		t.Fatal("RESULTS output not found")
	}
}

// TestParallelExecution_FailedChildOutputExclusion verifies that outputs from failed child DAGs are excluded
func TestParallelExecution_FailedChildOutputExclusion(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Create a child DAG that outputs data based on input
	testDir := test.TestdataPath(t, "integration")
	childDagFile := filepath.Join(testDir, "child-conditional-output.yaml")
	childDagContent := `name: child-conditional-output
steps:
  - name: process
    command: |
      INPUT="$1"
      if [ "${INPUT}" = "fail1" ]; then
        echo "Failed output 1"
        exit 1
      elif [ "${INPUT}" = "fail2" ]; then
        echo "Failed output 2"  
        exit 2
      else
        echo "Success output for ${INPUT}"
        exit 0
      fi
    output: RESULT
`
	err := os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })

	// Parent DAG with mixed success/failure items
	parentDagFile := filepath.Join(testDir, "test-parallel-output-exclusion.yaml")
	parentDagContent := `name: test-parallel-output-exclusion
steps:
  - name: parallel-mixed
    run: child-conditional-output
    parallel:
      items:
        - "success1"
        - "fail1"
        - "success2"
        - "fail2"
        - "success3"
    output: RESULTS
    continueOn:
      failure: true
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })

	// Run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-output-exclusion.yaml"))
	agent := dag.Agent()
	err = agent.Run(agent.Context)
	// Should fail because some children failed
	require.Error(t, err)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 1)

	// Check parallel node
	parallelNode := status.Nodes[0]
	require.Equal(t, "parallel-mixed", parallelNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusError, parallelNode.Status)

	// Verify output was captured
	require.NotNil(t, parallelNode.OutputVariables)
	if value, ok := parallelNode.OutputVariables.Load("RESULTS"); ok {
		results := value.(string)
		t.Logf("Captured results: %s", results)
		require.Contains(t, results, "RESULTS=")

		// Verify summary counts
		require.Contains(t, results, `"total": 5`)
		require.Contains(t, results, `"succeeded": 3`)
		require.Contains(t, results, `"failed": 2`)

		// Successful outputs should be included
		require.Contains(t, results, "Success output for success1")
		require.Contains(t, results, "Success output for success2")
		require.Contains(t, results, "Success output for success3")

		// Failed outputs should NOT be included
		require.NotContains(t, results, "Failed output 1")
		require.NotContains(t, results, "Failed output 2")

		// Verify outputs array only contains successful outputs
		require.Contains(t, results, `"outputs": [`)

		// The outputs array should only have 3 items (successful ones)
		// Count occurrences of RESULT in outputs array
		outputsSection := results[strings.Index(results, `"outputs": [`):]
		outputsEndIndex := strings.Index(outputsSection, `]`) + 1
		outputsContent := outputsSection[:outputsEndIndex]
		resultCount := strings.Count(outputsContent, `"RESULT": "Success output`)
		require.Equal(t, 3, resultCount, "Outputs array should only contain successful outputs")
	} else {
		t.Fatal("RESULTS output not found")
	}
}

// TestParallelExecution_ExceedsMaxLimit verifies that parallel execution with too many items fails
func TestParallelExecution_ExceedsMaxLimit(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Create a parent DAG with more than 1000 items
	testDir := test.TestdataPath(t, "integration")
	dagFile := filepath.Join(testDir, "test-parallel-exceed-limit.yaml")

	// Generate a large list of items (1001 items)
	items := make([]string, 1001)
	for i := 0; i < 1001; i++ {
		items[i] = fmt.Sprintf(`        - "item%d"`, i)
	}
	itemsStr := strings.Join(items, "\n")

	dagContent := fmt.Sprintf(`name: test-parallel-exceed-limit
steps:
  - name: too-many-items
    run: child-echo
    parallel:
      items:
%s
`, itemsStr)

	err := os.WriteFile(dagFile, []byte(dagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(dagFile) })

	// Load and run the DAG
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-exceed-limit.yaml"))
	agent := dag.Agent()

	// This should fail during execution when buildChildDAGRuns is called
	err = agent.Run(agent.Context)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parallel execution exceeds maximum limit: 1001 items (max: 1000)")
}

// TestParallelExecution_ExactlyMaxLimit verifies that exactly 1000 items works
func TestParallelExecution_ExactlyMaxLimit(t *testing.T) {
	// Run this test to verify the boundary condition

	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Create a parent DAG with exactly 1000 items
	testDir := test.TestdataPath(t, "integration")
	dagFile := filepath.Join(testDir, "test-parallel-max-limit.yaml")

	// Generate exactly 1000 items
	items := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = fmt.Sprintf(`        - "item%d"`, i)
	}
	itemsStr := strings.Join(items, "\n")

	dagContent := fmt.Sprintf(`name: test-parallel-max-limit
steps:
  - name: max-items
    run: child-echo
    parallel:
      items:
%s
      maxConcurrent: 10
`, itemsStr)

	err := os.WriteFile(dagFile, []byte(dagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(dagFile) })

	// Load the DAG - this should succeed
	dag := th.DAG(t, filepath.Join("integration", "test-parallel-max-limit.yaml"))

	// Load and start the DAG - this should succeed as we're at the exact limit
	agent := dag.Agent()

	// Start the DAG in a goroutine
	errChan := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		agent.Context = ctx
		errChan <- agent.Run(agent.Context)
	}()

	// Let it run for a short time to verify it starts without error
	select {
	case err := <-errChan:
		// If it finished quickly, it might be an error
		require.NoError(t, err, "DAG should not fail with exactly 1000 items")
	case <-time.After(1 * time.Second):
		// If it's still running after 1 second, it means it started successfully
		// Cancel it to clean up
		cancel()
		<-errChan // Wait for it to finish
	}
}

// TestParallelExecution_DynamicFileDiscovery tests the pattern from README where files are discovered dynamically
func TestParallelExecution_ObjectItemProperties(t *testing.T) {
	th := test.Setup(t)

	// Ensure the DAGs directory exists
	err := os.MkdirAll(th.Config.Paths.DAGsDir, 0755)
	require.NoError(t, err)

	// Create a child DAG that processes regions and buckets
	childDagFile := filepath.Join(th.Config.Paths.DAGsDir, "sync-data.yaml")
	childDagContent := `name: sync-data
params:
  - REGION: ""
  - BUCKET: ""
steps:
  - name: sync
    script: |
      echo "Syncing data from region: $REGION"
      echo "Using bucket: $BUCKET"
      echo "Sync completed for $BUCKET in $REGION"
    output: SYNC_RESULT
`
	err = os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })

	// Create the parent DAG that uses object items with property access
	parentDagFile := filepath.Join(th.Config.Paths.DAGsDir, "test-object-properties.yaml")
	parentDagContent := `name: test-object-properties
steps:
  - name: get configs
    command: |
      echo '[
        {"region": "us-east-1", "bucket": "data-us"},
        {"region": "eu-west-1", "bucket": "data-eu"},
        {"region": "ap-south-1", "bucket": "data-ap"}
      ]'
    output: CONFIGS
  
  - name: sync data
    run: sync-data
    parallel:
      items: ${CONFIGS}
      maxConcurrent: 2
    params:
      - REGION: ${ITEM.region}
      - BUCKET: ${ITEM.bucket}
    depends: get configs
    output: RESULTS
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })

	// Load and run the DAG
	dagStruct, err := digraph.Load(th.Context, parentDagFile)
	require.NoError(t, err)

	// Create the DAG wrapper
	dag := test.DAG{
		Helper: &th,
		DAG:    dagStruct,
	}

	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status to verify parallel execution
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 2) // get configs and sync data

	// Check get configs node
	getConfigsNode := status.Nodes[0]
	require.Equal(t, "get configs", getConfigsNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, getConfigsNode.Status)

	// Check sync data node
	syncNode := status.Nodes[1]
	require.Equal(t, "sync data", syncNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, syncNode.Status)

	// Verify child DAG runs were created
	require.NotEmpty(t, syncNode.Children)
	require.Len(t, syncNode.Children, 3) // 3 child runs for 3 regions

	// Verify parallel execution results
	require.NotNil(t, syncNode.OutputVariables)
	if value, ok := syncNode.OutputVariables.Load("RESULTS"); ok {
		results := value.(string)
		println(results)
		require.Contains(t, results, "RESULTS=")
		require.Contains(t, results, `"total": 3`)
		require.Contains(t, results, `"succeeded": 3`)
		require.Contains(t, results, `"failed": 0`)

		// Verify each region/bucket was processed correctly
		require.Contains(t, results, "Syncing data from region: us-east-1")
		require.Contains(t, results, "Using bucket: data-us")
		require.Contains(t, results, "Syncing data from region: eu-west-1")
		require.Contains(t, results, "Using bucket: data-eu")
		require.Contains(t, results, "Syncing data from region: ap-south-1")
		require.Contains(t, results, "Using bucket: data-ap")

		// Verify outputs contain the sync results
		require.Contains(t, results, "Sync completed for data-us in us-east-1")
		require.Contains(t, results, "Sync completed for data-eu in eu-west-1")
		require.Contains(t, results, "Sync completed for data-ap in ap-south-1")
	} else {
		t.Fatal("RESULTS output not found")
	}
}

func TestParallelExecution_DynamicFileDiscovery(t *testing.T) {
	th := test.Setup(t)

	// Create a temporary directory with test CSV files
	testDataDir := filepath.Join(th.Config.Paths.DAGsDir, "test-data")
	err := os.MkdirAll(testDataDir, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(testDataDir) })

	// Create test CSV files
	testFiles := []string{"data1.csv", "data2.csv", "data3.csv"}
	for _, file := range testFiles {
		filePath := filepath.Join(testDataDir, file)
		content := fmt.Sprintf("id,name\n1,%s\n", file)
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create a child DAG that processes a file
	childDagFile := filepath.Join(th.Config.Paths.DAGsDir, "process-file.yaml")
	childDagContent := `name: process-file
params:
  - ITEM: ""
steps:
  - name: process
    script: |
      FILE="$ITEM"
      echo "Processing file: ${FILE}"
      # Simulate file processing
      if [ -f "${FILE}" ]; then
        LINE_COUNT=$(wc -l < "${FILE}")
        echo "File has ${LINE_COUNT} lines"
      else
        echo "ERROR: File not found"
        exit 1
      fi
    output: PROCESS_RESULT
`
	err = os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })

	// Create the parent DAG that discovers and processes files
	parentDagFile := filepath.Join(th.Config.Paths.DAGsDir, "test-file-discovery.yaml")
	parentDagContent := fmt.Sprintf(`name: test-file-discovery
steps:
  - name: get files
    command: find %s -name "*.csv" -type f
    output: FILES
  
  - name: process files
    run: process-file
    parallel: ${FILES}
    params:
      - ITEM: ${ITEM}
    depends: get files
    output: RESULTS
`, testDataDir)
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })

	// Load and run the DAG
	dagStruct, err := digraph.Load(th.Context, parentDagFile)
	require.NoError(t, err)

	// Create the DAG wrapper
	dag := test.DAG{
		Helper: &th,
		DAG:    dagStruct,
	}

	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status to verify parallel execution
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 2) // get files and process files

	// Check get files node
	getFilesNode := status.Nodes[0]
	require.Equal(t, "get files", getFilesNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, getFilesNode.Status)

	// Verify FILES output contains the discovered files
	require.NotNil(t, getFilesNode.OutputVariables)
	if value, ok := getFilesNode.OutputVariables.Load("FILES"); ok {
		files := value.(string)
		// Should contain newline-separated file paths
		for _, testFile := range testFiles {
			require.Contains(t, files, testFile)
		}
	} else {
		t.Fatal("FILES output not found")
	}

	// Check process files node
	processNode := status.Nodes[1]
	require.Equal(t, "process files", processNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusSuccess, processNode.Status)

	// Verify child DAG runs were created for each file
	require.NotEmpty(t, processNode.Children)
	require.Len(t, processNode.Children, 3) // 3 child runs for 3 CSV files

	// Verify parallel execution results
	require.NotNil(t, processNode.OutputVariables)
	if value, ok := processNode.OutputVariables.Load("RESULTS"); ok {
		results := value.(string)
		require.Contains(t, results, "RESULTS=")
		require.Contains(t, results, `"total": 3`)
		require.Contains(t, results, `"succeeded": 3`)
		require.Contains(t, results, `"failed": 0`)

		// Each file should have been processed
		require.Contains(t, results, "Processing file:")
		require.Contains(t, results, "data1.csv")
		require.Contains(t, results, "data2.csv")
		require.Contains(t, results, "data3.csv")
		require.Regexp(t, `File has\s+2 lines`, results) // Each test file has 2 lines
	} else {
		t.Fatal("RESULTS output not found")
	}
}

// TestParallelExecution_StaticObjectItems tests parallel execution with static object items containing multiple properties
func TestParallelExecution_StaticObjectItems(t *testing.T) {
	th := test.Setup(t)

	// Ensure the DAGs directory exists
	err := os.MkdirAll(th.Config.Paths.DAGsDir, 0755)
	require.NoError(t, err)

	// Create a child DAG that deploys a service
	childDagFile := filepath.Join(th.Config.Paths.DAGsDir, "deploy-service.yaml")
	childDagContent := `name: deploy-service
params:
  - SERVICE_NAME: ""
  - PORT: ""
  - REPLICAS: ""
steps:
  - name: validate
    script: |
      echo "Validating deployment parameters..."
      if [ -z "$SERVICE_NAME" ] || [ -z "$PORT" ] || [ -z "$REPLICAS" ]; then
        echo "ERROR: Missing required parameters"
        exit 1
      fi
      echo "Service: $SERVICE_NAME"
      echo "Port: $PORT"
      echo "Replicas: $REPLICAS"
    output: VALIDATE_RESULT
  - name: deploy
    script: |
      echo "Deploying $SERVICE_NAME..."
      echo "  - Binding to port $PORT"
      echo "  - Scaling to $REPLICAS replicas"
      
      # Simulate deployment
      sleep 1
      
      # Simulate occasional failures for testing continueOnError
      if [ "$SERVICE_NAME" = "api-service" ]; then
        echo "ERROR: Failed to deploy $SERVICE_NAME - port $PORT already in use"
        exit 1
      fi
      
      echo "Successfully deployed $SERVICE_NAME"
    depends: validate
    output: DEPLOY_RESULT
`
	err = os.WriteFile(childDagFile, []byte(childDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(childDagFile) })

	// Create the parent DAG with static object items
	parentDagFile := filepath.Join(th.Config.Paths.DAGsDir, "test-static-objects.yaml")
	parentDagContent := `name: test-static-objects
steps:
  - name: deploy services
    run: deploy-service
    parallel:
      maxConcurrent: 3
      items:
        - name: web-service
          port: 8080
          replicas: 3
        - name: api-service
          port: 8081
          replicas: 2
        - name: worker-service
          port: 8082
          replicas: 5
    params:
      - SERVICE_NAME: ${ITEM.name}
      - PORT: ${ITEM.port}
      - REPLICAS: ${ITEM.replicas}
    continueOn:
      failure: true  # Continue even if some deployments fail
    output: DEPLOYMENT_RESULTS
`
	err = os.WriteFile(parentDagFile, []byte(parentDagContent), 0600)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(parentDagFile) })

	// Load and run the DAG
	dagStruct, err := digraph.Load(th.Context, parentDagFile)
	require.NoError(t, err)

	// Create the DAG wrapper
	dag := test.DAG{
		Helper: &th,
		DAG:    dagStruct,
	}

	agent := dag.Agent()
	err = agent.Run(agent.Context)
	// Should fail because api-service deployment fails, but continueOn.failure allows completion
	require.Error(t, err)

	// Get the latest status to verify parallel execution
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Len(t, status.Nodes, 1) // deploy services

	// Check deploy services node
	deployNode := status.Nodes[0]
	require.Equal(t, "deploy services", deployNode.Step.Name)
	require.Equal(t, scheduler.NodeStatusError, deployNode.Status) // Error because one child failed

	// Verify child DAG runs were created
	require.NotEmpty(t, deployNode.Children)
	require.Len(t, deployNode.Children, 3) // 3 child runs for 3 services

	// Verify parallel execution results
	require.NotNil(t, deployNode.OutputVariables)
	if value, ok := deployNode.OutputVariables.Load("DEPLOYMENT_RESULTS"); ok {
		results := value.(string)
		require.Contains(t, results, "DEPLOYMENT_RESULTS=")
		require.Contains(t, results, `"total": 3`)
		require.Contains(t, results, `"succeeded": 2`) // web-service and worker-service succeed
		require.Contains(t, results, `"failed": 1`)    // api-service fails

		// Verify service parameters were passed correctly for successful services
		require.Contains(t, results, "Service: web-service")
		require.Contains(t, results, "Port: 8080")
		require.Contains(t, results, "Replicas: 3")

		// api-service failed, so its output won't be in the results
		// Only successful child DAGs have their outputs captured

		require.Contains(t, results, "Service: worker-service")
		require.Contains(t, results, "Port: 8082")
		require.Contains(t, results, "Replicas: 5")

		// Verify deployment results
		require.Contains(t, results, "Successfully deployed web-service")
		require.Contains(t, results, "Successfully deployed worker-service")
		// api-service failed, so its output is not captured

		// Verify that successful outputs are captured
		require.Contains(t, results, "Binding to port 8080")
		require.Contains(t, results, "Scaling to 3 replicas")
		require.Contains(t, results, "Binding to port 8082")
		require.Contains(t, results, "Scaling to 5 replicas")
	} else {
		t.Fatal("DEPLOYMENT_RESULTS output not found")
	}
}
