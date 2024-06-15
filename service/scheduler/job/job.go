package job

import (
	"errors"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/util"
)

// TODO: write tests
type Job struct {
	DAG        *dag.DAG
	Executable string
	WorkDir    string
	Next       time.Time
	Engine     engine.Engine
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
	latestStatus, err := j.Engine.GetLatestStatus(j.DAG)
	if err != nil {
		return err
	}

	if latestStatus.Status == scheduler.StatusRunning {
		// already running
		return ErrJobRunning
	}

	// check the last execution time
	lastExecTime, err := util.ParseTime(latestStatus.StartedAt)
	if err == nil {
		lastExecTime = lastExecTime.Truncate(time.Second * 60)
		if lastExecTime.After(j.Next) || j.Next.Equal(lastExecTime) {
			return ErrJobFinished
		}
	}
	// should not be here
	return j.Engine.Start(j.DAG, "")
}

func (j *Job) Stop() error {
	latestStatus, err := j.Engine.GetLatestStatus(j.DAG)
	if err != nil {
		return err
	}
	if latestStatus.Status != scheduler.StatusRunning {
		return ErrJobIsNotRunning
	}
	return j.Engine.Stop(j.DAG)
}

func (j *Job) Restart() error {
	return j.Engine.Restart(j.DAG)
}

func (j *Job) String() string {
	return j.DAG.Name
}
