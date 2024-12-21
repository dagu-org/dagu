// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package client_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/dagu-org/dagu/internal/test"
)

var testdataDir = filepath.Join(fileutil.MustGetwd(), "./testdata")

func TestClient_GetStatus(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		th := test.Setup(t)

		file := testDAG("sleep1.yaml")

		cli := th.Client()
		ctx := context.Background()
		dagStatus, err := cli.GetStatus(ctx, file)
		require.NoError(t, err)

		socketServer, _ := sock.NewServer(
			dagStatus.DAG.SockAddr(),
			func(w http.ResponseWriter, _ *http.Request) {
				status := model.NewStatus(dagStatus.DAG, nil,
					scheduler.StatusRunning, 0, nil, nil)
				w.WriteHeader(http.StatusOK)
				b, _ := status.ToJSON()
				_, _ = w.Write(b)
			},
		)

		go func() {
			_ = socketServer.Serve(ctx, nil)
			_ = socketServer.Shutdown(ctx)
		}()

		time.Sleep(time.Millisecond * 100)
		curStatus, err := cli.GetCurrentStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, curStatus.Status)

		_ = socketServer.Shutdown(ctx)

		curStatus, err = cli.GetCurrentStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusNone, curStatus.Status)
	})
	t.Run("InvalidDAGName", func(t *testing.T) {
		th := test.Setup(t)

		cli := th.Client()

		ctx := context.Background()
		dagStatus, err := cli.GetStatus(ctx, testDAG("invalid_dag"))
		require.Error(t, err)
		require.NotNil(t, dagStatus)

		// Check the status contains error.
		require.Error(t, dagStatus.Error)
	})
	t.Run("UpdateStatus", func(t *testing.T) {
		th := test.Setup(t)

		var (
			file      = testDAG("success.yaml")
			requestID = "test-update-status"
			now       = time.Now()
			cli       = th.Client()
		)
		ctx := context.Background()
		dagStatus, err := cli.GetStatus(ctx, file)
		require.NoError(t, err)

		historyStore := th.DataStore().HistoryStore()

		err = historyStore.Open(ctx, dagStatus.DAG.Location, now, requestID)
		require.NoError(t, err)

		status := testNewStatus(dagStatus.DAG, requestID,
			scheduler.StatusSuccess, scheduler.NodeStatusSuccess)

		err = historyStore.Write(ctx, status)
		require.NoError(t, err)
		_ = historyStore.Close(ctx)

		time.Sleep(time.Millisecond * 100)

		status, err = cli.GetStatusByRequestID(ctx, dagStatus.DAG, requestID)
		require.NoError(t, err)
		require.Equal(t, scheduler.NodeStatusSuccess, status.Nodes[0].Status)

		newStatus := scheduler.NodeStatusError
		status.Nodes[0].Status = newStatus

		err = cli.UpdateStatus(ctx, dagStatus.DAG, status)
		require.NoError(t, err)

		statusByRequestID, err := cli.GetStatusByRequestID(ctx, dagStatus.DAG, requestID)
		require.NoError(t, err)

		require.Equal(t, 1, len(status.Nodes))
		require.Equal(t, newStatus, statusByRequestID.Nodes[0].Status)
	})
	t.Run("InvalidUpdateStatusWithInvalidReqID", func(t *testing.T) {
		th := test.Setup(t)

		var (
			cli        = th.Client()
			file       = testDAG("sleep1.yaml")
			wrongReqID = "invalid-request-id"
		)

		ctx := context.Background()
		dagStatus, err := cli.GetStatus(ctx, file)
		require.NoError(t, err)

		// update with invalid request id
		status := testNewStatus(dagStatus.DAG, wrongReqID, scheduler.StatusError,
			scheduler.NodeStatusError)

		// Check if the update fails.
		err = cli.UpdateStatus(ctx, dagStatus.DAG, status)
		require.Error(t, err)
	})
}

func TestClient_RunDAG(t *testing.T) {
	t.Run("RunDAG", func(t *testing.T) {
		th := test.Setup(t)

		cli := th.Client()
		file := testDAG("success.yaml")
		ctx := context.Background()
		dagStatus, err := cli.GetStatus(ctx, file)
		require.NoError(t, err)

		err = cli.Start(ctx, dagStatus.DAG, client.StartOptions{})
		require.NoError(t, err)

		status, err := cli.GetLatestStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		th := test.Setup(t)

		cli := th.Client()
		file := testDAG("sleep10.yaml")
		ctx := context.Background()
		dagStatus, err := cli.GetStatus(ctx, file)
		require.NoError(t, err)

		cli.StartAsync(ctx, dagStatus.DAG, client.StartOptions{})

		require.Eventually(t, func() bool {
			curStatus, _ := cli.GetCurrentStatus(ctx, dagStatus.DAG)
			return curStatus.Status == scheduler.StatusRunning
		}, time.Millisecond*1500, time.Millisecond*100)

		_ = cli.Stop(ctx, dagStatus.DAG)

		require.Eventually(t, func() bool {
			latestStatus, _ := cli.GetLatestStatus(ctx, dagStatus.DAG)
			return latestStatus.Status == scheduler.StatusCancel
		}, time.Millisecond*1500, time.Millisecond*100)
	})
	t.Run("Restart", func(t *testing.T) {
		th := test.Setup(t)

		cli := th.Client()
		file := testDAG("success.yaml")
		ctx := context.Background()
		dagStatus, err := cli.GetStatus(ctx, file)
		require.NoError(t, err)

		err = cli.Restart(ctx, dagStatus.DAG, client.RestartOptions{})
		require.NoError(t, err)

		status, err := cli.GetLatestStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, status.Status)
	})
	t.Run("Retry", func(t *testing.T) {
		th := test.Setup(t)

		ctx := context.Background()
		cli := th.Client()
		file := testDAG("retry.yaml")

		dagStatus, err := cli.GetStatus(ctx, file)
		require.NoError(t, err)

		err = cli.Start(ctx, dagStatus.DAG, client.StartOptions{Params: "x y z"})
		require.NoError(t, err)

		status, err := cli.GetLatestStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, status.Status)

		requestID := status.RequestID
		params := status.Params

		err = cli.Retry(ctx, dagStatus.DAG, requestID)
		require.NoError(t, err)
		status, err = cli.GetLatestStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)

		require.Equal(t, scheduler.StatusSuccess, status.Status)
		require.Equal(t, params, status.Params)

		statusByRequestID, err := cli.GetStatusByRequestID(ctx, dagStatus.DAG, status.RequestID)
		require.NoError(t, err)
		require.Equal(t, status, statusByRequestID)

		recentStatuses := cli.GetRecentHistory(ctx, dagStatus.DAG, 1)
		require.Equal(t, status, recentStatuses[0].Status)
	})
}

func TestClient_UpdateDAG(t *testing.T) {
	t.Parallel()
	t.Run("Update", func(t *testing.T) {
		th := test.Setup(t)

		cli := th.Client()
		ctx := context.Background()

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
		th := test.Setup(t)

		cli := th.Client()
		ctx := context.Background()

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
		th := test.Setup(t)

		cli := th.Client()
		ctx := context.Background()

		id, err := cli.CreateDAG(ctx, "test-dag")
		require.NoError(t, err)

		// Check if the new DAG is actually created.
		dag, err := digraph.Load(ctx, "", filepath.Join(th.Config.Paths.DAGsDir, id+".yaml"), "")
		require.NoError(t, err)
		require.Equal(t, "test-dag", dag.Name)
	})
	t.Run("Rename", func(t *testing.T) {
		th := test.Setup(t)

		cli := th.Client()
		ctx := context.Background()

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
	t.Run("TestClient_Empty", func(t *testing.T) {
		th := test.Setup(t)

		cli := th.Client()
		file := testDAG("success.yaml")
		ctx := context.Background()

		_, err := cli.GetStatus(ctx, file)
		require.NoError(t, err)
	})
	t.Run("TestClient_All", func(t *testing.T) {
		th := test.Setup(t)

		cli := th.Client()
		ctx := context.Background()

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

func testDAG(name string) string {
	return filepath.Join(testdataDir, name)
}

func testNewStatus(dag *digraph.DAG, requestID string, status scheduler.Status,
	nodeStatus scheduler.NodeStatus) *model.Status {
	ret := model.NewStatus(
		dag,
		[]scheduler.NodeData{
			{
				State: scheduler.NodeState{Status: nodeStatus},
			},
		},
		status,
		0,
		model.Time(time.Now()),
		nil,
	)
	ret.RequestID = requestID
	return ret
}

func TestClient_GetTagList(t *testing.T) {
	th := test.Setup(t)

	cli := th.Client()
	ctx := context.Background()

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
