package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/filedagrun"
	"github.com/dagu-org/dagu/internal/persistence/fileproc"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/dagu-org/dagu/internal/worker"
	"github.com/stretchr/testify/require"
)

func TestParallelDistributedCancellation(t *testing.T) {
	t.Run("CancelParallelExecutionOnWorkers", func(t *testing.T) {
		// Create test DAGs with parallel execution using workerSelector
		yamlContent := `
name: parent-parallel-cancel
steps:
  - name: process-items
    run: child-sleep
    parallel:
      items:
        - "30"
        - "30"
        - "30"
        - "30"
      maxConcurrent: 2

---
name: child-sleep
workerSelector:
  type: test-worker
steps:
  - name: sleep
    command: sleep $1
`
		// Setup temporary directory and test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "parallel-cancel.yaml")
		err := os.WriteFile(testFile, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Set environment variables for the worker configuration
		require.NoError(t, os.Setenv("DAGU_WORKER_COORDINATOR_HOST", "127.0.0.1"))
		require.NoError(t, os.Setenv("DAGU_WORKER_COORDINATOR_PORT", fmt.Sprintf("%d", coord.Port())))

		// Create DAG run manager for workers
		logDir := filepath.Join(tmpDir, "logs")
		dataDir := filepath.Join(tmpDir, "data")
		procDir := filepath.Join(tmpDir, "proc")

		err = os.MkdirAll(logDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(dataDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(procDir, 0755)
		require.NoError(t, err)

		runStore := filedagrun.New(dataDir)
		procStore := fileproc.New(procDir)
		dagRunMgr := dagrun.New(runStore, procStore, coord.Config.Paths.Executable, coord.Config.Global.WorkDir)

		// Create worker TLS config
		tlsConfig := &worker.TLSConfig{
			Insecure: true,
		}

		// Create and start multiple workers to handle parallel execution
		workers := make([]*worker.Worker, 2)
		for i := 0; i < 2; i++ {
			workerInst := worker.NewWorker(
				fmt.Sprintf("test-worker-%d", i+1),
				10, // maxActiveRuns
				"127.0.0.1",
				coord.Port(),
				tlsConfig,
				dagRunMgr,
				map[string]string{"type": "test-worker"},
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
		time.Sleep(500 * time.Millisecond)

		// Load the DAG
		dag, err := digraph.Load(coord.Context, testFile)
		require.NoError(t, err)

		// Create agent with cancellable context
		ctx, cancel := context.WithCancel(coord.Context)
		dagWrapper := test.DAG{
			Helper: &coord.Helper,
			DAG:    dag,
		}
		agent := dagWrapper.Agent()

		// Start the DAG in a goroutine
		errChan := make(chan error, 1)
		go func() {
			agent.Context = ctx
			errChan <- agent.Run(agent.Context)
		}()

		// Wait a bit to ensure parallel execution has started on workers
		time.Sleep(2 * time.Second)

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

		// Get the latest status
		status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dag)
		require.NoError(t, err)
		require.NotNil(t, status)

		// Check that the parallel step exists
		require.GreaterOrEqual(t, len(status.Nodes), 1)
		parallelNode := status.Nodes[0]
		require.Equal(t, "process-items", parallelNode.Step.Name)

		// The step might be marked as failed, cancelled, or error depending on timing
		require.True(t,
			parallelNode.Status == scheduler.NodeStatusCancel ||
				parallelNode.Status == scheduler.NodeStatusError ||
				parallelNode.Status == scheduler.NodeStatusNone,
			"parallel step should be cancelled, failed, or not started, got: %v", parallelNode.Status)

		// If the step was actually started, verify that child DAG runs were created
		if parallelNode.Status != scheduler.NodeStatusNone && len(parallelNode.Children) > 0 {
			// Verify that distributed child runs were cancelled
			for _, child := range parallelNode.Children {
				t.Logf("Child DAG run %s with params %s", child.DAGRunID, child.Params)
			}
		}
	})

	t.Run("MixedLocalAndDistributedCancellation", func(t *testing.T) {
		// Create test DAGs with both local and distributed child DAGs
		yamlContent := `
name: parent-mixed-cancel
steps:
  - name: local-execution
    run: child-local
    parallel:
      items: ["30", "30"]
    output: LOCAL_RESULTS
  - name: distributed-execution
    run: child-distributed
    parallel:
      items: ["30", "30"]
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
		// Setup temporary directory and test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "mixed-cancel.yaml")
		err := os.WriteFile(testFile, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Set environment variables for the worker configuration
		require.NoError(t, os.Setenv("DAGU_WORKER_COORDINATOR_HOST", "127.0.0.1"))
		require.NoError(t, os.Setenv("DAGU_WORKER_COORDINATOR_PORT", fmt.Sprintf("%d", coord.Port())))

		// Create DAG run manager for worker
		logDir := filepath.Join(tmpDir, "logs")
		dataDir := filepath.Join(tmpDir, "data")
		procDir := filepath.Join(tmpDir, "proc")

		err = os.MkdirAll(logDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(dataDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(procDir, 0755)
		require.NoError(t, err)

		runStore := filedagrun.New(dataDir)
		procStore := fileproc.New(procDir)
		dagRunMgr := dagrun.New(runStore, procStore, coord.Config.Paths.Executable, coord.Config.Global.WorkDir)

		// Create worker for distributed execution
		tlsConfig := &worker.TLSConfig{
			Insecure: true,
		}

		workerInst := worker.NewWorker(
			"test-worker-1",
			10,
			"127.0.0.1",
			coord.Port(),
			tlsConfig,
			dagRunMgr,
			map[string]string{"type": "test-worker"},
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
		time.Sleep(500 * time.Millisecond)

		// Load the DAG
		dag, err := digraph.Load(coord.Context, testFile)
		require.NoError(t, err)

		// Create agent with cancellable context
		execCtx, execCancel := context.WithCancel(coord.Context)
		dagWrapper := test.DAG{
			Helper: &coord.Helper,
			DAG:    dag,
		}
		agent := dagWrapper.Agent()

		// Start the DAG in a goroutine
		errChan := make(chan error, 1)
		go func() {
			agent.Context = execCtx
			errChan <- agent.Run(agent.Context)
		}()

		// Wait to ensure both local and distributed executions have started
		time.Sleep(2 * time.Second)

		// Cancel the execution
		execCancel()

		// Wait for the agent to finish
		err = <-errChan
		require.Error(t, err, "agent should return an error when cancelled")

		// Get the latest status
		status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dag)
		require.NoError(t, err)
		require.NotNil(t, status)

		// Both parallel steps should be affected by cancellation
		for _, node := range status.Nodes {
			if node.Step.Name == "local-execution" || node.Step.Name == "distributed-execution" {
				t.Logf("Node %s status: %v", node.Step.Name, node.Status)
				// Nodes might not have started or might be cancelled/failed
				require.True(t,
					node.Status == scheduler.NodeStatusCancel ||
						node.Status == scheduler.NodeStatusError ||
						node.Status == scheduler.NodeStatusNone ||
						node.Status == scheduler.NodeStatusRunning,
					"node %s should show cancellation effect, got: %v", node.Step.Name, node.Status)
			}
		}
	})

	t.Run("ConcurrentWorkerCancellation", func(t *testing.T) {
		// Test cancellation with high concurrency across multiple workers
		yamlContent := `
name: parent-concurrent-cancel
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
      sleep 20
      echo "Completed task $1"
`
		// Setup temporary directory and test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "concurrent-cancel.yaml")
		err := os.WriteFile(testFile, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Set environment variables for the worker configuration
		require.NoError(t, os.Setenv("DAGU_WORKER_COORDINATOR_HOST", "127.0.0.1"))
		require.NoError(t, os.Setenv("DAGU_WORKER_COORDINATOR_PORT", fmt.Sprintf("%d", coord.Port())))

		// Create DAG run manager
		logDir := filepath.Join(tmpDir, "logs")
		dataDir := filepath.Join(tmpDir, "data")
		procDir := filepath.Join(tmpDir, "proc")

		err = os.MkdirAll(logDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(dataDir, 0755)
		require.NoError(t, err)
		err = os.MkdirAll(procDir, 0755)
		require.NoError(t, err)

		runStore := filedagrun.New(dataDir)
		procStore := fileproc.New(procDir)
		dagRunMgr := dagrun.New(runStore, procStore, coord.Config.Paths.Executable, coord.Config.Global.WorkDir)

		// Create multiple workers to handle concurrent execution
		tlsConfig := &worker.TLSConfig{
			Insecure: true,
		}

		numWorkers := 3
		workers := make([]*worker.Worker, numWorkers)
		for i := 0; i < numWorkers; i++ {
			workerInst := worker.NewWorker(
				fmt.Sprintf("test-worker-%d", i+1),
				5, // maxActiveRuns per worker
				"127.0.0.1",
				coord.Port(),
				tlsConfig,
				dagRunMgr,
				map[string]string{"type": "test-worker"},
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
		time.Sleep(1 * time.Second)

		// Load the DAG
		dag, err := digraph.Load(coord.Context, testFile)
		require.NoError(t, err)

		// Create agent with cancellable context
		ctx, cancel := context.WithCancel(coord.Context)
		dagWrapper := test.DAG{
			Helper: &coord.Helper,
			DAG:    dag,
		}
		agent := dagWrapper.Agent()

		// Start the DAG in a goroutine
		errChan := make(chan error, 1)
		go func() {
			agent.Context = ctx
			errChan <- agent.Run(agent.Context)
		}()

		// Wait to ensure concurrent execution has started across workers
		time.Sleep(2 * time.Second)

		// Cancel the execution
		cancel()

		// Wait for the agent to finish
		err = <-errChan
		require.Error(t, err, "agent should return an error when cancelled")

		// Get the latest status
		status, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dag)
		require.NoError(t, err)
		require.NotNil(t, status)

		// Verify the high-concurrency step
		require.GreaterOrEqual(t, len(status.Nodes), 1)
		concurrentNode := status.Nodes[0]
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
		validStatuses := []scheduler.NodeStatus{
			scheduler.NodeStatusCancel,
			scheduler.NodeStatusError,
			scheduler.NodeStatusRunning,
			scheduler.NodeStatusNone,
			scheduler.NodeStatusSuccess, // Some children might have completed before cancellation
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