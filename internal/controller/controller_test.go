package controller_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/agent"
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
	assert.Equal(t, "basic success", dag.Config.Name)
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
