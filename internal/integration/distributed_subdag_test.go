package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDistributedLocalDAGExecution(t *testing.T) {
	t.Run("LocalFailurePropagatesToParentStep", func(t *testing.T) {
		yamlContent := `
steps:
  - name: run-local-on-worker
    call: local-sub

---
name: local-sub
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: |
      echo "Start task $1"
      exit 1
`
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		setupWorker(t, coord, "test-worker-1", 10, map[string]string{"type": "test-worker"})

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
		require.Len(t, node.SubRuns, 1)
	})
	t.Run("LocalDAG", func(t *testing.T) {
		// Create test DAG with local sub that uses workerSelector
		yamlContent := `
steps:
  - name: run-local-on-worker
    call: local-sub
    output: RESULT

---
name: local-sub
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: echo "Hello from worker"
    output: MESSAGE
`
		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		// Create and start worker with selector labels
		setupWorker(t, coord, "test-worker-1", 10, map[string]string{"type": "test-worker"})

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
    call: local-sub
    output: RESULT

---

name: local-sub
workerSelector:
  type: nonexistent-worker
steps:
  - name: worker-task
    command: echo "Should not run"
    output: MESSAGE
`
		// Setup coordinator without any matching workers
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

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
    call: local-sub
    output: RESULT

---
name: local-sub
workerSelector:
  type: test-worker
steps:
  - name: worker-task
    command: sleep 1000
    output: MESSAGE
`
		// Setup and start coordinator
		coord := test.SetupCoordinator(t, test.WithStatusPersistence())

		// Create and start worker with selector labels
		setupWorker(t, coord, "test-worker-1", 10, map[string]string{"type": "test-worker"})

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
		dagWrapper.AssertLatestStatus(t, core.Aborted)

		// Wait for run to finish
		<-done
	})
}
