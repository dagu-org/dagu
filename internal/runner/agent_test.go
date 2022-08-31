package runner

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

func TestAgent(t *testing.T) {
	tmpDir := utils.MustTempDir("runner_agent_test")
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	a := NewAgent(
		&admin.Config{
			DAGs:    testdataDir,
			Command: testBin,
			LogDir:  path.Join(tmpDir, "log"),
		})
	utils.FixedTime = now

	go func() {
		err := a.Start()
		require.NoError(t, err)
	}()

	f := path.Join(testdataDir, "scheduled_job.yaml")
	cl := &dag.Loader{}
	dag, err := cl.LoadHeadOnly(f)
	require.NoError(t, err)
	c := controller.New(dag)

	require.Eventually(t, func() bool {
		s, err := c.GetLastStatus()
		return err == nil && s.Status == scheduler.SchedulerStatus_Success
	}, time.Second*1, time.Millisecond*100)

	a.Stop()
}

func TestAgentForStop(t *testing.T) {
	tmpDir := utils.MustTempDir("runner_agent_test_for_stop")
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	now := time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC)
	a := NewAgent(
		&admin.Config{
			DAGs:    testdataDir,
			Command: testBin,
			LogDir:  path.Join(tmpDir, "log"),
		})
	utils.FixedTime = now

	// read the test DAG
	file := path.Join(testdataDir, "start_stop.yaml")
	dr := controller.NewDAGReader()
	dag, _ := dr.ReadDAG(file, false)
	c := controller.New(dag.DAG)

	j := &job{
		DAG:    dag.DAG,
		Config: testConfig,
		Next:   time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	}

	// start the test job
	go func() {
		_ = j.Start()
	}()

	time.Sleep(time.Millisecond * 100)

	// confirm the job is running
	s, _ := c.GetLastStatus()
	require.Equal(t, scheduler.SchedulerStatus_Running, s.Status)

	// start the agent
	go func() {
		err := a.Start()
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond * 100)

	// confirm the test job is canceled
	require.Eventually(t, func() bool {
		s, err := c.GetLastStatus()
		return err == nil && s.Status == scheduler.SchedulerStatus_Cancel
	}, time.Second*1, time.Millisecond*100)

	// stop the agent
	a.Stop()
}
