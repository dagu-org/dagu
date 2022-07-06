package runner

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
)

func TestJobRun(t *testing.T) {
	file := path.Join(testsDir, "testdata/runner_job_run.yaml")
	dag, err := controller.NewDAG(file, false)
	require.NoError(t, err)
	c := controller.New(dag.Config)

	j := &job{
		DAG:       dag.Config,
		Config:    testConfig,
		StartTime: time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	}

	go func() {
		_ = j.Run()
	}()

	time.Sleep(time.Millisecond * 100)

	err = j.Run()
	require.Equal(t, ErrJobRunning, err)

	c.Stop()
	time.Sleep(time.Millisecond * 100)

	s, _ := c.GetLastStatus()
	require.Equal(t, scheduler.SchedulerStatus_Cancel, s.Status)

	err = j.Run()
	require.Equal(t, ErrJobFinished, err)
}
