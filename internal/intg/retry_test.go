package intg_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestRetryDAGAfterManualStatusUpdate reproduces a bug where retrying a DAG
// after manually updating a node status to failure does not properly retry
// the failed node. The expected behavior is that the retry should re-execute
// the failed node, but the bug causes it to skip or fail incorrectly.
func TestRetryDAGAfterManualStatusUpdate(t *testing.T) {
	th := test.SetupCommand(t)

	// Create a simple 3-step DAG where all steps should succeed
	th.CreateDAGFile(t, "test_retry.yaml", `steps:
  - name: step1
    command: echo "step 1"
    output: OUT1

  - name: step2
    command: echo "step 2"
    output: OUT2
    depends:
      - step1

  - name: step3
    command: echo "step 3"
    output: OUT3
    depends:
      - step2
`)

	// First run - should succeed completely
	dagRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", dagRunID, "test_retry"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG run finished"},
	})

	// Verify the initial run completed successfully
	ctx := context.Background()
	ref := exec.NewDAGRunRef("test_retry", dagRunID)
	attempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	firstRunStatus, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, firstRunStatus.Status, core.Succeeded)

	// Manually update the second node status to failed
	// This simulates a scenario where the user wants to retry from a specific point
	firstRunStatus.Nodes[1].Status = core.NodeFailed

	err = th.DAGRunMgr.UpdateStatus(ctx, ref, *firstRunStatus)
	require.NoError(t, err)

	// Read back the status to verify it was persisted correctly
	readStatus, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeFailed.String(), readStatus.Nodes[1].Status.String(), "step2 should be marked as failed in persisted status")

	// Now retry the DAG using the retry command
	retryArgs := []string{"retry", "--run-id", dagRunID, "test_retry"}
	th.RunCommand(t, cmd.Retry(), test.CmdTest{
		Args:        retryArgs,
		ExpectedOut: []string{"DAG run finished"},
	})

	// The bug: The retry does not succeed even though all commands are valid
	// Expected: The retry should re-execute step2 and step3 successfully
	// Actual: The retry fails or doesn't properly execute the failed nodes

	// Get the status after retry
	retryAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)
	retryStatus, err := retryAttempt.ReadStatus(ctx)
	require.NoError(t, err)

	// This assertion will fail if the bug exists
	// The retry should have succeeded since all commands are valid
	t.Logf("Retry status: %s", retryStatus.Status.String())
	for i, node := range retryStatus.Nodes {
		t.Logf("Node %d (%s): status=%s, error=%s", i, node.Step.Name, node.Status.String(), node.Error)
	}

	// The expected behavior is that the retry succeeds
	require.Equal(t, core.Succeeded.String(), retryStatus.Status.String(), "retry should succeed")

	// Verify that step2 and step3 were re-executed
	require.Equal(t, core.NodeSucceeded.String(), retryStatus.Nodes[1].Status.String(), "step2 should have succeeded after retry")
	require.Equal(t, core.NodeSucceeded.String(), retryStatus.Nodes[2].Status.String(), "step3 should have succeeded after retry")
}
