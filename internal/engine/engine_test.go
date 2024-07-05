package engine_test

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/client"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/sock"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

var testdataDir = path.Join(util.MustGetwd(), "./testdata")
var lock sync.Mutex

func setupTest(t *testing.T) (
	string, engine.Engine, persistence.DataStoreFactory, *config.Config,
) {
	t.Helper()
	lock.Lock()
	defer lock.Unlock()

	tmpDir := util.MustTempDir("dagu_test")
	_ = os.Setenv("HOME", tmpDir)
	cfg, _ := config.Load()

	dataStore := client.NewDataStoreFactory(&client.NewDataStoreFactoryArgs{
		DataDir: path.Join(tmpDir, ".dagu", "data"),
		DAGs:    testdataDir,
		Loader:  dag.NewLoader(),
	})

	exec := path.Join(util.MustGetwd(), "../../bin/dagu")

	return tmpDir,
		engine.New(
			&engine.NewEngineArgs{DataStore: dataStore, Executable: exec},
		),
		dataStore, cfg
}

func setupTestTmpDir(
	t *testing.T,
) (string, engine.Engine, persistence.DataStoreFactory, *config.Config) {
	t.Helper()

	tmpDir := util.MustTempDir("dagu_test")
	_ = os.Setenv("HOME", tmpDir)
	cfg, _ := config.Load()

	dataStore := client.NewDataStoreFactory(&client.NewDataStoreFactoryArgs{
		DataDir: path.Join(tmpDir, ".dagu", "data"),
		DAGs:    path.Join(tmpDir, ".dagu", "dags"),
		Loader:  dag.NewLoader(),
	})

	exec := path.Join(util.MustGetwd(), "../../bin/dagu")

	return tmpDir,
		engine.New(
			&engine.NewEngineArgs{DataStore: dataStore, Executable: exec}),
		dataStore, cfg
}

func TestEngine_GetStatus(t *testing.T) {
	t.Run("Get status of a DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()
		file := testDAG("get_status.yaml")

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
	t.Run("[Invalid] Invalid DAG name", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dagStatus, err := eng.GetStatus(testDAG("invalid_dag"))
		require.Error(t, err)
		require.NotNil(t, dagStatus)

		// Check the status contains error.
		require.Error(t, dagStatus.Error)
	})
	t.Run("Update status", func(t *testing.T) {
		tmpDir, eng, dataStore, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		var (
			file      = testDAG("update_status.yaml")
			requestID = "test-update-status"
			now       = time.Now()
		)

		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		historyStore := dataStore.NewHistoryStore()

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
	t.Run("[Invalid] Update status with invalid request ID", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		var (
			file       = testDAG("update_status_failed.yaml")
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
	t.Run("Start a DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()
		file := testDAG("start.yaml")

		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		err = eng.Start(dagStatus.DAG, "")
		require.Error(t, err)

		status, err := eng.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusError.String(), status.Status.String())
	})
	t.Run("Stop a DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		file := testDAG("stop.yaml")

		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		eng.StartAsync(dagStatus.DAG, "")

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
	t.Run("Restart a DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		file := testDAG("restart.yaml")

		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		err = eng.Restart(dagStatus.DAG)
		require.NoError(t, err)

		status, err := eng.GetLatestStatus(dagStatus.DAG)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, status.Status)
	})
	t.Run("Retry a DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		file := testDAG("retry.yaml")

		dagStatus, err := eng.GetStatus(file)
		require.NoError(t, err)

		err = eng.Start(dagStatus.DAG, "x y z")
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
	t.Run("Update DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTestTmpDir(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

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
	t.Run("Remove a DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTestTmpDir(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

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
	t.Run("Create a new DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTestTmpDir(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		id, err := eng.CreateDAG("test-dag")
		require.NoError(t, err)

		// Check if the new DAG is actually created.
		loader := dag.NewLoader()
		dg, err := loader.Load("",
			path.Join(tmpDir, ".dagu", "dags", id+".yaml"), "")
		require.NoError(t, err)
		require.Equal(t, "test-dag", dg.Name)
	})
	t.Run("Rename a DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTestTmpDir(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		// Create a DAG to rename.
		id, err := eng.CreateDAG("old_name")
		require.NoError(t, err)
		_, err = eng.GetStatus(path.Join(tmpDir, ".dagu", "dags", id+".yaml"))
		require.NoError(t, err)

		// Rename the file.
		err = eng.Rename(id, id+"_renamed")

		// Check if the file is renamed.
		require.NoError(t, err)
		require.FileExists(t, path.Join(tmpDir, ".dagu", "dags", id+"_renamed.yaml"))
	})
}

func TestEngin_ReadHistory(t *testing.T) {
	t.Parallel()
	t.Run("Read empty history", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		file := testDAG("read_status.yaml")

		_, err := eng.GetStatus(file)
		require.NoError(t, err)
	})
	t.Run("Read all history", func(t *testing.T) {
		tmpDir, e, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		allDagStatus, _, err := e.GetAllStatus()
		require.NoError(t, err)
		require.Greater(t, len(allDagStatus), 0)

		pattern := path.Join(testdataDir, "*.yaml")
		matches, err := filepath.Glob(pattern)
		require.NoError(t, err)
		if len(matches) != len(allDagStatus) {
			t.Fatalf("unexpected number of dags: %d", len(allDagStatus))
		}
	})
}

func testDAG(name string) string {
	return path.Join(testdataDir, name)
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
