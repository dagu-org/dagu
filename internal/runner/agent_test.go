package runner

import (
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

func TestAgent(t *testing.T) {
	tmpDir := utils.MustTempDir("runner_agent_test")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	agent := NewAgent(
		&config.Config{
			DAGs:    testdataDir,
			Command: testBin,
			LogDir:  path.Join(tmpDir, "log"),
		})
	utils.FixedTime = now

	go func() {
		err := agent.Start()
		require.NoError(t, err)
	}()

	pathToDAG := path.Join(testdataDir, "scheduled_job.yaml")
	loader := &dag.Loader{}
	d, err := loader.LoadMetadataOnly(pathToDAG)
	require.NoError(t, err)
	c := controller.New(d, jsondb.New())

	require.Eventually(t, func() bool {
		status, err := c.GetLastStatus()
		return err == nil && status.Status == scheduler.SchedulerStatus_Success
	}, time.Second*1, time.Millisecond*100)

	agent.Signal(os.Interrupt)
}

func TestAgentForStop(t *testing.T) {
	tmpDir := utils.MustTempDir("runner_agent_test_for_stop")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	now := time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC)
	agent := NewAgent(
		&config.Config{
			DAGs:    testdataDir,
			Command: testBin,
			LogDir:  path.Join(tmpDir, "log"),
		})
	utils.FixedTime = now

	// read the test DAG
	file := path.Join(testdataDir, "start_stop.yaml")
	dr := controller.NewDAGStatusReader(jsondb.New())
	d, _ := dr.ReadStatus(file, false)
	c := controller.New(d.DAG, jsondb.New())

	j := &job{
		DAG:    d.DAG,
		Config: testConfig,
		Next:   time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	}

	// start the test job
	go func() {
		_ = j.Start()
	}()

	time.Sleep(time.Millisecond * 100)

	// confirm the job is running
	status, err := c.GetLastStatus()
	require.NoError(t, err)
	require.Equal(t, scheduler.SchedulerStatus_Running, status.Status)

	// start the agent
	go func() {
		err := agent.Start()
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond * 100)

	// confirm the test job is canceled
	require.Eventually(t, func() bool {
		s, err := c.GetLastStatus()
		return err == nil && s.Status == scheduler.SchedulerStatus_Cancel
	}, time.Second*1, time.Millisecond*100)

	// stop the agent
	agent.Signal(os.Interrupt)
}
