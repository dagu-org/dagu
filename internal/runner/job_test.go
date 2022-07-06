package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
)

func TestJob(t *testing.T) {
	dag := temporaryDAG(t, "job_test",
		testDAGConfig(t, "* * * * *", "sleep 1"))
	j := &job{
		DAG:    dag,
		Config: testConfig,
	}

	require.Equal(t, "job_test", j.String())

	ch := make(chan struct{})
	go func() {
		err := j.Run()
		require.NoError(t, err)
		ch <- struct{}{}
	}()

	// Fail to run the job because it's already running
	time.Sleep(time.Millisecond * 500)
	err := j.Run()
	require.Equal(t, ErrJobRunning, err)

	<-ch

	c := controller.New(dag)
	status, err := c.GetLastStatus()
	require.NoError(t, err)
	require.Equal(t, scheduler.SchedulerStatus_Success, status.Status)

	// Fail to run the job because it's already finished
	j2 := &job{
		DAG:       dag,
		Config:    testConfig,
		StartTime: j.StartTime,
	}
	err = j2.Run()
	require.Equal(t, ErrJobFinished, err)
}
