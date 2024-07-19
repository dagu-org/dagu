package engine_test

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/dag/scheduler"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/sock"
	"github.com/dagu-dev/dagu/internal/test"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

var testdataDir = filepath.Join(util.MustGetwd(), "./testdata")

func TestEngine_GetStatus(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		file := testDAG("sleep1.yaml")

		eng := setup.Engine()
		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		socketServer, _ := sock.NewServer(
			&sock.Config{
				Addr: dagStatus.DAG.SockAddr(),
				HandlerFunc: func(w http.ResponseWriter, _ *http.Request) {
					status := model.NewStatus(dagStatus.DAG, nil,
						scheduler.StatusRunning, 0, nil, nil)
					w.WriteHeader(http.StatusOK)
					b, _ := status.ToJSON()
					_, _ = w.Write(b)
				},
			})

		go func() {
			_ = socketServer.Serve(nil)
			_ = socketServer.Shutdown()
		}()

		time.Sleep(time.Millisecond * 100)
		curStatus, err := eng.GetCurrentStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, curStatus.Status)

		_ = socketServer.Shutdown()

		curStatus, err = eng.GetCurrentStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusNone, curStatus.Status)
	})
	t.Run("InvalidDAGName", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()

		dagStatus, err := eng.GetStatus(testDAG("invalid_dag"))
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
			eng       = setup.Engine()
		)

		dagStatus, err := eng.GetStatus(file)
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

		status, err = eng.GetStatusByRequestID(dagStatus.DAG, requestID)
		require.NoError(t, err)
		require.Equal(t, scheduler.NodeStatusSuccess, status.Nodes[0].Status)

		newStatus := scheduler.NodeStatusError
		status.Nodes[0].Status = newStatus

		err = eng.UpdateStatus(dagStatus.DAG, status)
		require.NoError(t, err)

		statusByRequestID, err := eng.GetStatusByRequestID(dagStatus.DAG, requestID)
		require.NoError(t, err)

		require.Equal(t, 1, len(status.Nodes))
		require.Equal(t, newStatus, statusByRequestID.Nodes[0].Status)
	})
	t.Run("InvalidUpdateStatusWithInvalidReqID", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		var (
			eng        = setup.Engine()
			file       = testDAG("sleep1.yaml")
			wrongReqID = "invalid-request-id"
		)

		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		// update with invalid request id
		status := testNewStatus(dagStatus.DAG, wrongReqID, scheduler.StatusError,
			scheduler.NodeStatusError)

		// Check if the update fails.
		err = eng.UpdateStatus(dagStatus.DAG, status)
		require.Error(t, err)
	})
}

// nolint // paralleltest
func TestEngine_RunDAG(t *testing.T) {
	t.Run("RunDAG", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()
		file := testDAG("success.yaml")
		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		err = eng.Start(dagStatus.DAG, engine.StartOptions{})
		require.NoError(t, err)

		status, err := eng.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()
		file := testDAG("sleep10.yaml")
		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		eng.StartAsync(dagStatus.DAG, engine.StartOptions{})

		require.Eventually(t, func() bool {
			curStatus, _ := eng.GetCurrentStatus(dagStatus.DAG)
			return curStatus.Status == scheduler.StatusRunning
		}, time.Millisecond*1500, time.Millisecond*100)

		_ = eng.Stop(dagStatus.DAG)

		require.Eventually(t, func() bool {
			latestStatus, _ := eng.GetLatestStatus(dagStatus.DAG)
			return latestStatus.Status == scheduler.StatusCancel
		}, time.Millisecond*1500, time.Millisecond*100)
	})
	t.Run("Restart", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()
		file := testDAG("success.yaml")
		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		err = eng.Restart(dagStatus.DAG)
		require.NoError(t, err)

		status, err := eng.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, status.Status)
	})
	t.Run("Retry", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()
		file := testDAG("retry.yaml")

		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		err = eng.Start(dagStatus.DAG, engine.StartOptions{
			Params: "x y z",
		})
		require.NoError(t, err)

		status, err := eng.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, status.Status)

		requestID := status.RequestID
		params := status.Params

		err = eng.Retry(dagStatus.DAG, requestID)
		require.NoError(t, err)
		status, err = eng.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)

		require.Equal(t, scheduler.StatusSuccess, status.Status)
		require.Equal(t, params, status.Params)

		statusByRequestID, err := eng.GetStatusByRequestID(
			dagStatus.DAG, status.RequestID)
		require.NoError(t, err)
		require.Equal(t, status, statusByRequestID)

		recentStatuses := eng.GetRecentHistory(dagStatus.DAG, 1)
		require.Equal(t, status, recentStatuses[0].Status)
	})
}

func TestEngine_UpdateDAG(t *testing.T) {
	t.Parallel()
	t.Run("Update", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()

		// valid DAG
		validDAG := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
		// Update Error: the DAG does not exist
		err := eng.UpdateDAG("non-existing-dag", validDAG)
		require.Error(t, err)

		// create a new DAG file
		id, err := eng.CreateDAG("new-dag-file")
		require.NoError(t, err)

		// Update the DAG
		err = eng.UpdateDAG(id, validDAG)
		require.NoError(t, err)

		// Check the content of the DAG file
		spec, err := eng.GetDAGSpec(id)
		require.NoError(t, err)
		require.Equal(t, validDAG, spec)
	})
	t.Run("Remove", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()

		spec := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
		id, err := eng.CreateDAG("test")
		require.NoError(t, err)
		err = eng.UpdateDAG(id, spec)
		require.NoError(t, err)

		// check file
		newSpec, err := eng.GetDAGSpec(id)
		require.NoError(t, err)
		require.Equal(t, spec, newSpec)

		status, _ := eng.GetStatus(id)

		// delete
		err = eng.DeleteDAG(id, status.DAG.Location)
		require.NoError(t, err)
	})
	t.Run("Create", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()

		id, err := eng.CreateDAG("test-dag")
		require.NoError(t, err)

		// Check if the new DAG is actually created.
		dg, err := dag.Load("",
			filepath.Join(setup.Config.DAGs, id+".yaml"), "")
		require.NoError(t, err)
		require.Equal(t, "test-dag", dg.Name)
	})
	t.Run("Rename", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()

		// Create a DAG to rename.
		id, err := eng.CreateDAG("old_name")
		require.NoError(t, err)
		_, err = eng.GetStatus(filepath.Join(setup.Config.DAGs, id+".yaml"))
		require.NoError(t, err)

		// Rename the file.
		err = eng.Rename(id, id+"_renamed")

		// Check if the file is renamed.
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(setup.Config.DAGs, id+"_renamed.yaml"))
	})
}

func TestEngine_ReadHistory(t *testing.T) {
	t.Run("TestEngine_Empty", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()
		file := testDAG("success.yaml")

		_, err := eng.GetStatus(file)
		require.NoError(t, err)
	})
	t.Run("TestEngine_All", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		eng := setup.Engine()

		// Create a DAG
		_, err := eng.CreateDAG("test-dag1")
		require.NoError(t, err)

		_, err = eng.CreateDAG("test-dag2")
		require.NoError(t, err)

		// Get all statuses.
		allDagStatus, _, err := eng.GetAllStatus()
		require.NoError(t, err)
		require.Equal(t, 2, len(allDagStatus))
	})
}

func testDAG(name string) string {
	return filepath.Join(testdataDir, name)
}

func testNewStatus(dg *dag.DAG, reqID string, status scheduler.Status,
	nodeStatus scheduler.NodeStatus) *model.Status {
	ret := model.NewStatus(
		dg,
		[]scheduler.NodeData{
			{
				NodeState: scheduler.NodeState{Status: nodeStatus},
			},
		},
		status,
		0,
		model.Time(time.Now()),
		nil,
	)
	ret.RequestID = reqID
	return ret
}
