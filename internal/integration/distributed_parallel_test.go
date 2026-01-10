package integration_test

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestParallelDistributedExecution(t *testing.T) {
	t.Run("ParallelExecutionOnWorkers", func(t *testing.T) {
		// Create test DAGs with parallel execution using workerSelector
		yamlContent := `
steps:
  - name: process-items
    call: child-worker
    parallel:
      items:
        - "item1"
        - "item2"
        - "item3"
      maxConcurrent: 2
    output: RESULTS

---
name: child-worker
workerSelector:
  type: test-worker
steps:
  - name: process
    command: echo "Processing $1 on worker"
    output: RESULT
`
		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		// Create and start multiple workers to handle parallel execution
		setupWorkers(t, coord, 2, map[string]string{"type": "test-worker"})

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()

		// Run the DAG
		agent.RunSuccess(t)

		// Verify the DAG completed successfully
		dagWrapper.AssertLatestStatus(t, core.Succeeded)

		// Get the latest st to verify parallel execution
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)
		require.Len(t, st.Nodes, 1) // process-items

		// Check process-items node
		processNode := st.Nodes[0]
		require.Equal(t, "process-items", processNode.Step.Name)
		require.Equal(t, core.NodeSucceeded, processNode.Status)

		// Verify sub DAG runs were created
		require.NotEmpty(t, processNode.SubRuns)
		require.Len(t, processNode.SubRuns, 3) // 3 sub runs

		// Verify all children completed successfully
		for _, child := range processNode.SubRuns {
			// Each child should have been executed on a worker
			require.Contains(t, child.Params, "item")
		}

		// Verify output was captured
		require.NotNil(t, processNode.OutputVariables)
		if value, ok := processNode.OutputVariables.Load("RESULTS"); ok {
			results := value.(string)
			require.Contains(t, results, "RESULTS=")
			require.Contains(t, results, `"total": 3`)
			require.Contains(t, results, `"succeeded": 3`)
			require.Contains(t, results, `"failed": 0`)

			// Verify each item was processed
			require.Contains(t, results, "Processing item1 on worker")
			require.Contains(t, results, "Processing item2 on worker")
			require.Contains(t, results, "Processing item3 on worker")
		} else {
			t.Fatal("RESULTS output not found")
		}
	})

	t.Run("ParallelDistributedWithSameWorkerType", func(t *testing.T) {
		// Test parallel execution where all items go to the same worker type
		yamlContent := `
steps:
  - name: process-regions
    call: child-regional
    parallel:
      items:
        - "us-east"
        - "eu-west"
        - "ap-south"
    output: RESULTS

---
name: child-regional
workerSelector:
  type: test-worker
steps:
  - name: process
    command: |
      echo "Processing region: $1"
    output: RESULT
`
		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		// Create multiple workers of the same type
		setupWorkers(t, coord, 3, map[string]string{"type": "test-worker"})

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()
		agent.RunSuccess(t)

		// Verify successful completion
		dagWrapper.AssertLatestStatus(t, core.Succeeded)

		// Get st to verify execution
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Check the parallel node
		processNode := st.Nodes[0]
		require.Equal(t, "process-regions", processNode.Step.Name)
		require.Equal(t, core.NodeSucceeded, processNode.Status)
		require.Len(t, processNode.SubRuns, 3)

		// Verify output shows all regions were processed
		if value, ok := processNode.OutputVariables.Load("RESULTS"); ok {
			results := value.(string)
			require.Contains(t, results, "Processing region: us-east")
			require.Contains(t, results, "Processing region: eu-west")
			require.Contains(t, results, "Processing region: ap-south")
			require.Contains(t, results, `"succeeded": 3`)
		} else {
			t.Fatal("RESULTS output not found")
		}
	})

	t.Run("ParallelDistributedFailurePropagatesToParentStep", func(t *testing.T) {
		yamlContent := `
steps:
  - name: process-items
    call: child-worker
    parallel:
      items:
        - "ok"
        - "fail"

---
name: child-worker
workerSelector:
  type: test-worker
steps:
  - name: run
    command: |
      if [ "$1" = "fail" ]; then
        echo "Simulated failure"
        exit 1
      fi
      echo "Processed $1"
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		setupWorker(t, coord, "test-worker-failure", 10, map[string]string{"type": "test-worker"})

		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()
		err := agent.Run(agent.Context)
		require.Error(t, err)

		st, statusErr := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, statusErr)
		require.NotNil(t, st)
		require.Len(t, st.Nodes, 1)

		node := st.Nodes[0]
		require.Equal(t, "process-items", node.Step.Name)
		require.Equal(t, core.NodeFailed, node.Status)
		require.Len(t, node.SubRuns, 2)
	})

	t.Run("ParallelDistributedWithNoMatchingWorkers", func(t *testing.T) {
		// Test that parallel execution fails gracefully when no workers match
		yamlContent := `
steps:
  - name: process-items
    call: child-nonexistent
    parallel:
      items: ["a", "b", "c"]
    output: RESULTS

---
name: child-nonexistent
workerSelector:
  type: nonexistent-worker
steps:
  - name: process
    command: echo "Should not run"
`
		// Setup coordinator without matching workers
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()

		// Run should fail because no workers match
		// Use a short timeout since we expect this to fail
		ctx, cancel := context.WithTimeout(coord.Context, 5*time.Second)
		defer cancel()
		err := agent.Run(ctx)
		require.Error(t, err)

		// Verify the DAG did not complete successfully
		st := agent.Status(coord.Context)
		require.NotEqual(t, core.Succeeded, st.Status)
	})
}

func TestParallelDistributedCancellation(t *testing.T) {
	t.Run("CancelParallelExecutionOnWorkers", func(t *testing.T) {
		// Create test DAGs with parallel execution using workerSelector
		yamlContent := `
steps:
  - name: process-items
    call: child-sleep
    parallel:
      items:
        - "100"
        - "101"
        - "102"
        - "103"
      maxConcurrent: 2

---
name: child-sleep
workerSelector:
  type: test-worker
steps:
  - name: sleep
    command: sleep $1
`
		// Setup and start coordinator
		tmpDir := t.TempDir()
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir), test.WithStatusPersistence())

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Create and start multiple workers to handle parallel execution
		setupWorkers(t, coord, 2, map[string]string{"type": "test-worker"})

		// Load the DAG using helper
		dag := coord.DAG(t, yamlContent)

		// Create agent with cancellable context
		agent := dag.Agent()
		done := make(chan struct{})

		// Start the DAG in a goroutine
		go func() {
			agent.Context = coord.Context
			_ = agent.Run(agent.Context)
			close(done)
		}()

		// Wait the step to be running
		require.Eventually(t, func() bool {
			st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dag.DAG)
			if err != nil || !st.Status.IsActive() {
				return false
			}
			if len(st.Nodes) == 0 {
				return false
			}
			parallelNode := st.Nodes[0]
			return parallelNode.Status == core.NodeRunning
		}, 5*time.Second, 100*time.Millisecond)

		// Cancel the execution after waiting workers are processing distributed tasks
		require.Eventually(t, func() bool {
			workerInfo, err := coordinatorClient.GetWorkers(coord.Context)
			require.NoError(t, err)
			var runningTasks int
			for _, w := range workerInfo {
				runningTasks += len(w.RunningTasks)
			}
			return runningTasks > 0
		}, 5*time.Second, 100*time.Millisecond)

		// Perform cancellation
		agent.Signal(coord.Context, os.Signal(syscall.SIGINT))

		// Wait for the agent to finish
		<-done

		// Get the latest st
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dag.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Check that the parallel step exists
		require.GreaterOrEqual(t, len(st.Nodes), 1)
		parallelNode := st.Nodes[0]
		require.Equal(t, "process-items", parallelNode.Step.Name)

		// Verify that the parallel step status
		require.Equal(t, core.NodeAborted, parallelNode.Status)

		// Verify sub DAG runs were cancelled
		runRef := execution.NewDAGRunRef(st.Name, st.DAGRunID)
		var canceled bool
		for _, child := range parallelNode.SubRuns {
			att, findErr := coord.DAGRunStore.FindSubAttempt(coord.Context, runRef, child.DAGRunID)
			if findErr != nil || att == nil {
				continue
			}

			status, readErr := att.ReadStatus(coord.Context)
			require.NoError(t, readErr)
			require.Equal(t, core.Aborted, status.Status)
			canceled = true
		}
		require.True(t, canceled, "expected at least one sub DAG run to be cancelled")

		// If the step was actually started, verify that sub DAG runs were created
		if parallelNode.Status != core.NodeNotStarted && len(parallelNode.SubRuns) > 0 {
			// Verify that distributed sub runs were cancelled
			for _, child := range parallelNode.SubRuns {
				t.Logf("Sub DAG run %s with params %s", child.DAGRunID, child.Params)
			}
		}
	})

	t.Run("MixedLocalAndDistributedCancellation", func(t *testing.T) {
		// Create test DAGs with both local and distributed sub DAGs
		yamlContent := `
steps:
  - name: local-execution
    call: child-local
    parallel:
      items: ["3", "5"]
    output: LOCAL_RESULTS
    depends: []
  - name: distributed-execution
    call: child-distributed
    parallel:
      items: ["4", "6"]
    output: DISTRIBUTED_RESULTS
    depends: []

---
name: child-local
steps:
  - name: sleep
    command: sleep $1

---
name: child-distributed
workerSelector:
  type: test-worker
steps:
  - name: sleep
    command: sleep $1
`
		// Setup and start coordinator
		tmpDir := t.TempDir()
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir), test.WithStatusPersistence())

		// Create worker for distributed execution
		setupWorker(t, coord, "test-worker-1", 10, map[string]string{"type": "test-worker"})

		// Load the DAG and create agent
		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()
		done := make(chan struct{})

		// Start the DAG in a goroutine
		go func() {
			agent.Context = coord.Context
			_ = agent.Run(agent.Context)
			close(done)
		}()

		// Wait for at least one step to be running
		require.Eventually(t, func() bool {
			st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil || !st.Status.IsActive() {
				return false
			}
			if len(st.Nodes) == 0 {
				return false
			}
			// Check if any node is running
			var started int
			for _, node := range st.Nodes {
				if node.Status == core.NodeRunning {
					started++
				}
			}
			return started == 2 // Both local and distributed steps should be running
		}, 5*time.Second, 100*time.Millisecond)

		// Perform cancellation
		agent.Signal(coord.Context, os.Signal(syscall.SIGTERM))

		// Wait for the agent to finish
		<-done

		// Get the latest status
		st := agent.Status(coord.Context)

		// Both parallel steps should be affected by cancellation
		for _, node := range st.Nodes {
			if node.Step.Name == "local-execution" || node.Step.Name == "distributed-execution" {
				require.Equal(t, core.NodeAborted, node.Status,
					"node %s should be canceled, got %v", node.Step.Name, node.Status)
			}
		}
	})

	t.Run("ConcurrentWorkerCancellation", func(t *testing.T) {
		// Test cancellation with high concurrency across multiple workers
		yamlContent := `
steps:
  - name: high-concurrency
    call: child-task
    parallel:
      items:
        - "task1"
        - "task2"
        - "task3"
        - "task4"
        - "task5"
        - "task6"
      maxConcurrent: 2

---
name: child-task
workerSelector:
  type: test-worker
steps:
  - name: process
    command: |
      echo "Starting task $1"
      sleep 1
      echo "Completed task $1"
`
		// Setup and start coordinator
		tmpDir := t.TempDir()
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir), test.WithStatusPersistence())

		// Create multiple workers using the helper
		setupWorkers(t, coord, 3, map[string]string{"type": "test-worker"})

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)

		// Create agent
		agent := dagWrapper.Agent()

		// Start the DAG in a goroutine
		done := make(chan struct{})
		go func() {
			agent.Context = coord.Context
			_ = agent.Run(agent.Context)
			close(done)
		}()

		// Wait for the step to be running
		require.Eventually(t, func() bool {
			st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
			if err != nil || !st.Status.IsActive() {
				return false
			}
			if len(st.Nodes) == 0 {
				return false
			}
			concurrentNode := st.Nodes[0]
			return concurrentNode.Status == core.NodeRunning
		}, 5*time.Second, 100*time.Millisecond)

		// Cancel the execution
		agent.Signal(coord.Context, os.Signal(syscall.SIGTERM))

		// Wait for the agent to finish
		<-done

		// Get the latest st
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Verify the high-concurrency step
		require.GreaterOrEqual(t, len(st.Nodes), 1)
		concurrentNode := st.Nodes[0]
		require.Equal(t, "high-concurrency", concurrentNode.Step.Name)

		// Log information about sub runs
		if len(concurrentNode.SubRuns) > 0 {
			t.Logf("Created %d sub runs before cancellation", len(concurrentNode.SubRuns))
			for _, child := range concurrentNode.SubRuns {
				t.Logf("Sub run %s for %s", child.DAGRunID, child.Params)
			}
		}

		require.Equal(t, core.NodePartiallySucceeded, concurrentNode.Status)
	})
}
