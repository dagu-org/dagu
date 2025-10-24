package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDistributedLocalDAGExecution(t *testing.T) {
	t.Run("LocalFailurePropagatesToParentStep", func(t *testing.T) {
		yamlContent := `
steps:
  - name: run-local-on-worker
    call: local-child

---
name: local-child
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: |
      echo "Start task $1"
      exit 1
`
		coord := test.SetupCoordinator(t)

		coordinatorClient := coord.GetCoordinatorClient(t)

		workerInst := worker.NewWorker(
			"test-worker-1",
			10,
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
			if err := workerInst.Stop(coord.Context); err != nil {
				t.Logf("Error stopping worker: %v", err)
			}
		})

		time.Sleep(50 * time.Millisecond)

		dagWrapper := coord.DAG(t, yamlContent)
		agent := dagWrapper.Agent()

		err := agent.Run(agent.Context)
		require.Error(t, err)

		dagWrapper.AssertLatestStatus(t, core.Failed)

		st, statusErr := coord.DAGRunMgr.GetLatestStatus(coord.Context, dagWrapper.DAG)
		require.NoError(t, statusErr)
		require.Len(t, st.Nodes, 1)

		node := st.Nodes[0]
		require.Equal(t, "run-local-on-worker", node.Step.Name)
		require.Equal(t, core.NodeFailed, node.Status)
		require.Len(t, node.Children, 1)
	})
	t.Run("LocalDAG", func(t *testing.T) {
		// Create test DAG with local child that uses workerSelector
		yamlContent := `
steps:
  - name: run-local-on-worker
    call: local-child
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
		dagWrapper.AssertLatestStatus(t, core.Succeeded)
	})
	t.Run("DistributedExecutionFailure", func(t *testing.T) {
		// Test that distributed execution failure is not fallback to local execution
		yamlContent := `
steps:
  - name: run-on-nonexistent-worker
    call: local-child
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
		require.NotEqual(t, core.Succeeded, st.Status)
	})
	t.Run("Cancellation", func(t *testing.T) {
		yamlContent := `
steps:
  - name: run-local-on-worker
    call: local-child
    output: RESULT

---
name: local-child
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: sleep 1000
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
		runID := uuid.New().String()
		agent := dagWrapper.Agent(test.WithDAGRunID(runID))

		// Start run in background
		var done = make(chan struct{})
		go func() {
			agent.RunCancel(t)
			close(done)
		}()

		// Wait for the DAG to start running
		dagWrapper.AssertLatestStatus(t, core.Running)

		err := coord.DAGRunMgr.Stop(coord.Context, agent.DAG, runID)
		require.NoError(t, err)

		// Verify the DAG completed successfully
		dagWrapper.AssertLatestStatus(t, core.Canceled)

		// Wait for run to finish
		<-done
	})
}
