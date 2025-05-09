package history_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/dagu-org/dagu/internal/test"
)

func TestManager(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	t.Run("Valid", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "valid.yaml"))
		ctx := th.Context

		reqID := uuid.Must(uuid.NewV7()).String()
		socketServer, _ := sock.NewServer(
			dag.SockAddr(reqID),
			func(w http.ResponseWriter, _ *http.Request) {
				status := models.NewStatusBuilder(dag.DAG).Create(
					reqID, scheduler.StatusRunning, 0, time.Now(),
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

		dag.AssertCurrentStatus(t, scheduler.StatusRunning)

		_ = socketServer.Shutdown(ctx)

		dag.AssertCurrentStatus(t, scheduler.StatusNone)
	})
	t.Run("UpdateStatus", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "update_status.yaml"))

		reqID := uuid.Must(uuid.NewV7()).String()
		now := time.Now()
		ctx := th.Context
		cli := th.HistoryMgr

		// Open the history store and write a status before updating it.
		record, err := th.HistoryRepo.Create(ctx, dag.DAG, now, reqID, models.NewRecordOptions{})
		require.NoError(t, err)

		err = record.Open(ctx)
		require.NoError(t, err)

		status := testNewStatus(dag.DAG, reqID, scheduler.StatusSuccess, scheduler.NodeStatusSuccess)

		err = record.Write(ctx, status)
		require.NoError(t, err)
		_ = record.Close(ctx)

		// Get the status and check if it is the same as the one we wrote.
		ref := digraph.NewWorkflowRef(dag.Name, reqID)
		statusToCheck, err := cli.FindByWorkflowID(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, scheduler.NodeStatusSuccess, statusToCheck.Nodes[0].Status)

		// Update the status.
		newStatus := scheduler.NodeStatusError
		status.Nodes[0].Status = newStatus

		root := digraph.NewWorkflowRef(dag.Name, reqID)
		err = cli.UpdateStatus(ctx, root, status)
		require.NoError(t, err)

		statusByReqID, err := cli.FindByWorkflowID(ctx, ref)
		require.NoError(t, err)

		require.Equal(t, 1, len(status.Nodes))
		require.Equal(t, newStatus, statusByReqID.Nodes[0].Status)
	})
	t.Run("UpdateChildExecStatus", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "tree_parent.yaml"))

		err := th.HistoryMgr.Start(th.Context, dag.DAG, history.StartOptions{Quiet: true})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		// Get the child workflow ID.
		status, err := th.HistoryMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		workflowID := status.ExecID
		childWorkflow := status.Nodes[0].Children[0]

		root := digraph.NewWorkflowRef(dag.Name, workflowID)
		childWorkflowStatus, err := th.HistoryMgr.FindChildExec(th.Context, root, childWorkflow.ExecID)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), childWorkflowStatus.Status.String())

		// Update the the child workflow status.
		childWorkflowStatus.Nodes[0].Status = scheduler.NodeStatusError
		err = th.HistoryMgr.UpdateStatus(th.Context, root, *childWorkflowStatus)
		require.NoError(t, err)

		// Check if the child workflow status is updated.
		childWorkflowStatus, err = th.HistoryMgr.FindChildExec(th.Context, root, childWorkflow.ExecID)
		require.NoError(t, err)
		require.Equal(t, scheduler.NodeStatusError.String(), childWorkflowStatus.Nodes[0].Status.String())
	})
	t.Run("InvalidUpdateStatusWithInvalidReqID", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "invalid_reqid.yaml"))
		ctx := th.Context
		cli := th.HistoryMgr

		// update with invalid workflow ID
		status := testNewStatus(dag.DAG, "unknown-req-id", scheduler.StatusError, scheduler.NodeStatusError)

		// Check if the update fails.
		root := digraph.NewWorkflowRef(dag.Name, "unknown-req-id")
		err := cli.UpdateStatus(ctx, root, status)
		require.Error(t, err)
	})
}

func TestClient_RunDAG(t *testing.T) {
	th := test.Setup(t)

	t.Run("RunDAG", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "run_dag.yaml"))

		err := th.HistoryMgr.Start(th.Context, dag.DAG, history.StartOptions{
			Quiet: true,
		})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		status, err := th.HistoryMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "stop.yaml"))
		ctx := th.Context

		err := th.HistoryMgr.Start(ctx, dag.DAG, history.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		err = th.HistoryMgr.Stop(ctx, dag.DAG, "")
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
	t.Run("Restart", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "restart.yaml"))
		ctx := th.Context

		err := th.HistoryMgr.Start(th.Context, dag.DAG, history.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		err = th.HistoryMgr.Restart(ctx, dag.DAG, history.RestartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
	})
	t.Run("Retry", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "retry.yaml"))
		ctx := th.Context
		cli := th.HistoryMgr

		err := cli.Start(ctx, dag.DAG, history.StartOptions{Params: "x y z"})
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		// Retry the DAG with the same params.
		status, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		prevExecID := status.ExecID
		prevParams := status.Params

		time.Sleep(1 * time.Second)

		err = cli.Retry(ctx, dag.DAG, prevExecID)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		status, err = cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		// Check if the params are the same as the previous run.
		require.Equal(t, prevExecID, status.ExecID)
		require.Equal(t, prevParams, status.Params)
	})
}

func testNewStatus(dag *digraph.DAG, reqID string, status scheduler.Status, nodeStatus scheduler.NodeStatus) models.Status {
	nodes := []scheduler.NodeData{{State: scheduler.NodeState{Status: nodeStatus}}}
	tm := time.Now()
	startedAt := &tm
	return models.NewStatusBuilder(dag).Create(
		reqID, status, 0, *startedAt, models.WithNodes(nodes),
	)
}
