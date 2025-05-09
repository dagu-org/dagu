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

func TestRetryChildExecution(t *testing.T) {
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

	workflowID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--workflow-id", workflowID, "parent"}
	th.RunCommand(t, cmd.CmdStart(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"workflow finished"},
	})

	// Update the child_2 status to "failed" to simulate a retry
	// First, find the child_2 workflow ID to update its status
	ctx := context.Background()
	ref := digraph.NewExecRef("parent", workflowID)
	parentRec, err := th.HistoryRepo.Find(ctx, ref)
	require.NoError(t, err)

	updateStatus := func(rec models.Record, status *models.Status) {
		err = rec.Open(ctx)
		require.NoError(t, err)
		err = rec.Write(ctx, *status)
		require.NoError(t, err)
		err = rec.Close(ctx)
		require.NoError(t, err)
	}

	// (1) Find the child_1 node and update its status to "failed"
	parentStatus, err := parentRec.ReadStatus(ctx)
	require.NoError(t, err)

	child1Node := parentStatus.Nodes[0]
	child1Node.Status = scheduler.NodeStatusError
	updateStatus(parentRec, parentStatus)

	// (2) Find the run record for child_1
	child1Rec, err := th.HistoryRepo.FindChildExecution(ctx, ref, child1Node.Children[0].ExecID)
	require.NoError(t, err)

	child1Status, err := child1Rec.ReadStatus(ctx)
	require.NoError(t, err)

	// (3) Find the child_2 node and update its status to "failed"
	child2Node := child1Status.Nodes[0]
	child2Node.Status = scheduler.NodeStatusError
	updateStatus(child1Rec, child1Status)

	// (4) Find the run record for child_2
	child2Rec, err := th.HistoryRepo.FindChildExecution(ctx, ref, child2Node.Children[0].ExecID)
	require.NoError(t, err)

	child2Status, err := child2Rec.ReadStatus(ctx)
	require.NoError(t, err)

	require.Equal(t, child2Status.Status.String(), scheduler.NodeStatusSuccess.String())

	// (5) Update the step in child_2 to "failed" to simulate a retry
	child2Status.Nodes[0].Status = scheduler.NodeStatusError
	updateStatus(child2Rec, child2Status)

	// (6) Check if the child_2 status is now "failed"
	child2Status, err = child2Rec.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, child2Status.Nodes[0].Status.String(), scheduler.NodeStatusError.String())

	// Retry the DAG

	args = []string{"retry", "--workflow-id", workflowID, "parent"}
	th.RunCommand(t, cmd.CmdRetry(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"workflow finished"},
	})

	// Check if the child_2 status is now "success"
	child2Rec, err = th.HistoryRepo.FindChildExecution(ctx, ref, child2Node.Children[0].ExecID)
	require.NoError(t, err)
	child2Status, err = child2Rec.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, child2Status.Nodes[0].Status.String(), scheduler.NodeStatusSuccess.String())
}
