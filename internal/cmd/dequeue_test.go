package cmd_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDequeueCommand(t *testing.T) {
	th := test.SetupCommand(t)

	dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)

	// Enqueue the DAG first
	th.RunCommand(t, cmd.Enqueue(), test.CmdTest{
		Name: "Enqueue",
		Args: []string{"enqueue", "--run-id", "test-DAG", dag.Location},
	})

	// Now test the dequeue command
	th.RunCommand(t, cmd.Dequeue(), test.CmdTest{
		Name:        "Dequeue",
		Args:        []string{"dequeue", "--dag-run", dag.Name + ":test-DAG"},
		ExpectedOut: []string{"Dequeued dag-run"},
	})
}

func TestDequeueCommand_PreservesState(t *testing.T) {
	th := test.SetupCommand(t)
	ctx := context.Background()

	// Create a DAG
	dag := th.DAG(t, `steps:
  - name: step1
    command: echo "success"
`)

	// First run the DAG successfully
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Name: "RunDAG",
		Args: []string{"start", "--run-id", "success-run", dag.Location},
	})

	// Wait for it to complete
	attempt, err := th.DAGRunStore.FindAttempt(ctx, execution.DAGRunRef{
		Name: dag.Name,
		ID:   "success-run",
	})
	require.NoError(t, err)

	dagStatus, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, core.Succeeded, dagStatus.Status)

	// Now enqueue a new run
	th.RunCommand(t, cmd.Enqueue(), test.CmdTest{
		Name: "Enqueue",
		Args: []string{"enqueue", "--run-id", "queued-run", dag.Location},
	})

	// Dequeue it
	th.RunCommand(t, cmd.Dequeue(), test.CmdTest{
		Name:        "Dequeue",
		Args:        []string{"dequeue", "--dag-run", dag.Name + ":queued-run"},
		ExpectedOut: []string{"Dequeued dag-run"},
	})

	// Verify the latest visible attempt shows success
	latestAttempt, err := th.DAGRunStore.LatestAttempt(ctx, dag.Name)
	require.NoError(t, err)

	latestStatus, err := latestAttempt.ReadStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, core.Succeeded, latestStatus.Status, "Latest visible status should be Success")
}
