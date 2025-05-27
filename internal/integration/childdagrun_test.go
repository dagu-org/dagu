package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRetryChildDAGRun(t *testing.T) {
	// Get DAG path
	th := test.SetupCommand(t)

	createDAGFile := func(name, content string) {
		// Create temporary DAG file
		dagFile := filepath.Join(th.Config.Paths.DAGsDir, name)
		// Create the directory if it doesn't exist
		err := os.MkdirAll(filepath.Dir(dagFile), 0750)
		require.NoError(t, err)
		// Write the DAG file
		err = os.WriteFile(dagFile, []byte(content), 0600)
		require.NoError(t, err)
	}

	createDAGFile("parent.yaml", `
steps:
  - name: parent
    run: child_1
    params: "PARAM=FOO"
`)

	createDAGFile("child_1.yaml", `
params: "PARAM=BAR"
steps:
  - name: child_2
    run: child_2
    params: "PARAM=$PARAM"
`)

	createDAGFile("child_2.yaml", `
params: "PARAM=BAZ"
steps:
  - name: child_2
    command: echo "Hello, $PARAM"
`)

	dagRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", dagRunID, "parent"}
	th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG-run finished"},
	})

	// Update the child_2 status to "failed" to simulate a retry
	// First, find the child_2 DAG-run ID to update its status
	ctx := context.Background()
	ref := digraph.NewDAGRunRef("parent", dagRunID)
	parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	updateStatus := func(rec models.DAGRunAttempt, status *models.DAGRunStatus) {
		err = rec.Open(ctx)
		require.NoError(t, err)
		err = rec.Write(ctx, *status)
		require.NoError(t, err)
		err = rec.Close(ctx)
		require.NoError(t, err)
	}

	// (1) Find the child_1 node and update its status to "failed"
	parentStatus, err := parentAttempt.ReadStatus(ctx)
	require.NoError(t, err)

	child1Node := parentStatus.Nodes[0]
	child1Node.Status = scheduler.NodeStatusError
	updateStatus(parentAttempt, parentStatus)

	// (2) Find the child_1 DAG-run ID to update its status
	child1Attempt, err := th.DAGRunStore.FindChildAttempt(ctx, ref, child1Node.Children[0].DAGRunID)
	require.NoError(t, err)

	child1Status, err := child1Attempt.ReadStatus(ctx)
	require.NoError(t, err)

	// (3) Find the child_2 node and update its status to "failed"
	child2Node := child1Status.Nodes[0]
	child2Node.Status = scheduler.NodeStatusError
	updateStatus(child1Attempt, child1Status)

	// (4) Find the child_2 DAG-run ID to update its status
	child2Attempt, err := th.DAGRunStore.FindChildAttempt(ctx, ref, child2Node.Children[0].DAGRunID)
	require.NoError(t, err)

	child2Status, err := child2Attempt.ReadStatus(ctx)
	require.NoError(t, err)

	require.Equal(t, child2Status.Status.String(), scheduler.NodeStatusSuccess.String())

	// (5) Update the step in child_2 to "failed" to simulate a retry
	child2Status.Nodes[0].Status = scheduler.NodeStatusError
	updateStatus(child2Attempt, child2Status)

	// (6) Check if the child_2 status is now "failed"
	child2Status, err = child2Attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, child2Status.Nodes[0].Status.String(), scheduler.NodeStatusError.String())

	// Retry the DAG

	args = []string{"retry", "--run-id", dagRunID, "parent"}
	th.RunCommand(t, cmd.CmdRetry(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"DAG-run finished"},
	})

	// Check if the child_2 status is now "success"
	child2Attempt, err = th.DAGRunStore.FindChildAttempt(ctx, ref, child2Node.Children[0].DAGRunID)
	require.NoError(t, err)
	child2Status, err = child2Attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, child2Status.Nodes[0].Status.String(), scheduler.NodeStatusSuccess.String())

	require.Equal(t, "parent", child2Status.Root.Name, "parent")
	require.Equal(t, dagRunID, child2Status.Root.ID)
}
