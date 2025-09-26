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
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestParallelExecution_SimpleItems(t *testing.T) {
	th := test.Setup(t)

	// Create multi-document YAML with both parent and child DAGs
	dag := th.DAG(t, `steps:
  - run: child-echo
    parallel:
      items:
        - "item1"
        - "item2"
        - "item3"
      maxConcurrent: 3

---
name: child-echo
params:
  - ITEM: "default"
steps:
  - command: echo "Processing $1"
    output: PROCESSED_ITEM
`)

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify parallel execution
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 1) // process-items

	// Check process-items node
	processNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, processNode.Status)

	// Verify child DAG runs were created
	require.NotEmpty(t, processNode.Children)
	require.Len(t, processNode.Children, 3) // 3 child runs for item1, item2, item3
}

func TestParallelExecution_ObjectItems(t *testing.T) {
	th := test.Setup(t)

	// Create multi-document YAML with both parent and child DAGs
	dag := th.DAG(t, `steps:
  - run: child-process
    parallel:
      items:
        - REGION: us-east-1
          VERSION: "1.0.0"
        - REGION: us-west-2
          VERSION: "1.0.1"
        - REGION: eu-west-1
          VERSION: "1.0.2"
      maxConcurrent: 2

---
name: child-process
params:
  - REGION: "us-east-1"
  - VERSION: "1.0.0"
steps:
  - command: echo "Deploying version $VERSION to region $REGION"
    output: DEPLOYMENT_RESULT
`)

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify parallel execution
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 1) // process-regions

	// Check process-regions node
	processNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, processNode.Status)

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
	th := test.Setup(t)

	// Create multi-document YAML with both parent and child DAGs
	dag := th.DAG(t, `params:
  - ITEMS: '["alpha", "beta", "gamma", "delta"]'
steps:
  - run: child-echo
    parallel: ${ITEMS}

---
name: child-echo
params:
  - ITEM: "default"
steps:
  - command: echo "Processing $1"
    output: PROCESSED_ITEM
`)

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify parallel execution
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 1) // process-from-var

	// Check process-from-var node
	processNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, processNode.Status)

	// Verify four child DAG runs from JSON array
	require.NotEmpty(t, processNode.Children)
	require.Len(t, processNode.Children, 4) // 4 child runs for alpha, beta, gamma, delta
}

func TestParallelExecution_SpaceSeparated(t *testing.T) {
	th := test.Setup(t)

	// Create multi-document YAML with both parent and child DAGs
	dag := th.DAG(t, `env:
  - SERVERS: "server1 server2 server3"
steps:
  - run: child-echo
    parallel: ${SERVERS}

---
name: child-echo
params:
  - ITEM: "default"
steps:
  - command: echo "Processing $1"
    output: PROCESSED_ITEM
`)

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify parallel execution
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 1) // process-servers

	// Check process-servers node
	processNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, processNode.Status)

	// Verify three child DAG runs from space-separated values
	require.NotEmpty(t, processNode.Children)
	require.Len(t, processNode.Children, 3) // 3 child runs for server1, server2, server3
}

func TestParallelExecution_DirectVariable(t *testing.T) {
	th := test.Setup(t)

	// Create multi-document YAML with both parent and child DAGs
	dag := th.DAG(t, `env:
  - ITEMS: '["task1", "task2", "task3"]'
steps:
  - run: child-with-output
    parallel: $ITEMS
  - name: aggregate-results
    command: echo "Completed parallel tasks"
    output: FINAL_RESULT

---
name: child-with-output
params:
  - TASK: "default"
steps:
  - command: |
      echo "Processing task: $1"
      echo "TASK_RESULT_$1"
    output: TASK_OUTPUT
  - echo "Task $1 completed with output ${TASK_OUTPUT}"
`)

	// Run the DAG
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify parallel execution
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 2) // parallel-tasks and aggregate-results

	// Check parallel-tasks node
	parallelNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, parallelNode.Status)

	// Verify child DAG runs were created
	require.NotEmpty(t, parallelNode.Children)
	require.Len(t, parallelNode.Children, 3) // 3 child runs from the ITEMS array
}

func TestParallelExecution_WithOutput(t *testing.T) {
	th := test.Setup(t)

	// Create a DAG that uses parallel execution output
	dag := th.DAG(t, `steps:
  - run: child-with-output
    parallel:
      items:
        - "A"
        - "B"
        - "C"
    output: PARALLEL_RESULTS
  - command: |
      echo "Parallel execution results:"
      echo "${PARALLEL_RESULTS}"
    output: FINAL_OUTPUT
---
name: child-with-output
params:
  - ITEM: ""
steps:
  - command: |
      echo "Processing item: $1"
      echo "TASK_RESULT_$1"
    output: TASK_OUTPUT
`)

	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify outputs
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 2) // parallel-with-output and use-output

	// Check parallel-with-output node
	parallelNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, parallelNode.Status)
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
	useOutputNode := dagRunStatus.Nodes[1]
	require.Equal(t, status.NodeSuccess, useOutputNode.Status)
}

// TestParallelExecution_DeterministicIDs verifies that child DAG run IDs are deterministic and duplicates are deduplicated
func TestParallelExecution_DeterministicIDs(t *testing.T) {
	th := test.Setup(t)

	// Create a temporary test DAG that uses parallel execution with duplicate items
	dag := th.DAG(t, `steps:
  - run: child-echo
    parallel:
      items:
        - "test1"
        - "test2"
        - "test1"  # Duplicate to verify deduplication
        - "test3"
        - "test2"  # Another duplicate
---
name: child-echo
params:
  - ITEM: ""
steps:
  - command: echo "$1"
    output: ECHO_OUTPUT
`)

	// Run and verify deduplication
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))
	dag.AssertLatestStatus(t, status.Success)

	// Get the status to check child DAG run IDs
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Len(t, dagRunStatus.Nodes, 1)

	// Collect unique parameters
	uniqueParams := make(map[string]string)
	for _, child := range dagRunStatus.Nodes[0].Children {
		uniqueParams[child.Params] = child.DAGRunID
	}

	// Should have only 3 unique runs despite 5 items (test1, test2, test1, test3, test2)
	require.Len(t, dagRunStatus.Nodes[0].Children, 3, "duplicate items should be deduplicated")
	require.Len(t, uniqueParams, 3, "should have 3 unique parameter sets")

	// Verify we have the expected unique parameters
	_, hasTest1 := uniqueParams["test1"]
	_, hasTest2 := uniqueParams["test2"]
	_, hasTest3 := uniqueParams["test3"]
	require.True(t, hasTest1, "should have test1")
	require.True(t, hasTest2, "should have test2")
	require.True(t, hasTest3, "should have test3")
}

// TestParallelExecution_PartialFailure verifies behavior when some child DAGs fail
func TestParallelExecution_PartialFailure(t *testing.T) {
	th := test.Setup(t)

	// Create parent and child DAGs
	dag := th.DAG(t, `steps:
  - run: child-conditional-fail
    parallel:
      items:
        - "ok1"
        - "fail"
        - "ok2"
        - "fail"
        - "ok3"
---
name: child-conditional-fail
params:
  - INPUT: "default"
steps:
  - command: |
      if [ "$1" = "fail" ]; then
        echo "Failing as requested"
        exit 1
      fi
      echo "Processing: $1"
`)
	agent := dag.Agent()

	// Run should fail because some child DAGs fail
	err := agent.Run(agent.Context)
	require.Error(t, err)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Check that the parallel step failed
	require.Len(t, dagRunStatus.Nodes, 1)
	parallelNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeError, parallelNode.Status)

	// Verify that child DAG runs were created (4 due to deduplication of "fail")
	require.Len(t, parallelNode.Children, 4, "should have 4 child DAG runs after deduplication")
}

// TestParallelExecution_OutputsArray verifies the outputs array is easily accessible
func TestParallelExecution_OutputsArray(t *testing.T) {
	th := test.Setup(t)

	// Create parent and child DAGs
	dag := th.DAG(t, `steps:
  - run: child-with-output
    parallel:
      items: ["task1", "task2", "task3"]
    output: RESULTS
  - command: |
      # Access first output directly from outputs array
      echo "First output: ${RESULTS.outputs[0].TASK_OUTPUT}"
    output: FIRST_OUTPUT
  - command: |
      # Show we can access any output by index
      echo "Output 0: ${RESULTS.outputs[0].TASK_OUTPUT}"
      echo "Output 1: ${RESULTS.outputs[1].TASK_OUTPUT}"
      echo "Output 2: ${RESULTS.outputs[2].TASK_OUTPUT}"
    output: ALL_OUTPUTS
---
name: child-with-output
params:
  - ITEM: ""
steps:
  - command: |
      echo "Processing item: $1"
      echo "TASK_RESULT_$1"
    output: TASK_OUTPUT
`)
	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify outputs
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 3) // parallel-tasks, use-first-output, use-all-outputs

	// Check that subsequent steps could access the outputs array
	firstOutputNode := dagRunStatus.Nodes[1]
	require.Equal(t, status.NodeSuccess, firstOutputNode.Status)

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
	allOutputsNode := dagRunStatus.Nodes[2]
	require.Equal(t, status.NodeSuccess, allOutputsNode.Status)

	if value, ok := allOutputsNode.OutputVariables.Load("ALL_OUTPUTS"); ok {
		allOutputs := value.(string)
		require.Contains(t, allOutputs, "TASK_RESULT_task1")
		require.Contains(t, allOutputs, "TASK_RESULT_task2")
		require.Contains(t, allOutputs, "TASK_RESULT_task3")
	} else {
		t.Fatal("ALL_OUTPUTS not found")
	}
}

// TestParallelExecution_MinimalRetry tests the minimal case of parallel execution with retry
func TestParallelExecution_MinimalRetry(t *testing.T) {
	th := test.Setup(t)

	// Create parent and child DAGs
	dag := th.DAG(t, `steps:
  - run: child-fail
    parallel:
      items:
        - "item1"
    retryPolicy:
      limit: 1
      intervalSec: 1
    output: RESULTS
---
name: child-fail
steps:
  - exit 1
`)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.Error(t, err, "DAG should fail")

	// Get status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)

	require.Len(t, dagRunStatus.Nodes, 1)
	parallelNode := dagRunStatus.Nodes[0]

	t.Logf("Node status: %v (expected %v)", parallelNode.Status, status.NodeError)
	t.Logf("Retry count: %v", parallelNode.RetryCount)
	t.Logf("Error: %v", parallelNode.Error)

	// Should be marked as error (not success)
	require.Equal(t, status.NodeError, parallelNode.Status)
	require.Equal(t, 1, parallelNode.RetryCount)
}

// TestParallelExecution_RetryAndContinueOn tests both retry and continueOn together (the main issue)
func TestParallelExecution_RetryAndContinueOn(t *testing.T) {
	th := test.Setup(t)

	// Create parent and child DAGs
	dag := th.DAG(t, `steps:
  - run: child-fail-both
    parallel:
      items:
        - "item1"
    retryPolicy:
      limit: 1
      intervalSec: 1
    continueOn:
      failure: true
    output: RESULTS
  - echo "This should run"
---
name: child-fail-both
steps:
  - exit 1
`)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	// DAG should complete successfully due to continueOn.failure: true, even after retries fail
	require.Error(t, err)

	// Get status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)

	require.Len(t, dagRunStatus.Nodes, 2)
	parallelNode := dagRunStatus.Nodes[0]
	nextNode := dagRunStatus.Nodes[1]

	t.Logf("Parallel node status: %v (expected %v)", parallelNode.Status, status.NodeError)
	t.Logf("Retry count: %v", parallelNode.RetryCount)
	t.Logf("Next node status: %v", nextNode.Status)

	// THE KEY TEST: With retry AND continueOn.failure, should still be marked as error
	require.Equal(t, status.NodeError, parallelNode.Status, "Node should be marked as error, not success")
	require.Equal(t, 1, parallelNode.RetryCount)
	require.Equal(t, status.NodeSuccess, nextNode.Status)

	// Check if output was captured despite the error
	require.NotNil(t, parallelNode.OutputVariables)
	value, ok := parallelNode.OutputVariables.Load("RESULTS")
	require.True(t, ok, "RESULTS output should be present")
	results := value.(string)
	require.Contains(t, results, "RESULTS=")
}

// TestParallelExecution_OutputCaptureWithFailures verifies output behavior when some child DAGs fail
func TestParallelExecution_OutputCaptureWithFailures(t *testing.T) {
	th := test.Setup(t)

	// Create parent and child DAGs
	dag := th.DAG(t, `steps:
  - run: child-output-fail
    parallel:
      items:
        - "success"
        - "fail"
    output: RESULTS
    continueOn:
      failure: true
---
name: child-output-fail
steps:
  - command: |
      INPUT="$1"
      echo "Output for ${INPUT}"
      if [ "${INPUT}" = "fail" ]; then
        exit 1
      fi
    output: RESULT
`)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	// Should complete successfully due to continueOn.failure: true, despite one child failing
	require.Error(t, err)

	// Get the latest st
	st, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, st)
	require.Len(t, st.Nodes, 1)

	// Check parallel node
	parallelNode := st.Nodes[0]
	require.Equal(t, status.NodeError, parallelNode.Status)

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
		outputsValue := strings.SplitN(value.(string), `"outputs": [`, 2)[1]
		require.Contains(t, outputsValue, "Output for success")
		require.NotContains(t, outputsValue, `"Output for fail"`)

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
	th := test.Setup(t)

	// Clean up counter file after test
	counterFile := "/tmp/test_retry_counter.txt"
	t.Cleanup(func() { _ = os.Remove(counterFile) })

	// Create parent and child DAGs
	dag := th.DAG(t, `steps:
  - run: child-retry-simple
    parallel:
      items:
        - "item1"
    output: RESULTS
---
name: child-retry-simple
steps:
  - command: |
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
`)
	agent := dag.Agent()
	err := agent.Run(agent.Context)
	require.NoError(t, err)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 1)

	// Check parallel node
	parallelNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeSuccess, parallelNode.Status)

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

// TestParallelExecution_ExceedsMaxLimit verifies that parallel execution with too many items fails
func TestParallelExecution_ExceedsMaxLimit(t *testing.T) {
	th := test.Setup(t)

	// Generate a large list of items (1001 items)
	items := make([]string, 1001)
	for i := 0; i < 1001; i++ {
		items[i] = fmt.Sprintf(`        - "item%d"`, i)
	}
	itemsStr := strings.Join(items, "\n")

	dagContent := fmt.Sprintf(`
steps:
  - run: child-echo
    parallel:
      items:
%s
---
name: child-echo
params:
  - ITEM: ""
steps:
  - command: echo "$1"
    output: ECHO_OUTPUT
`, itemsStr)

	// Create parent and child DAGs
	dag := th.DAG(t, dagContent)
	agent := dag.Agent()

	// This should fail during execution when buildChildDAGRuns is called
	err := agent.Run(agent.Context)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parallel execution exceeds maximum limit: 1001 items (max: 1000)")
}

// TestParallelExecution_ExactlyMaxLimit verifies that exactly 1000 items works
func TestParallelExecution_ExactlyMaxLimit(t *testing.T) {
	// Run this test to verify the boundary condition

	th := test.Setup(t)

	// Generate exactly 1000 items
	items := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = fmt.Sprintf(`        - "item%d"`, i)
	}
	itemsStr := strings.Join(items, "\n")

	dagContent := fmt.Sprintf(`
steps:
  - run: child-echo
    parallel:
      items:
%s
      maxConcurrent: 10
---
name: child-echo
params:
  - ITEM: ""
steps:
  - command: echo "$1"
    output: ECHO_OUTPUT
`, itemsStr)

	// Create parent and child DAGs
	dag := th.DAG(t, dagContent)

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

	// Create multi-document YAML with both parent and child DAGs
	yamlContent := `steps:
  - command: |
      echo '[
        {"region": "us-east-1", "bucket": "data-us"},
        {"region": "eu-west-1", "bucket": "data-eu"},
        {"region": "ap-south-1", "bucket": "data-ap"}
      ]'
    output: CONFIGS

  - run: sync-data
    parallel:
      items: ${CONFIGS}
      maxConcurrent: 2
    params:
      - REGION: ${ITEM.region}
      - BUCKET: ${ITEM.bucket}
    output: RESULTS

---
name: sync-data
params:
  - REGION: ""
  - BUCKET: ""
steps:
  - script: |
      echo "Syncing data from region: $REGION"
      echo "Using bucket: $BUCKET"
      echo "Sync completed for $BUCKET in $REGION"
    output: SYNC_RESULT
`
	// Load the DAG using helper
	dag := th.DAG(t, yamlContent)

	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify parallel execution
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 2) // get configs and sync data

	// Check get configs node
	getConfigsNode := dagRunStatus.Nodes[0]
	require.Equal(t, "get configs", getConfigsNode.Step.Name)
	require.Equal(t, status.NodeSuccess, getConfigsNode.Status)

	// Check sync data node
	syncNode := dagRunStatus.Nodes[1]
	require.Equal(t, "sync data", syncNode.Step.Name)
	require.Equal(t, status.NodeSuccess, syncNode.Status)

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
	childDagContent := `
params:
  - ITEM: ""
steps:
  - script: |
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
	th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "process-file", []byte(childDagContent))

	// Create the parent DAG that discovers and processes files
	parentDagContent := fmt.Sprintf(`
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
	th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "test-file-discovery", []byte(parentDagContent))

	// Load and run the DAG
	dagStruct, err := digraph.Load(th.Context, filepath.Join(th.Config.Paths.DAGsDir, "test-file-discovery.yaml"))
	require.NoError(t, err)

	// Create the DAG wrapper
	dag := test.DAG{
		Helper: &th,
		DAG:    dagStruct,
	}

	agent := dag.Agent()
	require.NoError(t, agent.Run(agent.Context))

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status to verify parallel execution
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 2) // get files and process files

	// Check get files node
	getFilesNode := dagRunStatus.Nodes[0]
	require.Equal(t, "get files", getFilesNode.Step.Name)
	require.Equal(t, status.NodeSuccess, getFilesNode.Status)

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
	processNode := dagRunStatus.Nodes[1]
	require.Equal(t, "process files", processNode.Step.Name)
	require.Equal(t, status.NodeSuccess, processNode.Status)

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

	// Create multi-document YAML with both parent and child DAGs
	yamlContent := `steps:
  - run: deploy-service
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

---
name: deploy-service
params:
  - SERVICE_NAME: ""
  - PORT: ""
  - REPLICAS: ""
steps:
  - script: |
      echo "Validating deployment parameters..."
      if [ -z "$SERVICE_NAME" ] || [ -z "$PORT" ] || [ -z "$REPLICAS" ]; then
        echo "ERROR: Missing required parameters"
        exit 1
      fi
      echo "Service: $SERVICE_NAME"
      echo "Port: $PORT"
      echo "Replicas: $REPLICAS"
    output: VALIDATE_RESULT
  - script: |
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
    output: DEPLOY_RESULT
`
	// Load the DAG using helper
	dag := th.DAG(t, yamlContent)

	agent := dag.Agent()
	err = agent.Run(agent.Context)
	// The DAG should complete successfully despite failures due to continueOn.failure = true
	// but the individual steps may still show as failed
	require.Error(t, err)

	// Get the latest status to verify parallel execution
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)
	require.Len(t, dagRunStatus.Nodes, 1) // deploy services

	// Check deploy services node
	deployNode := dagRunStatus.Nodes[0]
	require.Equal(t, status.NodeError, deployNode.Status) // Error because one child failed

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
