package engine_test

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
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

var (
	testdataDir = path.Join(util.MustGetwd(), "./testdata")
)

// TODO: fix this tests to use mock
func setupTest(t *testing.T) (string, engine.Engine, persistence.DataStoreFactory) {
	t.Helper()

	tmpDir := util.MustTempDir("dagu_test")
	_ = os.Setenv("HOME", tmpDir)
	_ = config.LoadConfig()

	ds := client.NewDataStoreFactory(&config.Config{
		DataDir: path.Join(tmpDir, ".dagu", "data"),
		DAGs:    testdataDir,
	})

	e := engine.NewFactory(ds, &config.Config{
		Executable: path.Join(util.MustGetwd(), "../../bin/dagu"),
	}).Create()

	return tmpDir, e, ds
}

func setupTestTmpDir(t *testing.T) (string, engine.Engine, persistence.DataStoreFactory) {
	t.Helper()

	tmpDir := util.MustTempDir("dagu_test")
	_ = os.Setenv("HOME", tmpDir)
	_ = config.LoadConfig()

	ds := client.NewDataStoreFactory(&config.Config{
		DataDir: path.Join(tmpDir, ".dagu", "data"),
		DAGs:    path.Join(tmpDir, ".dagu", "dags"),
	})

	e := engine.NewFactory(ds, &config.Config{
		Executable: path.Join(util.MustGetwd(), "../../bin/dagu"),
	}).Create()

	return tmpDir, e, ds
}

func TestGetStatusRunningAndDone(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	file := testDAG("get_status.yaml")

	ds, err := e.GetStatus(file)
	require.NoError(t, err)

	socketServer, _ := sock.NewServer(
		&sock.Config{
			Addr: ds.DAG.SockAddr(),
			HandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				status := model.NewStatus(ds.DAG, nil, scheduler.StatusRunning, 0, nil, nil)
				w.WriteHeader(http.StatusOK)
				b, _ := status.ToJson()
				_, _ = w.Write(b)
			},
		})

	go func() {
		_ = socketServer.Serve(nil)
		_ = socketServer.Shutdown()
	}()

	time.Sleep(time.Millisecond * 100)
	st, err := e.GetCurrentStatus(ds.DAG)
	require.NoError(t, err)
	require.Equal(t, scheduler.StatusRunning, st.Status)

	_ = socketServer.Shutdown()

	st, err = e.GetCurrentStatus(ds.DAG)
	require.NoError(t, err)
	require.Equal(t, scheduler.StatusNone, st.Status)
}

func TestUpdateStatus(t *testing.T) {
	tmpDir, e, hf := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	var (
		file      = testDAG("update_status.yaml")
		requestId = "test-update-status"
		now       = time.Now()
	)

	d, err := e.GetStatus(file)
	require.NoError(t, err)

	hs := hf.NewHistoryStore()

	err = hs.Open(d.DAG.Location, now, requestId)
	require.NoError(t, err)

	st := testNewStatus(d.DAG, requestId,
		scheduler.StatusSuccess, scheduler.NodeStatusSuccess)

	err = hs.Write(st)
	require.NoError(t, err)
	_ = hs.Close()

	time.Sleep(time.Millisecond * 100)

	st, err = e.GetStatusByRequestId(d.DAG, requestId)
	require.NoError(t, err)
	require.Equal(t, scheduler.NodeStatusSuccess, st.Nodes[0].Status)

	newStatus := scheduler.NodeStatusError
	st.Nodes[0].Status = newStatus

	err = e.UpdateStatus(d.DAG, st)
	require.NoError(t, err)

	statusByRequestId, err := e.GetStatusByRequestId(d.DAG, requestId)
	require.NoError(t, err)

	require.Equal(t, 1, len(st.Nodes))
	require.Equal(t, newStatus, statusByRequestId.Nodes[0].Status)
}

func TestUpdateStatusError(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	var (
		file      = testDAG("update_status_failed.yaml")
		requestId = "test-update-status-failure"
	)

	d, err := e.GetStatus(file)
	require.NoError(t, err)

	status := testNewStatus(d.DAG, requestId, scheduler.StatusError, scheduler.NodeStatusError)

	err = e.UpdateStatus(d.DAG, status)
	require.Error(t, err)

	// update with invalid request id
	status.RequestId = "invalid-request-id"
	err = e.UpdateStatus(d.DAG, status)
	require.Error(t, err)
}

func TestStart(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	file := testDAG("start.yaml")

	d, err := e.GetStatus(file)
	require.NoError(t, err)

	err = e.Start(d.DAG, "")
	require.Error(t, err)

	status, err := e.GetLatestStatus(d.DAG)
	require.NoError(t, err)
	require.Equal(t, scheduler.StatusError, status.Status)
}

func TestStop(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	file := testDAG("stop.yaml")

	d, err := e.GetStatus(file)
	require.NoError(t, err)

	e.StartAsync(d.DAG, "")

	require.Eventually(t, func() bool {
		st, _ := e.GetCurrentStatus(d.DAG)
		return st.Status == scheduler.StatusRunning
	}, time.Millisecond*1500, time.Millisecond*100)

	_ = e.Stop(d.DAG)

	require.Eventually(t, func() bool {
		st, _ := e.GetLatestStatus(d.DAG)
		return st.Status == scheduler.StatusCancel
	}, time.Millisecond*1500, time.Millisecond*100)
}

func TestRestart(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	file := testDAG("restart.yaml")

	d, err := e.GetStatus(file)
	require.NoError(t, err)

	err = e.Restart(d.DAG)
	require.NoError(t, err)

	status, err := e.GetLatestStatus(d.DAG)
	require.NoError(t, err)
	require.Equal(t, scheduler.StatusSuccess, status.Status)
}

func TestRetry(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	file := testDAG("retry.yaml")

	d, err := e.GetStatus(file)
	require.NoError(t, err)

	err = e.Start(d.DAG, "x y z")
	require.NoError(t, err)

	status, err := e.GetLatestStatus(d.DAG)
	require.NoError(t, err)
	require.Equal(t, scheduler.StatusSuccess, status.Status)

	requestId := status.RequestId
	params := status.Params

	err = e.Retry(d.DAG, requestId)
	require.NoError(t, err)
	status, err = e.GetLatestStatus(d.DAG)
	require.NoError(t, err)

	require.Equal(t, scheduler.StatusSuccess, status.Status)
	require.Equal(t, params, status.Params)

	statusByRequestId, err := e.GetStatusByRequestId(d.DAG, status.RequestId)
	require.NoError(t, err)
	require.Equal(t, status, statusByRequestId)

	recentStatuses := e.GetRecentHistory(d.DAG, 1)
	require.Equal(t, status, recentStatuses[0].Status)
}

func TestUpdate(t *testing.T) {
	tmpDir, e, _ := setupTestTmpDir(t)
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
	err := e.UpdateDAG("non-existing-dag", validDAG)
	require.Error(t, err)

	// create a new DAG file
	id, err := e.CreateDAG("new-dag-file")
	require.NoError(t, err)

	// Update the DAG
	err = e.UpdateDAG(id, validDAG)
	require.NoError(t, err)

	// Check the content of the DAG file
	spec, err := e.GetDAGSpec(id)
	require.NoError(t, err)
	require.Equal(t, validDAG, spec)
}

func TestRemove(t *testing.T) {
	tmpDir, e, _ := setupTestTmpDir(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	spec := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
	id, err := e.CreateDAG("test")
	require.NoError(t, err)
	err = e.UpdateDAG(id, spec)
	require.NoError(t, err)

	// check file
	newSpec, err := e.GetDAGSpec(id)
	require.NoError(t, err)
	require.Equal(t, spec, newSpec)

	status, _ := e.GetStatus(id)

	// delete
	err = e.DeleteDAG(id, status.DAG.Location)
	require.NoError(t, err)
}

func TestCreateNewDAG(t *testing.T) {
	tmpDir, e, _ := setupTestTmpDir(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	id, err := e.CreateDAG("test-dag")
	require.NoError(t, err)

	// Check if the new DAG is actually created.
	d, err := dag.Load("", path.Join(tmpDir, ".dagu", "dags", id+".yaml"), "")
	require.NoError(t, err)
	require.Equal(t, "test-dag", d.Name)
}

func TestRenameDAG(t *testing.T) {
	tmpDir, e, _ := setupTestTmpDir(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create a DAG to rename.
	id, err := e.CreateDAG("old_name")
	require.NoError(t, err)
	_, err = e.GetStatus(path.Join(tmpDir, ".dagu", "dags", id+".yaml"))
	require.NoError(t, err)

	// Rename the file.
	err = e.Rename(id, id+"_renamed")

	// Check if the file is renamed.
	require.NoError(t, err)
	require.FileExists(t, path.Join(tmpDir, ".dagu", "dags", id+"_renamed.yaml"))
}

func TestEngine_GetStatus(t *testing.T) {
	t.Run("[Failure] Invalid DAG name", func(t *testing.T) {
		tmpDir, e, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		file := testDAG("invalid_dag")

		d, err := e.GetStatus(file)
		require.Error(t, err)
		require.NotNil(t, d)

		// Check the status contains error.
		require.Error(t, d.Error)
	})
}

func TestReadAll(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	dags, _, err := e.GetAllStatus()
	require.NoError(t, err)
	require.Greater(t, len(dags), 0)

	pattern := path.Join(testdataDir, "*.yaml")
	matches, err := filepath.Glob(pattern)
	require.NoError(t, err)
	if len(matches) != len(dags) {
		t.Fatalf("unexpected number of dags: %d", len(dags))
	}
}

func TestReadDAGStatus(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	file := testDAG("read_status.yaml")

	_, err := e.GetStatus(file)
	require.NoError(t, err)
}

func testDAG(name string) string {
	return path.Join(testdataDir, name)
}

func testNewStatus(dg *dag.DAG, reqId string, status scheduler.Status, nodeStatus scheduler.NodeStatus) *model.Status {
	now := time.Now()
	ret := model.NewStatus(
		dg, []model.NodeStepPair{
			{
				Node: scheduler.NodeState{Status: nodeStatus},
			},
		},
		status, 0, &now, nil)
	ret.RequestId = reqId
	return ret
}
