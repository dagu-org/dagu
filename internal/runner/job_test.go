package runner

import (
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
)

func TestJobStart(t *testing.T) {
	file := path.Join(testdataDir, "start.yaml")
	dr := controller.NewDAGStatusReader(jsondb.New())
	dag, _ := dr.ReadStatus(file, false)
	c := controller.New(dag.DAG, jsondb.New())

	j := &job{
		DAG:    dag.DAG,
		Config: testConfig,
		Next:   time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	}

	go func() {
		_ = j.Start()
	}()

	time.Sleep(time.Millisecond * 100)

	err := j.Start()
	require.Equal(t, ErrJobRunning, err)

	err = c.Stop()
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 200)

	s, _ := c.GetLastStatus()
	require.Equal(t, scheduler.SchedulerStatus_Cancel, s.Status)

	err = j.Start()
	require.Equal(t, ErrJobFinished, err)
}

func TestJobSop(t *testing.T) {
	file := path.Join(testdataDir, "stop.yaml")
	dr := controller.NewDAGStatusReader(jsondb.New())
	dag, _ := dr.ReadStatus(file, false)

	j := &job{
		DAG:    dag.DAG,
		Config: testConfig,
		Next:   time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC),
	}

	go func() {
		_ = j.Start()
	}()

	c := controller.New(dag.DAG, jsondb.New())

	require.Eventually(t, func() bool {
		s, _ := c.GetLastStatus()
		return scheduler.SchedulerStatus_Running == s.Status
	}, time.Millisecond*1500, time.Millisecond*100)

	err := j.Stop()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		s, _ := c.GetLastStatus()
		return scheduler.SchedulerStatus_Cancel == s.Status
	}, time.Millisecond*1500, time.Millisecond*100)
}
