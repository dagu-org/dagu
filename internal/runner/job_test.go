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
	file := path.Join(testdataDir, "job_run.yaml")
	dr := controller.NewDAGReader()
	dag, err := dr.ReadDAG(file, false)
	require.NoError(t, err)
	c := controller.New(dag.DAG)

	j := &job{
		DAG:       dag.DAG,
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
	time.Sleep(time.Millisecond * 200)

	s, _ := c.GetLastStatus()
	require.Equal(t, scheduler.SchedulerStatus_Cancel, s.Status)

	err = j.Run()
	require.Equal(t, ErrJobFinished, err)
}
