package dagrun_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
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
	t.Parallel()

	th := test.Setup(t)

	t.Run("Valid", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "valid.yaml"))
		ctx := th.Context

		dagRunID := uuid.Must(uuid.NewV7()).String()
		socketServer, _ := sock.NewServer(
			dag.SockAddr(dagRunID),
			func(w http.ResponseWriter, _ *http.Request) {
				status := models.NewStatusBuilder(dag.DAG).Create(
					dagRunID, status.StatusRunning, 0, time.Now(),
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

		dag.AssertCurrentStatus(t, status.StatusRunning)

		_ = socketServer.Shutdown(ctx)

		dag.AssertCurrentStatus(t, status.StatusNone)
	})
	t.Run("UpdateStatus", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "update_status.yaml"))

		dagRunID := uuid.Must(uuid.NewV7()).String()
		now := time.Now()
		ctx := th.Context
		cli := th.DAGRunMgr

		// Open the Attempt data and write a status before updating it.
		att, err := th.DAGRunStore.CreateAttempt(ctx, dag.DAG, now, dagRunID, models.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		err = att.Open(ctx)
		require.NoError(t, err)

		dagRunStatus := testNewStatus(dag.DAG, dagRunID, status.StatusSuccess, status.NodeStatusSuccess)

		err = att.Write(ctx, dagRunStatus)
		require.NoError(t, err)
		_ = att.Close(ctx)

		// Get the status and check if it is the same as the one we wrote.
		ref := digraph.NewDAGRunRef(dag.Name, dagRunID)
		statusToCheck, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, status.NodeStatusSuccess, statusToCheck.Nodes[0].Status)

		// Update the status.
		newStatus := status.NodeStatusError
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
		dag := th.DAG(t, filepath.Join("client", "tree_parent.yaml"))

		err := th.DAGRunMgr.StartDAGRun(th.Context, dag.DAG, dagrun.StartOptions{Quiet: true})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.StatusSuccess)

		// Get the child dag-run status.
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		dagRunID := dagRunStatus.DAGRunID
		childDAGRun := dagRunStatus.Nodes[0].Children[0]

		root := digraph.NewDAGRunRef(dag.Name, dagRunID)
		childDAGRunStatus, err := th.DAGRunMgr.FindChildDAGRunStatus(th.Context, root, childDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, status.StatusSuccess.String(), childDAGRunStatus.Status.String())

		// Update the the child dag-run status.
		childDAGRunStatus.Nodes[0].Status = status.NodeStatusError
		err = th.DAGRunMgr.UpdateStatus(th.Context, root, *childDAGRunStatus)
		require.NoError(t, err)

		// Check if the child dag-run status is updated.
		childDAGRunStatus, err = th.DAGRunMgr.FindChildDAGRunStatus(th.Context, root, childDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, status.NodeStatusError.String(), childDAGRunStatus.Nodes[0].Status.String())
	})
	t.Run("InvalidUpdateStatusWithInvalidDAGRunID", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "invalid_run_id.yaml"))
		ctx := th.Context
		cli := th.DAGRunMgr

		// update with invalid dag-run ID.
		status := testNewStatus(dag.DAG, "unknown-req-id", status.StatusError, status.NodeStatusError)

		// Check if the update fails.
		root := digraph.NewDAGRunRef(dag.Name, "unknown-req-id")
		err := cli.UpdateStatus(ctx, root, status)
		require.Error(t, err)
	})
}

func TestClient_RunDAG(t *testing.T) {
	th := test.Setup(t)

	t.Run("RunDAG", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "run_dag.yaml"))

		err := th.DAGRunMgr.StartDAGRun(th.Context, dag.DAG, dagrun.StartOptions{
			Quiet: true,
		})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.StatusSuccess)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, status.StatusSuccess.String(), dagRunStatus.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "stop.yaml"))
		ctx := th.Context

		err := th.DAGRunMgr.StartDAGRun(ctx, dag.DAG, dagrun.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.StatusRunning)

		err = th.DAGRunMgr.Stop(ctx, dag.DAG, "")
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.StatusCancel)
	})
	t.Run("Restart", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "restart.yaml"))
		ctx := th.Context

		err := th.DAGRunMgr.StartDAGRun(th.Context, dag.DAG, dagrun.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.StatusRunning)

		err = th.DAGRunMgr.RestartDAG(ctx, dag.DAG, dagrun.RestartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.StatusSuccess)
	})
	t.Run("Retry", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "retry.yaml"))
		ctx := th.Context
		cli := th.DAGRunMgr

		err := cli.StartDAGRun(ctx, dag.DAG, dagrun.StartOptions{Params: "x y z"})
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.StatusSuccess)

		// Retry the DAG with the same params.
		dagRunStatus, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		prevDAGRunID := dagRunStatus.DAGRunID
		prevParams := dagRunStatus.Params

		time.Sleep(1 * time.Second)

		err = cli.RetryDAGRun(ctx, dag.DAG, prevDAGRunID)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.StatusSuccess)

		dagRunStatus, err = cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		// Check if the params are the same as the previous run.
		require.Equal(t, prevDAGRunID, dagRunStatus.DAGRunID)
		require.Equal(t, prevParams, dagRunStatus.Params)
	})
	t.Run("RetryStep", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "retry.yaml"))
		ctx := th.Context
		cli := th.DAGRunMgr

		err := cli.StartDAGRun(ctx, dag.DAG, dagrun.StartOptions{})
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.StatusSuccess)

		dagRunStatus, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		dagRunID := dagRunStatus.DAGRunID
		prevParams := dagRunStatus.Params

		time.Sleep(1 * time.Second)

		err = cli.RetryDAGStep(ctx, dag.DAG, dagRunID, "2")
		require.NoError(t, err)

		// Wait for the DAG to finish again
		dag.AssertLatestStatus(t, status.StatusSuccess)

		dagRunStatus, err = cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		// Check if the params are the same as the previous run.
		require.Equal(t, dagRunID, dagRunStatus.DAGRunID)
		require.Equal(t, prevParams, dagRunStatus.Params)
	})
}

func testNewStatus(dag *digraph.DAG, dagRunID string, dagStatus status.Status, nodeStatus status.NodeStatus) models.DAGRunStatus {
	nodes := []scheduler.NodeData{{State: scheduler.NodeState{Status: nodeStatus}}}
	tm := time.Now()
	startedAt := &tm
	return models.NewStatusBuilder(dag).Create(dagRunID, dagStatus, 0, *startedAt, models.WithNodes(nodes))
}
