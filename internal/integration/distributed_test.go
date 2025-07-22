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

func TestDistributedLocalDAGExecution(t *testing.T) {
	t.Run("E2E_LocalDAGOnWorker", func(t *testing.T) {
		// Create test DAG with local child that uses workerSelector
		yamlContent := `
name: parent-distributed
steps:
  - name: run-local-on-worker
    run: local-child
    output: RESULT

---
name: local-child
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: echo "Hello from worker"
    output: MESSAGE
`
		// Setup temporary directory and test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "distributed-local.yaml")
		err := os.WriteFile(testFile, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Set environment variables for the worker configuration
		// This is needed because the agent loads its own config
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

		// Create and start worker with selector labels
		workerInst := worker.NewWorker(
			"test-worker-1",
			10, // maxActiveRuns
			"127.0.0.1",
			coord.Port(),
			tlsConfig,
			dagRunMgr,
			map[string]string{"type": "test-worker"},
		)

		ctx, cancel := context.WithCancel(coord.Context)
		defer cancel()

		go func() {
			if err := workerInst.Start(ctx); err != nil {
				t.Logf("Worker stopped: %v", err)
			}
		}()
		t.Cleanup(func() {
			if err := workerInst.Stop(ctx); err != nil {
				t.Logf("Error stopping worker: %v", err)
			}
		})

		// Give worker time to connect
		time.Sleep(500 * time.Millisecond)

		// Load the DAG directly
		dag, err := digraph.Load(coord.Context, testFile)
		require.NoError(t, err)

		// Create agent using the loaded DAG with proper config that includes coordinator settings
		dagWrapper := test.DAG{
			Helper: &coord.Helper,
			DAG:    dag,
		}

		// The coordinator configuration is already set in coord.Config
		// from SetupCoordinator which sets CoordinatorHost and CoordinatorPort
		agent := dagWrapper.Agent()

		// Run the DAG
		agent.RunSuccess(t)

		// Verify the DAG completed successfully
		dagWrapper.AssertLatestStatus(t, status.Success)
	})

	t.Run("TempFileCreationForLocalDAG", func(t *testing.T) {
		// Create a simple local DAG YAML
		localDAGYAML := `
name: local-child
steps:
  - name: worker-task
    command: echo "Hello from worker"
    output: MESSAGE
`
		// Setup test environment
		// Setup test environment
		test.Setup(t)

		// Test creating temp file functionality
		tmpDir := t.TempDir()
		tempFile := filepath.Join(tmpDir, "local-child-test.yaml")
		err := os.WriteFile(tempFile, []byte(localDAGYAML), 0644)
		require.NoError(t, err)

		// Verify the file exists
		_, err = os.Stat(tempFile)
		require.NoError(t, err)

		// Verify the content
		content, err := os.ReadFile(tempFile)
		require.NoError(t, err)
		require.Equal(t, localDAGYAML, string(content))

		// Clean up
		err = os.Remove(tempFile)
		require.NoError(t, err)
	})

	t.Run("DistributedExecutionFailure", func(t *testing.T) {
		// Test that distributed execution failure is not fallback to local execution
		yamlContent := `
name: parent-distributed-fail
steps:
  - name: run-on-nonexistent-worker
    run: local-child
    output: RESULT

---

name: local-child
workerSelector:
  type: nonexistent-worker
steps:
  - name: worker-task
    command: echo "Should not run"
    output: MESSAGE
`
		// Setup temporary directory and test file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "distributed-fail.yaml")
		err := os.WriteFile(testFile, []byte(yamlContent), 0644)
		require.NoError(t, err)

		// Setup coordinator without any matching workers
		coord := test.SetupCoordinator(t, test.WithDAGsDir(tmpDir))

		// Set environment variables for the worker configuration
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

		// Run should fail because no worker matches the selector
		err = agent.Run(coord.Context)
		require.Error(t, err)
		require.Contains(t, err.Error(), "distributed execution failed")

		// Verify the DAG did not complete successfully
		st := agent.Status(coord.Context)
		require.NotEqual(t, status.Success, st.Status)
	})
}
