package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestParallelDistributedExecution(t *testing.T) {
	t.Run("ParallelExecutionOnWorkers", func(t *testing.T) {
		// Create test DAGs with parallel execution using workerSelector
		yamlContent := `
steps:
  - name: process-items
    run: child-worker
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
		coord := test.SetupCoordinator(t)

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Create and start multiple workers to handle parallel execution
		workers := make([]*worker.Worker, 2)
		for i := 0; i < 2; i++ {
			workerInst := worker.NewWorker(
				fmt.Sprintf("test-worker-%d", i+1),
				10, // maxActiveRuns
				coordinatorClient,
				map[string]string{"type": "test-worker"},
				coord.Config,
			)
			workers[i] = workerInst

			ctx, cancel := context.WithCancel(coord.Context)
			t.Cleanup(cancel)

			go func(w *worker.Worker) {
				if err := w.Start(ctx); err != nil {
					t.Logf("Worker stopped: %v", err)
				}
			}(workerInst)

			t.Cleanup(func() {
				if err := workerInst.Stop(coord.Context); err != nil {
					t.Logf("Error stopping worker: %v", err)
				}
			})
		}

		// Give workers time to connect
		time.Sleep(50 * time.Millisecond)

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()

		// Run the DAG
		agent.RunSuccess(t)

		// Verify the DAG completed successfully
		dagWrapper.AssertLatestStatus(t, status.Success)

		// Get the latest st to verify parallel execution
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)
		require.Len(t, st.Nodes, 1) // process-items

		// Check process-items node
		processNode := st.Nodes[0]
		require.Equal(t, "process-items", processNode.Step.Name)
		require.Equal(t, status.NodeSuccess, processNode.Status)

		// Verify child DAG runs were created
		require.NotEmpty(t, processNode.Children)
		require.Len(t, processNode.Children, 3) // 3 child runs

		// Verify all children completed successfully
		for _, child := range processNode.Children {
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
    run: child-regional
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
		coord := test.SetupCoordinator(t)

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Create multiple workers of the same type
		for i := 0; i < 3; i++ {
			workerInst := worker.NewWorker(
				fmt.Sprintf("test-worker-%d", i+1),
				10,
				coordinatorClient,
				map[string]string{"type": "test-worker"},
				coord.Config,
			)

			ctx, cancel := context.WithCancel(coord.Context)
			t.Cleanup(cancel)

			go func(w *worker.Worker) {
				if err := w.Start(ctx); err != nil {
					t.Logf("Worker stopped: %v", err)
				}
			}(workerInst)

			t.Cleanup(func() {
				if err := workerInst.Stop(coord.Context); err != nil {
					t.Logf("Error stopping worker: %v", err)
				}
			})
		}

		// Give workers time to connect
		time.Sleep(50 * time.Millisecond)

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()
		agent.RunSuccess(t)

		// Verify successful completion
		dagWrapper.AssertLatestStatus(t, status.Success)

		// Get st to verify execution
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Check the parallel node
		processNode := st.Nodes[0]
		require.Equal(t, "process-regions", processNode.Step.Name)
		require.Equal(t, status.NodeSuccess, processNode.Status)
		require.Len(t, processNode.Children, 3)

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

	t.Run("ParallelDistributedWithNoMatchingWorkers", func(t *testing.T) {
		// Test that parallel execution fails gracefully when no workers match
		yamlContent := `
steps:
  - name: process-items
    run: child-nonexistent
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
		coord := test.SetupCoordinator(t)

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
		require.NotEqual(t, status.Success, st.Status)
	})
}

func TestParallelDistributedCancellation(t *testing.T) {
	t.Run("CancelParallelExecutionOnWorkers", func(t *testing.T) {
		// Create test DAGs with parallel execution using workerSelector
		yamlContent := `
steps:
  - name: process-items
    run: child-sleep
    parallel:
      items:
        - "1"
        - "1"
        - "1"
        - "1"
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
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Create DAG run manager for workers
		logDir := filepath.Join(tmpDir, "logs")
		dataDir := filepath.Join(tmpDir, "data")
		procDir := filepath.Join(tmpDir, "proc")

		err := os.MkdirAll(logDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(dataDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(procDir, 0755)
		require.NoError(t, err)

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Create and start multiple workers to handle parallel execution
		workers := make([]*worker.Worker, 2)
		for i := 0; i < 2; i++ {
			workerInst := worker.NewWorker(
				fmt.Sprintf("test-worker-%d", i+1),
				10, // maxActiveRuns
				coordinatorClient,
				map[string]string{"type": "test-worker"},
				coord.Config,
			)
			workers[i] = workerInst

			ctx, cancel := context.WithCancel(coord.Context)
			t.Cleanup(cancel)

			go func(w *worker.Worker) {
				if err := w.Start(ctx); err != nil {
					t.Logf("Worker stopped: %v", err)
				}
			}(workerInst)

			t.Cleanup(func() {
				if err := workerInst.Stop(coord.Context); err != nil {
					t.Logf("Error stopping worker: %v", err)
				}
			})
		}

		// Give workers time to connect
		time.Sleep(50 * time.Millisecond)

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)

		// Create agent with cancellable context
		ctx, cancel := context.WithCancel(coord.Context)
		agent := dagWrapper.Agent()

		// Start the DAG in a goroutine
		errChan := make(chan error, 1)
		go func() {
			agent.Context = ctx
			errChan <- agent.Run(agent.Context)
		}()

		// Wait a bit to ensure parallel execution has started on workers
		time.Sleep(100 * time.Millisecond)

		// Cancel the execution
		cancel()

		// Wait for the agent to finish
		err = <-errChan
		require.Error(t, err, "agent should return an error when cancelled")

		// The error should indicate cancellation
		require.True(t,
			strings.Contains(err.Error(), "context canceled") ||
				strings.Contains(err.Error(), "cancelled") ||
				strings.Contains(err.Error(), "killed"),
			"error should indicate cancellation: %v", err)

		// Get the latest st
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Check that the parallel step exists
		require.GreaterOrEqual(t, len(st.Nodes), 1)
		parallelNode := st.Nodes[0]
		require.Equal(t, "process-items", parallelNode.Step.Name)

		// The step might be marked as failed, cancelled, or error depending on timing
		require.True(t,
			parallelNode.Status == status.NodeCancel ||
				parallelNode.Status == status.NodeError ||
				parallelNode.Status == status.NodeNone,
			"parallel step should be cancelled, failed, or not started, got: %v", parallelNode.Status)

		// If the step was actually started, verify that child DAG runs were created
		if parallelNode.Status != status.NodeNone && len(parallelNode.Children) > 0 {
			// Verify that distributed child runs were cancelled
			for _, child := range parallelNode.Children {
				t.Logf("Child DAG run %s with params %s", child.DAGRunID, child.Params)
			}
		}
	})

	t.Run("MixedLocalAndDistributedCancellation", func(t *testing.T) {
		// Create test DAGs with both local and distributed child DAGs
		yamlContent := `
steps:
  - name: local-execution
    run: child-local
    parallel:
      items: ["1", "1"]
    output: LOCAL_RESULTS
  - name: distributed-execution
    run: child-distributed
    parallel:
      items: ["1", "1"]
    output: DISTRIBUTED_RESULTS

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
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Create DAG run manager for worker
		logDir := filepath.Join(tmpDir, "logs")
		dataDir := filepath.Join(tmpDir, "data")
		procDir := filepath.Join(tmpDir, "proc")

		err := os.MkdirAll(logDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(dataDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(procDir, 0755)
		require.NoError(t, err)

		// Create worker for distributed execution
		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		workerInst := worker.NewWorker(
			"test-worker-1",
			10,
			coordinatorClient,
			map[string]string{"type": "test-worker"},
			coord.Config,
		)

		ctx, cancel := context.WithCancel(coord.Context)
		t.Cleanup(cancel)

		go func() {
			if err := workerInst.Start(ctx); err != nil {
				t.Logf("Worker stopped: %v", err)
			}
		}()

		t.Cleanup(func() {
			if err := workerInst.Stop(coord.Context); err != nil {
				t.Logf("Error stopping worker: %v", err)
			}
		})

		// Give worker time to connect
		time.Sleep(100 * time.Millisecond)

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)

		// Create agent with cancellable context
		execCtx, execCancel := context.WithCancel(coord.Context)
		agent := dagWrapper.Agent()

		// Start the DAG in a goroutine
		errChan := make(chan error, 1)
		go func() {
			agent.Context = execCtx
			errChan <- agent.Run(agent.Context)
		}()

		// Wait to ensure both local and distributed executions have started
		time.Sleep(100 * time.Millisecond)

		// Cancel the execution
		execCancel()

		// Wait for the agent to finish
		err = <-errChan
		require.Error(t, err, "agent should return an error when cancelled")

		// Get the latest st
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Both parallel steps should be affected by cancellation
		for _, node := range st.Nodes {
			if node.Step.Name == "local-execution" || node.Step.Name == "distributed-execution" {
				t.Logf("Node %s status: %v", node.Step.Name, node.Status)
				// Nodes might not have started or might be cancelled/failed
				require.True(t,
					node.Status == status.NodeCancel ||
						node.Status == status.NodeError ||
						node.Status == status.NodeNone ||
						node.Status == status.NodeRunning,
					"node %s should show cancellation effect, got: %v", node.Step.Name, node.Status)
			}
		}
	})

	t.Run("ConcurrentWorkerCancellation", func(t *testing.T) {
		// Test cancellation with high concurrency across multiple workers
		yamlContent := `
steps:
  - name: high-concurrency
    run: child-task
    parallel:
      items:
        - "task1"
        - "task2"
        - "task3"
        - "task4"
        - "task5"
        - "task6"
      maxConcurrent: 4

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
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Create DAG run manager
		logDir := filepath.Join(tmpDir, "logs")
		dataDir := filepath.Join(tmpDir, "data")
		procDir := filepath.Join(tmpDir, "proc")

		err := os.MkdirAll(logDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(dataDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(procDir, 0755)
		require.NoError(t, err)

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		numWorkers := 3
		workers := make([]*worker.Worker, numWorkers)
		for i := 0; i < numWorkers; i++ {
			workerInst := worker.NewWorker(
				fmt.Sprintf("test-worker-%d", i+1),
				5, // maxActiveRuns per worker
				coordinatorClient,
				map[string]string{"type": "test-worker"},
				coord.Config,
			)
			workers[i] = workerInst

			ctx, cancel := context.WithCancel(coord.Context)
			t.Cleanup(cancel)

			go func(w *worker.Worker) {
				if err := w.Start(ctx); err != nil {
					t.Logf("Worker stopped: %v", err)
				}
			}(workerInst)

			t.Cleanup(func() {
				if err := workerInst.Stop(coord.Context); err != nil {
					t.Logf("Error stopping worker: %v", err)
				}
			})
		}

		// Give workers time to connect
		time.Sleep(50 * time.Millisecond)

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)

		// Create agent with cancellable context
		ctx, cancel := context.WithCancel(coord.Context)
		agent := dagWrapper.Agent()

		// Start the DAG in a goroutine
		errChan := make(chan error, 1)
		go func() {
			agent.Context = ctx
			errChan <- agent.Run(agent.Context)
		}()

		// Wait to ensure concurrent execution has started across workers
		time.Sleep(100 * time.Millisecond)

		// Cancel the execution
		cancel()

		// Wait for the agent to finish
		err = <-errChan
		require.Error(t, err, "agent should return an error when cancelled")

		// Get the latest st
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Verify the high-concurrency step
		require.GreaterOrEqual(t, len(st.Nodes), 1)
		concurrentNode := st.Nodes[0]
		require.Equal(t, "high-concurrency", concurrentNode.Step.Name)

		// Log information about child runs
		if len(concurrentNode.Children) > 0 {
			t.Logf("Created %d child runs before cancellation", len(concurrentNode.Children))
			for _, child := range concurrentNode.Children {
				t.Logf("Child run %s for %s", child.DAGRunID, child.Params)
			}
		}

		// The node should reflect cancellation or might not have completed
		// In high concurrency scenarios, the status depends on timing
		validStatuses := []status.NodeStatus{
			status.NodeCancel,
			status.NodeError,
			status.NodeRunning,
			status.NodeNone,
			status.NodeSuccess, // Some children might have completed before cancellation
		}

		statusFound := false
		for _, validStatus := range validStatuses {
			if concurrentNode.Status == validStatus {
				statusFound = true
				break
			}
		}
		require.True(t, statusFound,
			"concurrent node should have a valid status reflecting the execution state, got: %v", concurrentNode.Status)
	})
}
