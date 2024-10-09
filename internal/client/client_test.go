// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/history"
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
		ctx := context.Background()
		dagStatus, err := cli.GetLatestDAGStatus(ctx, file)
		require.NoError(t, err)

		socketServer, _ := sock.NewServer(
			dagStatus.DAG.SockAddr(),
			func(w http.ResponseWriter, _ *http.Request) {
				status := history.NewStatus(history.NewStatusArgs{
					DAG:    dagStatus.DAG,
					Status: scheduler.StatusRunning,
				})
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
		curStatus, err := cli.GetCurrentStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, curStatus.Status)

		_ = socketServer.Shutdown()

		curStatus, err = cli.GetCurrentStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusNone, curStatus.Status)
	})
	t.Run("InvalidDAGName", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()

		ctx := context.Background()
		dagStatus, err := cli.GetLatestDAGStatus(ctx, testDAG("invalid_dag"))
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

		ctx := context.Background()
		dagStatus, err := cli.GetLatestDAGStatus(ctx, file)
		require.NoError(t, err)

		historyStore := setup.DataStore().HistoryStore()

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
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		var (
			cli        = setup.Client()
			file       = testDAG("sleep1.yaml")
			wrongReqID = "invalid-request-id"
		)

		ctx := context.Background()
		dagStatus, err := cli.GetLatestDAGStatus(ctx, file)
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
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()
		file := testDAG("success.yaml")
		ctx := context.Background()
		dagStatus, err := cli.GetLatestDAGStatus(ctx, file)
		require.NoError(t, err)

		err = cli.Start(ctx, dagStatus.DAG, client.StartOptions{})
		require.NoError(t, err)

		status, err := cli.GetLatestStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		ctx := context.Background()
		cli := setup.Client()
		file := testDAG("sleep10.yaml")
		dagStatus, err := cli.GetLatestDAGStatus(ctx, file)
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
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		ctx := context.Background()
		cli := setup.Client()
		file := testDAG("success.yaml")
		dagStatus, err := cli.GetLatestDAGStatus(ctx, file)
		require.NoError(t, err)

		err = cli.Restart(ctx, dagStatus.DAG, client.RestartOptions{})
		require.NoError(t, err)

		status, err := cli.GetLatestStatus(ctx, dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, status.Status)
	})
	t.Run("Retry", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		cli := setup.Client()
		file := testDAG("retry.yaml")

		ctx := context.Background()
		dagStatus, err := cli.GetLatestDAGStatus(ctx, file)
		require.NoError(t, err)

		err = cli.Start(ctx, dagStatus.DAG, client.StartOptions{
			Params: "x y z",
		})
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

		statusByRequestID, err := cli.GetStatusByRequestID(
			ctx, dagStatus.DAG, status.RequestID)
		require.NoError(t, err)
		require.Equal(t, status, statusByRequestID)

		recentStatuses := cli.ListRecentHistory(ctx, dagStatus.DAG, 1)
		require.Equal(t, status, recentStatuses[0].Status)
	})
}

func testDAG(name string) string {
	return filepath.Join(testdataDir, name)
}

func testNewStatus(dAG *dag.DAG, requestID string, status scheduler.Status,
	nodeStatus scheduler.NodeStatus) *history.Status {
	startTime := time.Now()
	ret := history.NewStatus(
		history.NewStatusArgs{
			DAG: dAG,
			Nodes: []scheduler.NodeData{
				{
					State: scheduler.NodeState{Status: nodeStatus},
				},
			},
			StartedAt: startTime,
			Status:    status,
		},
	)
	ret.RequestID = requestID
	return ret
}

func TestClient_GetTagList(t *testing.T) {
	setup := test.SetupTest(t)
	defer setup.Cleanup()

	cli := setup.Client()
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
		if err = cli.UpdateDAGSpec(ctx, id, spec); err != nil {
			t.Fatal(err)
		}

	}

	tags, errs, err := cli.ListTags(ctx)
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
