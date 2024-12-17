// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package client_test

import (
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
	"github.com/dagu-org/dagu/internal/util"
)

var testdataDir = filepath.Join(util.MustGetwd(), "./testdata")

func TestClient_GetStatus(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		file := testDAG("sleep1.yaml")

		cli := setup.Client()
		dagStatus, err := cli.GetStatus(file)
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
			test.NewLogger(),
		)

		go func() {
			_ = socketServer.Serve(nil)
			_ = socketServer.Shutdown()
		}()

		time.Sleep(time.Millisecond * 100)
		curStatus, err := cli.GetCurrentStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, curStatus.Status)

		_ = socketServer.Shutdown()

		curStatus, err = cli.GetCurrentStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusNone, curStatus.Status)
	})
	t.Run("InvalidDAGName", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()

		dagStatus, err := cli.GetStatus(testDAG("invalid_dag"))
		require.Error(t, err)
		require.NotNil(t, dagStatus)

		// Check the status contains error.
		require.Error(t, dagStatus.Error)
	})
	t.Run("UpdateStatus", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		var (
			file      = testDAG("success.yaml")
			requestID = "test-update-status"
			now       = time.Now()
			cli       = setup.Client()
		)

		dagStatus, err := cli.GetStatus(file)
		require.NoError(t, err)

		historyStore := setup.DataStore().HistoryStore()

		err = historyStore.Open(dagStatus.DAG.Location, now, requestID)
		require.NoError(t, err)

		status := testNewStatus(dagStatus.DAG, requestID,
			scheduler.StatusSuccess, scheduler.NodeStatusSuccess)

		err = historyStore.Write(status)
		require.NoError(t, err)
		_ = historyStore.Close()

		time.Sleep(time.Millisecond * 100)

		status, err = cli.GetStatusByRequestID(dagStatus.DAG, requestID)
		require.NoError(t, err)
		require.Equal(t, scheduler.NodeStatusSuccess, status.Nodes[0].Status)

		newStatus := scheduler.NodeStatusError
		status.Nodes[0].Status = newStatus

		err = cli.UpdateStatus(dagStatus.DAG, status)
		require.NoError(t, err)

		statusByRequestID, err := cli.GetStatusByRequestID(dagStatus.DAG, requestID)
		require.NoError(t, err)

		require.Equal(t, 1, len(status.Nodes))
		require.Equal(t, newStatus, statusByRequestID.Nodes[0].Status)
	})
	t.Run("InvalidUpdateStatusWithInvalidReqID", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		var (
			cli        = setup.Client()
			file       = testDAG("sleep1.yaml")
			wrongReqID = "invalid-request-id"
		)

		dagStatus, err := cli.GetStatus(file)
		require.NoError(t, err)

		// update with invalid request id
		status := testNewStatus(dagStatus.DAG, wrongReqID, scheduler.StatusError,
			scheduler.NodeStatusError)

		// Check if the update fails.
		err = cli.UpdateStatus(dagStatus.DAG, status)
		require.Error(t, err)
	})
}

func TestClient_RunDAG(t *testing.T) {
	t.Run("RunDAG", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()
		file := testDAG("success.yaml")
		dagStatus, err := cli.GetStatus(file)
		require.NoError(t, err)

		err = cli.Start(dagStatus.DAG, client.StartOptions{})
		require.NoError(t, err)

		status, err := cli.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()
		file := testDAG("sleep10.yaml")
		dagStatus, err := cli.GetStatus(file)
		require.NoError(t, err)

		cli.StartAsync(dagStatus.DAG, client.StartOptions{})

		require.Eventually(t, func() bool {
			curStatus, _ := cli.GetCurrentStatus(dagStatus.DAG)
			return curStatus.Status == scheduler.StatusRunning
		}, time.Millisecond*1500, time.Millisecond*100)

		_ = cli.Stop(dagStatus.DAG)

		require.Eventually(t, func() bool {
			latestStatus, _ := cli.GetLatestStatus(dagStatus.DAG)
			return latestStatus.Status == scheduler.StatusCancel
		}, time.Millisecond*1500, time.Millisecond*100)
	})
	t.Run("Restart", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()
		file := testDAG("success.yaml")
		dagStatus, err := cli.GetStatus(file)
		require.NoError(t, err)

		err = cli.Restart(dagStatus.DAG, client.RestartOptions{})
		require.NoError(t, err)

		status, err := cli.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, status.Status)
	})
	t.Run("Retry", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()
		file := testDAG("retry.yaml")

		dagStatus, err := cli.GetStatus(file)
		require.NoError(t, err)

		err = cli.Start(dagStatus.DAG, client.StartOptions{
			Params: "x y z",
		})
		require.NoError(t, err)

		status, err := cli.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, status.Status)

		requestID := status.RequestID
		params := status.Params

		err = cli.Retry(dagStatus.DAG, requestID)
		require.NoError(t, err)
		status, err = cli.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)

		require.Equal(t, scheduler.StatusSuccess, status.Status)
		require.Equal(t, params, status.Params)

		statusByRequestID, err := cli.GetStatusByRequestID(
			dagStatus.DAG, status.RequestID)
		require.NoError(t, err)
		require.Equal(t, status, statusByRequestID)

		recentStatuses := cli.GetRecentHistory(dagStatus.DAG, 1)
		require.Equal(t, status, recentStatuses[0].Status)
	})
}

func TestClient_UpdateDAG(t *testing.T) {
	t.Parallel()
	t.Run("Update", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()

		// valid DAG
		validDAG := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
		// Update Error: the DAG does not exist
		err := cli.UpdateDAG("non-existing-dag", validDAG)
		require.Error(t, err)

		// create a new DAG file
		id, err := cli.CreateDAG("new-dag-file")
		require.NoError(t, err)

		// Update the DAG
		err = cli.UpdateDAG(id, validDAG)
		require.NoError(t, err)

		// Check the content of the DAG file
		spec, err := cli.GetDAGSpec(id)
		require.NoError(t, err)
		require.Equal(t, validDAG, spec)
	})
	t.Run("Remove", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()

		spec := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
		id, err := cli.CreateDAG("test")
		require.NoError(t, err)
		err = cli.UpdateDAG(id, spec)
		require.NoError(t, err)

		// check file
		newSpec, err := cli.GetDAGSpec(id)
		require.NoError(t, err)
		require.Equal(t, spec, newSpec)

		status, _ := cli.GetStatus(id)

		// delete
		err = cli.DeleteDAG(id, status.DAG.Location)
		require.NoError(t, err)
	})
	t.Run("Create", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()

		id, err := cli.CreateDAG("test-dag")
		require.NoError(t, err)

		// Check if the new DAG is actually created.
		workflow, err := digraph.Load("", filepath.Join(setup.Config.DAGs, id+".yaml"), "")
		require.NoError(t, err)
		require.Equal(t, "test-dag", workflow.Name)
	})
	t.Run("Rename", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()

		// Create a DAG to rename.
		id, err := cli.CreateDAG("old_name")
		require.NoError(t, err)
		_, err = cli.GetStatus(filepath.Join(setup.Config.DAGs, id+".yaml"))
		require.NoError(t, err)

		// Rename the file.
		err = cli.Rename(id, id+"_renamed")

		// Check if the file is renamed.
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(setup.Config.DAGs, id+"_renamed.yaml"))
	})
}

func TestClient_ReadHistory(t *testing.T) {
	t.Run("TestClient_Empty", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()
		file := testDAG("success.yaml")

		_, err := cli.GetStatus(file)
		require.NoError(t, err)
	})
	t.Run("TestClient_All", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()

		// Create a DAG
		_, err := cli.CreateDAG("test-dag1")
		require.NoError(t, err)

		_, err = cli.CreateDAG("test-dag2")
		require.NoError(t, err)

		// Get all statuses.
		allDagStatus, _, err := cli.GetAllStatus()
		require.NoError(t, err)
		require.Equal(t, 2, len(allDagStatus))
	})
}

func testDAG(name string) string {
	return filepath.Join(testdataDir, name)
}

func testNewStatus(workflow *digraph.DAG, requestID string, status scheduler.Status,
	nodeStatus scheduler.NodeStatus) *model.Status {
	ret := model.NewStatus(
		workflow,
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
	setup := test.SetupTest(t)
	defer setup.Cleanup()

	cli := setup.Client()

	// Create DAG List
	for i := 0; i < 40; i++ {
		spec := ""
		id, err := cli.CreateDAG("1test-dag-pagination" + fmt.Sprintf("%d", i))
		require.NoError(t, err)
		if i%2 == 0 {
			spec = "tags: tag1,tag2\nsteps:\n  - name: step1\n    command: echo hello\n"
		} else {
			spec = "tags: tag2,tag3\nsteps:\n  - name: step1\n    command: echo hello\n"
		}
		if err = cli.UpdateDAG(id, spec); err != nil {
			t.Fatal(err)
		}

	}

	tags, errs, err := cli.GetTagList()
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
