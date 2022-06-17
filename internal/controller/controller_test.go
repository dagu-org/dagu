package controller_test

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/sock"
	"github.com/yohamta/dagu/internal/utils"
)

var (
	testsDir = path.Join(utils.MustGetwd(), "../../tests/testdata")
)

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("controller_test")
	settings.InitTest(tempDir)
	code := m.Run()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

func testConfig(name string) string {
	return path.Join(testsDir, name)
}

func TestGetStatus(t *testing.T) {
	file := testConfig("controller_success.yaml")
	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	st, err := controller.New(dag.Config).GetStatus()
	require.NoError(t, err)
	require.Equal(t, scheduler.SchedulerStatus_None, st.Status)
}

func TestGetStatusRunningAndDone(t *testing.T) {
	file := testConfig("controller_status.yaml")

	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	socketServer, _ := sock.NewServer(
		&sock.Config{
			Addr: sock.GetSockAddr(file),
			HandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				status := models.NewStatus(
					dag.Config, []*scheduler.Node{},
					scheduler.SchedulerStatus_Running, 0, nil, nil)
				w.WriteHeader(http.StatusOK)
				b, _ := status.ToJson()
				w.Write(b)
			},
		})
	go func() {
		socketServer.Serve(nil)
	}()
	defer socketServer.Shutdown()

	time.Sleep(time.Millisecond * 100)
	st, _ := controller.New(dag.Config).GetStatus()
	require.Equal(t, scheduler.SchedulerStatus_Running, st.Status)

	socketServer.Shutdown()

	st, _ = controller.New(dag.Config).GetStatus()
	require.Equal(t, scheduler.SchedulerStatus_None, st.Status)
}

func TestGetDAG(t *testing.T) {
	file := testConfig("controller_get_dag.yaml")
	dag, err := controller.FromConfig(file)
	require.NoError(t, err)
	require.Equal(t, "test", dag.Config.Name)
}

func TestGetDAGList(t *testing.T) {
	dags, errs, err := controller.GetDAGs(testsDir)
	require.NoError(t, err)
	require.Equal(t, 0, len(errs))

	matches, _ := filepath.Glob(path.Join(testsDir, "*.yaml"))
	require.Equal(t, len(matches), len(dags))
}

func TestUpdateStatus(t *testing.T) {
	file := testConfig("controller_update_status.yaml")

	dag, err := controller.FromConfig(file)
	require.NoError(t, err)
	req := "test-update-status"
	now := time.Now()

	db := database.New(database.DefaultConfig())
	w, _, _ := db.NewWriter(dag.Config.ConfigPath, now, req)
	err = w.Open()
	require.NoError(t, err)

	st := newStatus(dag.Config, req,
		scheduler.SchedulerStatus_Success, scheduler.NodeStatus_Success)

	err = w.Write(st)
	require.NoError(t, err)
	w.Close()

	time.Sleep(time.Millisecond * 100)

	st, err = controller.New(dag.Config).GetStatusByRequestId(req)
	require.NoError(t, err)
	require.Equal(t, scheduler.NodeStatus_Success, st.Nodes[0].Status)

	st.Nodes[0].Status = scheduler.NodeStatus_Error
	err = controller.New(dag.Config).UpdateStatus(st)
	require.NoError(t, err)

	updated, err := controller.New(dag.Config).GetStatusByRequestId(req)
	require.NoError(t, err)

	require.Equal(t, 1, len(st.Nodes))
	require.Equal(t, scheduler.NodeStatus_Error, updated.Nodes[0].Status)
}

func TestUpdateStatusFailure(t *testing.T) {
	file := testConfig("controller_update_status_failed.yaml")

	dag, err := controller.FromConfig(file)
	require.NoError(t, err)
	req := "test-update-status-failure"

	socketServer, _ := sock.NewServer(
		&sock.Config{
			Addr: sock.GetSockAddr(file),
			HandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				st := newStatus(dag.Config, req,
					scheduler.SchedulerStatus_Running, scheduler.NodeStatus_Success)
				w.WriteHeader(http.StatusOK)
				b, _ := st.ToJson()
				w.Write(b)
			},
		})
	go func() {
		socketServer.Serve(nil)
	}()
	defer socketServer.Shutdown()

	st := newStatus(dag.Config, req,
		scheduler.SchedulerStatus_Error, scheduler.NodeStatus_Error)
	err = controller.New(dag.Config).UpdateStatus(st)
	require.Error(t, err)

	st.RequestId = "invalid request id"
	err = controller.New(dag.Config).UpdateStatus(st)
	require.Error(t, err)
}

func TestStartStop(t *testing.T) {
	file := testConfig("controller_start.yaml")
	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	c := controller.New(dag.Config)
	go func() {
		err = c.Start(path.Join(utils.MustGetwd(), "../../bin/dagu"), "", "")
		require.NoError(t, err)
	}()

	require.Eventually(t, func() bool {
		st, _ := c.GetStatus()
		return st.Status == scheduler.SchedulerStatus_Running
	}, time.Millisecond*1500, time.Millisecond*100)

	c.Stop()

	require.Eventually(t, func() bool {
		st, _ := c.GetLastStatus()
		return st.Status == scheduler.SchedulerStatus_Cancel
	}, time.Millisecond*1500, time.Millisecond*100)
}

func TestRetry(t *testing.T) {
	file := testConfig("controller_retry.yaml")
	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	c := controller.New(dag.Config)
	err = c.Start(path.Join(utils.MustGetwd(), "../../bin/dagu"), "", "x y z")
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 50)

	s, err := c.GetLastStatus()
	require.Equal(t, scheduler.SchedulerStatus_Success, s.Status)
	require.NoError(t, err)

	err = c.Retry(path.Join(utils.MustGetwd(), "../../bin/dagu"), "", s.RequestId)
	require.NoError(t, err)
	s2, err := c.GetLastStatus()
	require.NoError(t, err)

	require.Equal(t, scheduler.SchedulerStatus_Success, s2.Status)
	require.Equal(t, s.Params, s2.Params)

	s3, err := c.GetStatusByRequestId(s2.RequestId)
	require.NoError(t, err)
	require.Equal(t, s2, s3)

	s4 := c.GetStatusHist(1)
	require.Equal(t, s2, s4[0].Status)
}

func TestSave(t *testing.T) {
	tmpDir := utils.MustTempDir("controller-test-save")
	defer os.RemoveAll(tmpDir)
	cfg := &config.Config{
		Name:       "test",
		ConfigPath: path.Join(tmpDir, "test.yaml"),
	}

	c := controller.New(cfg)

	// invalid config
	dat := `name: test DAG`
	err := c.Save(dat)
	require.Error(t, err)

	// valid config
	dat = `name: test DAG
steps:
  - name: "1"
    command: "true"
`
	err = c.Save(dat)
	require.Error(t, err) // no config file

	// create file
	f, _ := utils.CreateFile(cfg.ConfigPath)
	defer f.Close()

	err = c.Save(dat)
	require.NoError(t, err) // no config file

	// check file
	saved, _ := os.Open(cfg.ConfigPath)
	defer saved.Close()
	b, _ := io.ReadAll(saved)
	require.Equal(t, dat, string(b))
}

func TestNewConfig(t *testing.T) {
	tmpDir := utils.MustTempDir("controller-test-save")
	defer os.RemoveAll(tmpDir)

	// invalid filename
	filename := path.Join(tmpDir, "test")
	err := controller.NewConfig(filename)
	require.Error(t, err)

	// correct filename
	filename = path.Join(tmpDir, "test.yaml")
	err = controller.NewConfig(filename)
	require.NoError(t, err)

	// check file
	cl := &config.Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}

	cfg, err := cl.Load(filename, "")
	require.NoError(t, err)
	require.Equal(t, "test", cfg.Name)

	steps := cfg.Steps[0]
	require.Equal(t, "step1", steps.Name)
	require.Equal(t, "echo", steps.Command)
	require.Equal(t, []string{"hello"}, steps.Args)
}

func TestRenameConfig(t *testing.T) {
	tmpDir := utils.MustTempDir("controller-test-rename")
	defer os.RemoveAll(tmpDir)
	oldFile := path.Join(tmpDir, "test.yaml")
	newFile := path.Join(tmpDir, "test2.yaml")

	err := controller.NewConfig(oldFile)
	require.NoError(t, err)

	err = controller.RenameConfig(oldFile, "invalid-config-name")
	require.Error(t, err)

	err = controller.RenameConfig(oldFile, newFile)
	require.NoError(t, err)
	require.FileExists(t, newFile)
}

func newStatus(cfg *config.Config, reqId string,
	schedulerStatus scheduler.SchedulerStatus, nodeStatus scheduler.NodeStatus) *models.Status {
	n := time.Now()
	ret := models.NewStatus(
		cfg, []*scheduler.Node{
			{
				NodeState: scheduler.NodeState{
					Status: nodeStatus,
				},
			},
		},
		schedulerStatus, 0, &n, nil)
	ret.RequestId = reqId
	return ret
}
