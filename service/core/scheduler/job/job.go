package job

import (
	"errors"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/utils"
)

// TODO: write tests
type Job struct {
	DAG           *dag.DAG
	Command       string
	WorkDir       string
	Next          time.Time
	EngineFactory engine.Factory
}

var (
	ErrJobRunning      = errors.New("job already running")
	ErrJobIsNotRunning = errors.New("job is not running")
	ErrJobFinished     = errors.New("job already finished")
)

func (j *Job) GetDAG() *dag.DAG {
	return j.DAG
}

func (j *Job) Start() error {
	e := j.EngineFactory.Create()
	s, err := e.GetLatestStatus(j.DAG)
	if err != nil {
		return err
	}
	switch s.Status {
	case scheduler.SchedulerStatus_Running:
		// already running
		return ErrJobRunning
	case scheduler.SchedulerStatus_None:
	default:
		// check the last execution time
		t, err := utils.ParseTime(s.StartedAt)
		if err == nil {
			t = t.Truncate(time.Second * 60)
			if t.After(j.Next) || j.Next.Equal(t) {
				return ErrJobFinished
			}
		}
		// should not be here
	}
	return e.Start(j.DAG, "")
}

func (j *Job) Stop() error {
	e := j.EngineFactory.Create()
	s, err := e.GetLatestStatus(j.DAG)
	if err != nil {
		return err
	}
	if s.Status != scheduler.SchedulerStatus_Running {
		return ErrJobIsNotRunning
	}
	return e.Stop(j.DAG)
}

func (j *Job) Restart() error {
	e := j.EngineFactory.Create()
	return e.Restart(j.DAG)
}

func (j *Job) String() string {
	return j.DAG.Name
}
