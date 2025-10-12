package dagrun_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/dagu-org/dagu/internal/test"
)

func TestManager(t *testing.T) {
	th := test.Setup(t)

	t.Run("Valid", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: sleep 1
`)
		ctx := th.Context

		dagRunID := uuid.Must(uuid.NewV7()).String()
		socketServer, _ := sock.NewServer(
			dag.SockAddr(dagRunID),
			func(w http.ResponseWriter, _ *http.Request) {
				status := models.NewStatusBuilder(dag.DAG).Create(
					dagRunID, status.Running, 0, time.Now(),
				)
				w.WriteHeader(http.StatusOK)
				jsonData, err := json.Marshal(status)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write(jsonData)
			},
		)

		go func() {
			_ = socketServer.Serve(ctx, nil)
			_ = socketServer.Shutdown(ctx)
		}()

		dag.AssertCurrentStatus(t, status.Running)

		_ = socketServer.Shutdown(ctx)

		dag.AssertCurrentStatus(t, status.None)
	})
	t.Run("UpdateStatus", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		now := time.Now()
		ctx := th.Context
		cli := th.DAGRunMgr

		// Open the Attempt data and write a status before updating it.
		att, err := th.DAGRunStore.CreateAttempt(ctx, dag.DAG, now, dagRunID, models.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		err = att.Open(ctx)
		require.NoError(t, err)

		dagRunStatus := testNewStatus(dag.DAG, dagRunID, status.Success, status.NodeSuccess)

		err = att.Write(ctx, dagRunStatus)
		require.NoError(t, err)
		_ = att.Close(ctx)

		// Get the status and check if it is the same as the one we wrote.
		ref := digraph.NewDAGRunRef(dag.Name, dagRunID)
		statusToCheck, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, status.NodeSuccess, statusToCheck.Nodes[0].Status)

		// Update the status.
		newStatus := status.NodeError
		dagRunStatus.Nodes[0].Status = newStatus

		root := digraph.NewDAGRunRef(dag.Name, dagRunID)
		err = cli.UpdateStatus(ctx, root, dagRunStatus)
		require.NoError(t, err)

		statusByDAGRunID, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)

		require.Equal(t, 1, len(dagRunStatus.Nodes))
		require.Equal(t, newStatus, statusByDAGRunID.Nodes[0].Status)
	})
	t.Run("UpdateChildDAGRunStatus", func(t *testing.T) {
		dag := th.DAG(t, `
steps:
  - name: "1"
    run: tree_child
---
name: tree_child
steps:
  - name: "1"
    command: "true"
---
`)

		spec := th.SubCmdBuilder.Start(dag.DAG, dagrun.StartOptions{})
		err := dagrun.Start(th.Context, spec)
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.Success)

		// Get the child dag-run status.
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		dagRunID := dagRunStatus.DAGRunID
		childDAGRun := dagRunStatus.Nodes[0].Children[0]

		root := digraph.NewDAGRunRef(dag.Name, dagRunID)
		childDAGRunStatus, err := th.DAGRunMgr.FindChildDAGRunStatus(th.Context, root, childDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, status.Success.String(), childDAGRunStatus.Status.String())

		// Update the the child dag-run status.
		childDAGRunStatus.Nodes[0].Status = status.NodeError
		err = th.DAGRunMgr.UpdateStatus(th.Context, root, *childDAGRunStatus)
		require.NoError(t, err)

		// Check if the child dag-run status is updated.
		childDAGRunStatus, err = th.DAGRunMgr.FindChildDAGRunStatus(th.Context, root, childDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, status.NodeError.String(), childDAGRunStatus.Nodes[0].Status.String())
	})
	t.Run("InvalidUpdateStatusWithInvalidDAGRunID", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: sleep 1
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// update with invalid dag-run ID.
		status := testNewStatus(dag.DAG, "unknown-req-id", status.Error, status.NodeError)

		// Check if the update fails.
		root := digraph.NewDAGRunRef(dag.Name, "unknown-req-id")
		err := cli.UpdateStatus(ctx, root, status)
		require.Error(t, err)
	})
}

func testNewStatus(dag *digraph.DAG, dagRunID string, dagStatus status.Status, nodeStatus status.NodeStatus) models.DAGRunStatus {
	nodes := []scheduler.NodeData{{State: scheduler.NodeState{Status: nodeStatus}}}
	tm := time.Now()
	startedAt := &tm
	return models.NewStatusBuilder(dag).Create(dagRunID, dagStatus, 0, *startedAt, models.WithNodes(nodes))
}
