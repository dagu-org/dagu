package client_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/dagu-org/dagu/internal/test"
)

func TestClient_GetStatus(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	t.Run("Valid", func(t *testing.T) {
		dag := th.LoadDAGFile(t, "valid.yaml")
		ctx := th.Context

		requestID := fmt.Sprintf("request-id-%d", time.Now().Unix())
		socketServer, _ := sock.NewServer(
			dag.SockAddr(),
			func(w http.ResponseWriter, _ *http.Request) {
				status := model.NewStatusFactory(dag.DAG).Create(
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
	t.Run("InvalidDAGName", func(t *testing.T) {
		ctx := th.Context
		cli := th.Client

		dagStatus, err := cli.GetStatus(ctx, "invalid-dag-name")
		require.Error(t, err)
		require.NotNil(t, dagStatus)

		// Check the status contains error.
		require.Error(t, dagStatus.Error)
	})
	t.Run("UpdateStatus", func(t *testing.T) {
		dag := th.LoadDAGFile(t, "update_status.yaml")

		requestID := "test-update-status"
		now := time.Now()
		ctx := th.Context
		cli := th.Client

		// Open the history store and write a status before updating it.
		err := th.HistoryStore.Open(ctx, dag.Location, now, requestID)
		require.NoError(t, err)

		status := testNewStatus(dag.DAG, requestID, scheduler.StatusSuccess, scheduler.NodeStatusSuccess)

		err = th.HistoryStore.Write(ctx, status)
		require.NoError(t, err)
		_ = th.HistoryStore.Close(ctx)

		// Get the status and check if it is the same as the one we wrote.
		statusToCheck, err := cli.GetStatusByRequestID(ctx, dag.DAG, requestID)
		require.NoError(t, err)
		require.Equal(t, scheduler.NodeStatusSuccess, statusToCheck.Nodes[0].Status)

		// Update the status.
		newStatus := scheduler.NodeStatusError
		status.Nodes[0].Status = newStatus

		err = cli.UpdateStatus(ctx, dag.DAG, status)
		require.NoError(t, err)

		statusByRequestID, err := cli.GetStatusByRequestID(ctx, dag.DAG, requestID)
		require.NoError(t, err)

		require.Equal(t, 1, len(status.Nodes))
		require.Equal(t, newStatus, statusByRequestID.Nodes[0].Status)
	})
	t.Run("InvalidUpdateStatusWithInvalidReqID", func(t *testing.T) {
		wrongReqID := "invalid-request-id"
		dag := th.LoadDAGFile(t, "invalid_reqid.yaml")
		ctx := th.Context
		cli := th.Client

		// update with invalid request id
		status := testNewStatus(dag.DAG, wrongReqID, scheduler.StatusError,
			scheduler.NodeStatusError)

		// Check if the update fails.
		err := cli.UpdateStatus(ctx, dag.DAG, status)
		require.Error(t, err)
	})
}

func TestClient_RunDAG(t *testing.T) {
	th := test.Setup(t)

	t.Run("RunDAG", func(t *testing.T) {
		dag := th.LoadDAGFile(t, "run_dag.yaml")
		dagStatus, err := th.Client.GetStatus(th.Context, dag.Location)
		require.NoError(t, err)

		err = th.Client.Start(th.Context, dagStatus.DAG, client.StartOptions{})
		require.NoError(t, err)

		status, err := th.Client.GetLatestStatus(th.Context, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		dag := th.LoadDAGFile(t, "stop.yaml")
		ctx := th.Context

		th.Client.StartAsync(ctx, dag.DAG, client.StartOptions{})

		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		err := th.Client.Stop(ctx, dag.DAG)
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
	t.Run("Restart", func(t *testing.T) {
		dag := th.LoadDAGFile(t, "restart.yaml")
		ctx := th.Context

		err := th.Client.Restart(ctx, dag.DAG, client.RestartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
	})
	t.Run("Retry", func(t *testing.T) {
		dag := th.LoadDAGFile(t, "retry.yaml")
		ctx := th.Context
		cli := th.Client

		err := cli.Start(ctx, dag.DAG, client.StartOptions{Params: "x y z"})
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		// Retry the DAG with the same params.
		status, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		previousRequestID := status.RequestID
		previousParams := status.Params

		err = cli.Retry(ctx, dag.DAG, previousRequestID)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, scheduler.StatusSuccess)

		status, err = cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		// Check if the params are the same as the previous run.
		require.NotEqual(t, previousRequestID, status.RequestID)
		require.Equal(t, previousParams, status.Params)
	})
}

func TestClient_UpdateDAG(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	t.Run("Update", func(t *testing.T) {
		ctx := th.Context
		cli := th.Client

		// valid DAG
		validDAG := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
		// Update Error: the DAG does not exist
		err := cli.UpdateDAG(ctx, "non-existing-dag", validDAG)
		require.Error(t, err)

		// create a new DAG file
		id, err := cli.CreateDAG(ctx, "new-dag-file")
		require.NoError(t, err)

		// Update the DAG
		err = cli.UpdateDAG(ctx, id, validDAG)
		require.NoError(t, err)

		// Check the content of the DAG file
		spec, err := cli.GetDAGSpec(ctx, id)
		require.NoError(t, err)
		require.Equal(t, validDAG, spec)
	})
	t.Run("Remove", func(t *testing.T) {
		ctx := th.Context
		cli := th.Client

		spec := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
		id, err := cli.CreateDAG(ctx, "test")
		require.NoError(t, err)
		err = cli.UpdateDAG(ctx, id, spec)
		require.NoError(t, err)

		// check file
		newSpec, err := cli.GetDAGSpec(ctx, id)
		require.NoError(t, err)
		require.Equal(t, spec, newSpec)

		status, _ := cli.GetStatus(ctx, id)

		// delete
		err = cli.DeleteDAG(ctx, id, status.DAG.Location)
		require.NoError(t, err)
	})
	t.Run("Create", func(t *testing.T) {
		ctx := th.Context
		cli := th.Client

		id, err := cli.CreateDAG(ctx, "test-dag")
		require.NoError(t, err)

		// Check if the new DAG is actually created.
		filePath := filepath.Join(th.Config.Paths.DAGsDir, id+".yaml")
		dag, err := digraph.Load(ctx, filePath)
		require.NoError(t, err)
		require.Equal(t, "test-dag", dag.Name)
	})
	t.Run("Rename", func(t *testing.T) {
		ctx := th.Context
		cli := th.Client

		// Create a DAG to rename.
		id, err := cli.CreateDAG(ctx, "old_name")
		require.NoError(t, err)
		_, err = cli.GetStatus(ctx, filepath.Join(th.Config.Paths.DAGsDir, id+".yaml"))
		require.NoError(t, err)

		// Rename the file.
		err = cli.Rename(ctx, id, id+"_renamed")

		// Check if the file is renamed.
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(th.Config.Paths.DAGsDir, id+"_renamed.yaml"))
	})
}

func TestClient_ReadHistory(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	t.Run("TestClient_Empty", func(t *testing.T) {
		ctx := th.Context
		cli := th.Client
		dag := th.LoadDAGFile(t, "empty_status.yaml")

		_, err := cli.GetStatus(ctx, dag.Location)
		require.NoError(t, err)
	})
	t.Run("TestClient_All", func(t *testing.T) {
		ctx := th.Context
		cli := th.Client

		// Create a DAG
		_, err := cli.CreateDAG(ctx, "test-dag1")
		require.NoError(t, err)

		_, err = cli.CreateDAG(ctx, "test-dag2")
		require.NoError(t, err)

		// Get all statuses.
		allDagStatus, _, err := cli.GetAllStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, len(allDagStatus))
	})
}

func testNewStatus(dag *digraph.DAG, requestID string, status scheduler.Status, nodeStatus scheduler.NodeStatus) model.Status {
	nodes := []scheduler.NodeData{{State: scheduler.NodeState{Status: nodeStatus}}}
	startedAt := model.Time(time.Now())
	return model.NewStatusFactory(dag).Create(
		requestID, status, 0, *startedAt, model.WithNodes(nodes),
	)
}

func TestClient_GetTagList(t *testing.T) {
	th := test.Setup(t)

	ctx := th.Context
	cli := th.Client

	// Create DAG List
	for i := 0; i < 40; i++ {
		spec := ""
		id, err := cli.CreateDAG(ctx, "1test-dag-pagination"+fmt.Sprintf("%d", i))
		require.NoError(t, err)
		if i%2 == 0 {
			spec = "tags: tag1,tag2\nsteps:\n  - name: step1\n    command: echo hello\n"
		} else {
			spec = "tags: tag2,tag3\nsteps:\n  - name: step1\n    command: echo hello\n"
		}
		if err = cli.UpdateDAG(ctx, id, spec); err != nil {
			t.Fatal(err)
		}
	}

	tags, errs, err := cli.GetTagList(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(errs))
	require.Equal(t, 3, len(tags))

	mapTags := make(map[string]bool)
	for _, tag := range tags {
		mapTags[tag] = true
	}

	require.True(t, mapTags["tag1"])
	require.True(t, mapTags["tag2"])
	require.True(t, mapTags["tag3"])
}
