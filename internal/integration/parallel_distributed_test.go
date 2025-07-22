package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/persistence/filedagrun"
	"github.com/dagu-org/dagu/internal/persistence/fileproc"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/dagu-org/dagu/internal/worker"
	"github.com/stretchr/testify/require"
)

func TestParallelDistributedExecution(t *testing.T) {
	t.Run("ParallelExecutionOnWorkers", func(t *testing.T) {
		// Create test DAGs with parallel execution using workerSelector
		yamlContent := `
name: parent-parallel-distributed
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
		// Setup temporary directory and test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "parallel-distributed.yaml")
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

		// Create agent
		dagWrapper := test.DAG{
			Helper: &coord.Helper,
			DAG:    dag,
		}
		agent := dagWrapper.Agent()

		// Run the DAG
		agent.RunSuccess(t)

		// Verify the DAG completed successfully
		dagWrapper.AssertLatestStatus(t, status.Success)

		// Get the latest st to verify parallel execution
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dag)
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
name: parent-same-workers
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
		// Setup temporary directory and test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "mixed-workers.yaml")
		err := os.WriteFile(testFile, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Set environment variables for worker configuration
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

		// Create workers with same type
		tlsConfig := &worker.TLSConfig{
			Insecure: true,
		}

		// Create multiple workers of the same type
		for i := 0; i < 3; i++ {
			workerInst := worker.NewWorker(
				fmt.Sprintf("test-worker-%d", i+1),
				10,
				"127.0.0.1",
				coord.Port(),
				tlsConfig,
				dagRunMgr,
				map[string]string{"type": "test-worker"},
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
		time.Sleep(500 * time.Millisecond)

		// Load and run the DAG
		dag, err := digraph.Load(coord.Context, testFile)
		require.NoError(t, err)

		dagWrapper := test.DAG{
			Helper: &coord.Helper,
			DAG:    dag,
		}
		agent := dagWrapper.Agent()
		agent.RunSuccess(t)

		// Verify successful completion
		dagWrapper.AssertLatestStatus(t, status.Success)

		// Get st to verify execution
		st, err := coord.DAGRunMgr.GetLatestStatus(coord.Context, dag)
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
name: parent-no-workers
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
		// Setup temporary directory and test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "no-workers.yaml")
		err := os.WriteFile(testFile, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Setup coordinator without matching workers
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Set environment variables
		require.NoError(t, os.Setenv("DAGU_WORKER_COORDINATOR_HOST", "127.0.0.1"))
		require.NoError(t, os.Setenv("DAGU_WORKER_COORDINATOR_PORT", fmt.Sprintf("%d", coord.Port())))

		// Load the DAG
		dag, err := digraph.Load(coord.Context, testFile)
		require.NoError(t, err)

		// Create agent
		dagWrapper := test.DAG{
			Helper: &coord.Helper,
			DAG:    dag,
		}
		agent := dagWrapper.Agent()

		// Run should fail because no workers match
		err = agent.Run(coord.Context)
		require.Error(t, err)

		// Verify the DAG did not complete successfully
		st := agent.Status(coord.Context)
		require.NotEqual(t, status.Success, st.Status)
	})
}
