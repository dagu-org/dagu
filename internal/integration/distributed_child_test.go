package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/dagu-org/dagu/internal/worker"
	"github.com/stretchr/testify/require"
)

func TestDistributedLocalDAGExecution(t *testing.T) {
	t.Run("E2ELocalDAGOnWorker", func(t *testing.T) {
		// Create test DAG with local child that uses workerSelector
		yamlContent := `
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

		// Get dispatcher client from coordinator
		coordinatorClient := coord.GetCoordinatorClient(t)

		// Create and start worker with selector labels
		workerInst := worker.NewWorker(
			"test-worker-1",
			10, // maxActiveRuns
			coordinatorClient,
			map[string]string{"type": "test-worker"},
			coord.Config,
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
		time.Sleep(50 * time.Millisecond)

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)

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
steps:
  - name: worker-task
    command: echo "Hello from worker"
    output: MESSAGE
`
		// Setup test environment
		th := test.Setup(t)

		// Test using the helper to create DAG
		dagWrapper := th.DAG(t, localDAGYAML)

		// Verify the DAG was loaded correctly
		require.NotNil(t, dagWrapper.DAG)
	})

	t.Run("DistributedExecutionFailure", func(t *testing.T) {
		// Test that distributed execution failure is not fallback to local execution
		yamlContent := `
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

		// Load the DAG using helper
		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()

		// Run should fail because no worker matches the selector
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
