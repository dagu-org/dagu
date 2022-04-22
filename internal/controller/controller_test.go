package controller_test

import (
	"jobctl/internal/agent"
	"jobctl/internal/controller"
	"jobctl/internal/scheduler"
	"jobctl/internal/settings"
	"jobctl/internal/utils"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	file := testConfig("basic_success_2.yaml")
	job, err := controller.FromConfig(file)
	require.NoError(t, err)

	st, err := controller.New(job.Config).GetStatus()
	require.NoError(t, err)
	assert.Equal(t, scheduler.SchedulerStatus_None, st.Status)
}

func TestGetStatusRunningAndDone(t *testing.T) {
	file := testConfig("basic_sleep.yaml")

	job, err := controller.FromConfig(file)
	require.NoError(t, err)

	a := agent.Agent{Config: &agent.Config{
		Job: job.Config,
	}}

	go func() {
		err := a.Run()
		require.NoError(t, err)
	}()
	time.Sleep(time.Millisecond * 500)

	st, err := controller.New(job.Config).GetStatus()
	require.NoError(t, err)
	time.Sleep(time.Millisecond * 50)

	assert.Equal(t, scheduler.SchedulerStatus_Running, st.Status)

	assert.Eventually(t, func() bool {
		st, _ := controller.New(job.Config).GetLastStatus()
		return scheduler.SchedulerStatus_Success == st.Status
	}, time.Millisecond*1500, time.Millisecond*100)
}

func TestGetJob(t *testing.T) {
	file := testConfig("basic_success.yaml")
	job, err := controller.FromConfig(file)
	require.NoError(t, err)
	assert.Equal(t, "basic success", job.Config.Name)
}

func TestGetJobList(t *testing.T) {
	jobs, err := controller.GetJobList(testsDir)
	require.NoError(t, err)

	matches, err := filepath.Glob(path.Join(testsDir, "*.yaml"))
	assert.Equal(t, len(matches), len(jobs))
}
