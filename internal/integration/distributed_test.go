package integration_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
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
		// Setup and start coordinator
		coord := test.SetupCoordinator(t)

		// Set environment variables for the worker configuration
		t.Setenv("DAGU_WORKER_COORDINATOR_HOST", "127.0.0.1")
		t.Setenv("DAGU_WORKER_COORDINATOR_PORT", strconv.Itoa(coord.Port()))

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
			coord.DAGRunMgr,
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

		// Load the DAG using helper
		dagWrapper := coord.DAGWithYAML(t, "distributed-local", []byte(yamlContent))

		// Run the DAG
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
		th := test.Setup(t)

		// Test using the helper to create DAG
		dagWrapper := th.DAGWithYAML(t, "local-child-test", []byte(localDAGYAML))

		// Verify the DAG was loaded correctly
		require.NotNil(t, dagWrapper.DAG)
		require.Equal(t, "local-child", dagWrapper.Name)
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
		// Setup coordinator without any matching workers
		coord := test.SetupCoordinator(t)

		// Set environment variables for the worker configuration
		t.Setenv("DAGU_WORKER_COORDINATOR_HOST", "127.0.0.1")
		t.Setenv("DAGU_WORKER_COORDINATOR_PORT", strconv.Itoa(coord.Port()))

		// Load the DAG using helper
		dagWrapper := coord.DAGWithYAML(t, "distributed-fail", []byte(yamlContent))
		agent := dagWrapper.Agent()

		// Run should fail because no worker matches the selector
		err := agent.Run(coord.Context)
		require.Error(t, err)
		require.Contains(t, err.Error(), "distributed execution failed")

		// Verify the DAG did not complete successfully
		st := agent.Status(coord.Context)
		require.NotEqual(t, status.Success, st.Status)
	})
}
