package controller_test

import (
	"github.com/yohamta/dagu/agent"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var (
	testsDir = path.Join(utils.MustGetwd(), "../../tests/testdata")
)

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("controller_test")
	settings.InitTest(tempDir)
	code := m.Run()
	_ = os.RemoveAll(tempDir)
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
	assert.Equal(t, scheduler.SchedulerStatus_None, st.Status)
}

func TestGetStatusRunningAndDone(t *testing.T) {
	file := testConfig("controller_status.yaml")

	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	a := agent.Agent{Config: &agent.Config{
		DAG: dag.Config,
	}}

	go func() {
		err := a.Run()
		require.NoError(t, err)
	}()
	time.Sleep(time.Millisecond * 500)

	st, err := controller.New(dag.Config).GetStatus()
	require.NoError(t, err)
	time.Sleep(time.Millisecond * 50)

	assert.Equal(t, scheduler.SchedulerStatus_Running, st.Status)

	assert.Eventually(t, func() bool {
		st, _ := controller.New(dag.Config).GetLastStatus()
		return scheduler.SchedulerStatus_Success == st.Status
	}, time.Millisecond*1500, time.Millisecond*100)
}

func TestGetDAG(t *testing.T) {
	file := testConfig("controller_get_dag.yaml")
	dag, err := controller.FromConfig(file)
	require.NoError(t, err)
	assert.Equal(t, "test", dag.Config.Name)
}

func TestGetDAGList(t *testing.T) {
	dags, errs, err := controller.GetDAGs(testsDir)
	require.NoError(t, err)
	require.Equal(t, 0, len(errs))

	matches, _ := filepath.Glob(path.Join(testsDir, "*.yaml"))
	assert.Equal(t, len(matches), len(dags))
}

func TestUpdateStatus(t *testing.T) {
	file := testConfig("controller_update_status.yaml")

	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	a := agent.Agent{Config: &agent.Config{
		DAG: dag.Config,
	}}

	err = a.Run()
	require.NoError(t, err)

	st, err := controller.New(dag.Config).GetLastStatus()
	require.NoError(t, err)

	require.Equal(t, 1, len(st.Nodes))
	require.Equal(t, scheduler.NodeStatus_Success, st.Nodes[0].Status)

	st.Nodes[0].Status = scheduler.NodeStatus_Error
	err = controller.New(dag.Config).UpdateStatus(st)
	require.NoError(t, err)

	updated, err := controller.New(dag.Config).GetLastStatus()
	require.NoError(t, err)

	require.Equal(t, 1, len(st.Nodes))
	require.Equal(t, scheduler.NodeStatus_Error, updated.Nodes[0].Status)
}

func TestUpdateStatusError(t *testing.T) {
	file := testConfig("controller_update_status_failed.yaml")

	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	a := agent.Agent{Config: &agent.Config{
		DAG: dag.Config,
	}}

	go func() {
		err = a.Run()
		require.Error(t, err)
	}()

	time.Sleep(time.Millisecond * 30)

	c := controller.New(dag.Config)
	st, err := c.GetLastStatus()
	require.NoError(t, err)
	require.Equal(t, scheduler.SchedulerStatus_Running, st.Status)

	st.Nodes[0].Status = scheduler.NodeStatus_Error
	err = c.UpdateStatus(st)
	require.Error(t, err)

	err = c.Stop()
	require.NoError(t, err)

	st.RequestId = "invalid request id"
	err = c.UpdateStatus(st)
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

	_ = c.Stop()

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
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
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
	defer func() {
		_ = f.Close()
	}()

	err = c.Save(dat)
	require.NoError(t, err) // no config file

	// check file
	saved, _ := os.Open(cfg.ConfigPath)
	defer func() {
		_ = saved.Close()
	}()
	b, _ := ioutil.ReadAll(saved)
	require.Equal(t, dat, string(b))
}

func TestNewConfig(t *testing.T) {
	tmpDir := utils.MustTempDir("controller-test-save")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

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
