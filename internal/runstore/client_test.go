package runstore_test

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
	"github.com/dagu-org/dagu/internal/runstore"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/dagu-org/dagu/internal/test"
)

func TestClient_GetStatus(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	t.Run("Valid", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "valid.yaml"))
		ctx := th.Context

		requestID := uuid.Must(uuid.NewV7()).String()
		socketServer, _ := sock.NewServer(
			dag.SockAddr(requestID),
			func(w http.ResponseWriter, _ *http.Request) {
				status := runstore.NewStatusBuilder(dag.DAG).Create(
					requestID, scheduler.StatusRunning, 0, time.Now(),
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

		requestID := uuid.Must(uuid.NewV7()).String()
		now := time.Now()
		ctx := th.Context
		cli := th.RunClient

		// Open the runstore store and write a status before updating it.
		record, err := th.RunStore.NewRecord(ctx, dag.DAG, now, requestID, runstore.NewRecordOptions{})
		require.NoError(t, err)

		err = record.Open(ctx)
		require.NoError(t, err)

		status := testNewStatus(dag.DAG, requestID, scheduler.StatusSuccess, scheduler.NodeStatusSuccess)

		err = record.Write(ctx, status)
		require.NoError(t, err)
		_ = record.Close(ctx)

		// Get the status and check if it is the same as the one we wrote.
		statusToCheck, err := cli.FindByRequestID(ctx, dag.Name, requestID)
		require.NoError(t, err)
		require.Equal(t, scheduler.NodeStatusSuccess, statusToCheck.Nodes[0].Status)

		// Update the status.
		newStatus := scheduler.NodeStatusError
		status.Nodes[0].Status = newStatus

		rootDAG := digraph.NewRootDAG(dag.Name, requestID)
		err = cli.UpdateStatus(ctx, rootDAG, status)
		require.NoError(t, err)

		statusByRequestID, err := cli.FindByRequestID(ctx, dag.Name, requestID)
		require.NoError(t, err)

		require.Equal(t, 1, len(status.Nodes))
		require.Equal(t, newStatus, statusByRequestID.Nodes[0].Status)
	})
	t.Run("UpdateSubRunStatus", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "tree_parent.yaml"))
		dagStatus, err := th.DAGClient.Status(th.Context, dag.Location)
		require.NoError(t, err)

		err = th.RunClient.Start(th.Context, dagStatus.DAG, runstore.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		// Get the sub run status.
		status, err := th.RunClient.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		requestId := status.RequestID
		subRun := status.Nodes[0].SubRuns[0]

		rootDAG := digraph.NewRootDAG(dag.Name, requestId)
		subRunStatus, err := th.RunClient.FindBySubRunRequestID(th.Context, rootDAG, subRun.RequestID)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), subRunStatus.Status.String())

		// Update the sub run status.
		subRunStatus.Nodes[0].Status = scheduler.NodeStatusError
		err = th.RunClient.UpdateStatus(th.Context, rootDAG, *subRunStatus)
		require.NoError(t, err)

		// Check if the sub run status is updated.
		subRunStatus, err = th.RunClient.FindBySubRunRequestID(th.Context, rootDAG, subRun.RequestID)
		require.NoError(t, err)
		require.Equal(t, scheduler.NodeStatusError.String(), subRunStatus.Nodes[0].Status.String())
	})
	t.Run("InvalidUpdateStatusWithInvalidReqID", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "invalid_reqid.yaml"))
		ctx := th.Context
		cli := th.RunClient

		// update with invalid request id
		status := testNewStatus(dag.DAG, "unknown-req-id", scheduler.StatusError, scheduler.NodeStatusError)

		// Check if the update fails.
		rootDAG := digraph.NewRootDAG(dag.Name, "unknown-req-id")
		err := cli.UpdateStatus(ctx, rootDAG, status)
		require.Error(t, err)
	})
}

func TestClient_RunDAG(t *testing.T) {
	th := test.Setup(t)

	t.Run("RunDAG", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "run_dag.yaml"))
		dagStatus, err := th.DAGClient.Status(th.Context, dag.Location)
		require.NoError(t, err)

		err = th.RunClient.Start(th.Context, dagStatus.DAG, runstore.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		status, err := th.RunClient.GetLatestStatus(th.Context, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "stop.yaml"))
		ctx := th.Context

		err := th.RunClient.Start(ctx, dag.DAG, runstore.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		err = th.RunClient.Stop(ctx, dag.DAG, "")
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
	t.Run("Restart", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "restart.yaml"))
		ctx := th.Context

		err := th.RunClient.Start(th.Context, dag.DAG, runstore.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		err = th.RunClient.Restart(ctx, dag.DAG, runstore.RestartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
	})
	t.Run("Retry", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("client", "retry.yaml"))
		ctx := th.Context
		cli := th.RunClient

		err := cli.Start(ctx, dag.DAG, runstore.StartOptions{Params: "x y z"})
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		// Retry the DAG with the same params.
		status, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		previousRequestID := status.RequestID
		previousParams := status.Params

		time.Sleep(1 * time.Second)

		err = cli.Retry(ctx, dag.DAG, previousRequestID)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		status, err = cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		// Check if the params are the same as the previous run.
		require.Equal(t, previousRequestID, status.RequestID)
		require.Equal(t, previousParams, status.Params)
	})
}

func testNewStatus(dag *digraph.DAG, requestID string, status scheduler.Status, nodeStatus scheduler.NodeStatus) runstore.Status {
	nodes := []scheduler.NodeData{{State: scheduler.NodeState{Status: nodeStatus}}}
	tm := time.Now()
	startedAt := &tm
	return runstore.NewStatusBuilder(dag).Create(
		requestID, status, 0, *startedAt, runstore.WithNodes(nodes),
	)
}
