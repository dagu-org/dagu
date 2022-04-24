package agent_test

import (
	"jobctl/internal/agent"
	"jobctl/internal/config"
	"jobctl/internal/controller"
	"jobctl/internal/models"
	"jobctl/internal/scheduler"
	"jobctl/internal/settings"
	"jobctl/internal/utils"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testsDir = path.Join(utils.MustGetwd(), "../../tests/testdata")

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("agent_test")
	settings.InitTest(tempDir)
	code := m.Run()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

func TestRunJob(t *testing.T) {
	job, err := controller.FromConfig(testConfig("agent_run.yaml"))
	require.NoError(t, err)

	status, err := testJob(t, job)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
}

func TestCancelJob(t *testing.T) {
	for _, abort := range []func(*agent.Agent){
		func(a *agent.Agent) { a.Signal(syscall.SIGTERM) },
		func(a *agent.Agent) { a.Cancel(syscall.SIGTERM) },
		func(a *agent.Agent) { a.Kill(nil) },
	} {
		a, job := testJobAsync(t, testConfig("agent_sleep.yaml"))
		time.Sleep(time.Millisecond * 100)
		abort(a)
		time.Sleep(time.Millisecond * 500)
		status, err := controller.New(job.Config).GetLastStatus()
		require.NoError(t, err)
		assert.Equal(t, scheduler.SchedulerStatus_Cancel, status.Status)
	}
}

func TestPreConditionInvalid(t *testing.T) {
	job, err := controller.FromConfig(testConfig("agent_multiple_steps.yaml"))
	require.NoError(t, err)

	job.Config.Preconditions = []*config.Condition{
		{
			Condition: "`echo 1`",
			Expected:  "0",
		},
	}

	status, err := testJob(t, job)
	require.Error(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Cancel, status.Status)
	for _, s := range status.Nodes {
		assert.Equal(t, scheduler.NodeStatus_Cancel, s.Status)
	}
}

func TestPreConditionValid(t *testing.T) {
	job, err := controller.FromConfig(testConfig("agent_with_params.yaml"))
	require.NoError(t, err)

	job.Config.Preconditions = []*config.Condition{
		{
			Condition: "`echo 1`",
			Expected:  "1",
		},
	}
	status, err := testJob(t, job)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
	for _, s := range status.Nodes {
		assert.Equal(t, scheduler.NodeStatus_Success, s.Status)
	}
}

func TestOnExit(t *testing.T) {
	job, err := controller.FromConfig(testConfig("agent_on_exit.yaml"))
	require.NoError(t, err)
	status, err := testJob(t, job)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
	for _, s := range status.Nodes {
		assert.Equal(t, scheduler.NodeStatus_Success, s.Status)
	}
	assert.Equal(t, scheduler.NodeStatus_Success, status.OnExit.Status)
}

func TestRetry(t *testing.T) {
	cfg := testConfig("agent_retry.yaml")
	job, err := controller.FromConfig(cfg)
	require.NoError(t, err)

	status, err := testJob(t, job)
	require.Error(t, err)
	assert.Equal(t, scheduler.SchedulerStatus_Error, status.Status)

	for _, n := range status.Nodes {
		n.Command = "true"
	}
	a := &agent.Agent{
		Config: &agent.Config{
			Job: job.Config,
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

func testJob(t *testing.T, job *controller.Job) (*models.Status, error) {
	t.Helper()
	a := &agent.Agent{Config: &agent.Config{
		Job: job.Config,
	}}
	err := a.Run()
	return a.Status(), err
}

func testConfig(name string) string {
	return path.Join(testsDir, name)
}

func testJobAsync(t *testing.T, file string) (*agent.Agent, *controller.Job) {
	t.Helper()

	job, err := controller.FromConfig(file)
	require.NoError(t, err)

	a := &agent.Agent{Config: &agent.Config{
		Job: job.Config,
	}}

	go func() {
		a.Run()
	}()

	return a, job
}
