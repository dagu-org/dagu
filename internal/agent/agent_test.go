package agent_test

import (
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var testsDir = path.Join(utils.MustGetwd(), "../../tests/testdata")

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("agent_test")
	settings.InitTest(tempDir)
	code := m.Run()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

func TestRunDAG(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_run.yaml"))
	require.NoError(t, err)

	status, err := testDAG(t, dag)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
}

func TestCancelDAG(t *testing.T) {
	for _, abort := range []func(*agent.Agent){
		func(a *agent.Agent) { a.Signal(syscall.SIGTERM) },
		func(a *agent.Agent) { a.Cancel(syscall.SIGTERM) },
		func(a *agent.Agent) { a.Kill(nil) },
	} {
		a, dag := testDAGAsync(t, testConfig("agent_sleep.yaml"))
		time.Sleep(time.Millisecond * 100)
		abort(a)
		time.Sleep(time.Millisecond * 500)
		status, err := controller.New(dag.Config).GetLastStatus()
		require.NoError(t, err)
		assert.Equal(t, scheduler.SchedulerStatus_Cancel, status.Status)
	}
}

func TestPreConditionInvalid(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_multiple_steps.yaml"))
	require.NoError(t, err)

	dag.Config.Preconditions = []*config.Condition{
		{
			Condition: "`echo 1`",
			Expected:  "0",
		},
	}

	status, err := testDAG(t, dag)
	require.Error(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Cancel, status.Status)
	for _, s := range status.Nodes {
		assert.Equal(t, scheduler.NodeStatus_Cancel, s.Status)
	}
}

func TestPreConditionValid(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_with_params.yaml"))
	require.NoError(t, err)

	dag.Config.Preconditions = []*config.Condition{
		{
			Condition: "`echo 1`",
			Expected:  "1",
		},
	}
	status, err := testDAG(t, dag)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
	for _, s := range status.Nodes {
		assert.Equal(t, scheduler.NodeStatus_Success, s.Status)
	}
}

func TestOnExit(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_on_exit.yaml"))
	require.NoError(t, err)
	status, err := testDAG(t, dag)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
	for _, s := range status.Nodes {
		assert.Equal(t, scheduler.NodeStatus_Success, s.Status)
	}
	assert.Equal(t, scheduler.NodeStatus_Success, status.OnExit.Status)
}

func TestRetry(t *testing.T) {
	cfg := testConfig("agent_retry.yaml")
	dag, err := controller.FromConfig(cfg)
	require.NoError(t, err)

	status, err := testDAG(t, dag)
	require.Error(t, err)
	assert.Equal(t, scheduler.SchedulerStatus_Error, status.Status)

	for _, n := range status.Nodes {
		n.Command = "true"
	}
	a := &agent.Agent{
		Config: &agent.Config{
			DAG: dag.Config,
		},
		RetryConfig: &agent.RetryConfig{
			Status: status,
		},
	}
	err = a.Run()
	status = a.Status()
	require.NoError(t, err)
	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)

	for _, n := range status.Nodes {
		if n.Status != scheduler.NodeStatus_Success &&
			n.Status != scheduler.NodeStatus_Skipped {
			t.Errorf("invalid status: %s", n.Status.String())
		}
	}
}

func testDAG(t *testing.T, dag *controller.DAG) (*models.Status, error) {
	t.Helper()
	a := &agent.Agent{Config: &agent.Config{
		DAG: dag.Config,
	}}
	err := a.Run()
	return a.Status(), err
}

func testConfig(name string) string {
	return path.Join(testsDir, name)
}

func testDAGAsync(t *testing.T, file string) (*agent.Agent, *controller.DAG) {
	t.Helper()

	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	a := &agent.Agent{Config: &agent.Config{
		DAG: dag.Config,
	}}

	go func() {
		a.Run()
	}()

	return a, dag
}
