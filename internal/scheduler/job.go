package scheduler

import (
	"errors"
	"time"

	"github.com/daguflow/dagu/internal/client"
	"github.com/daguflow/dagu/internal/dag"
	dagscheduler "github.com/daguflow/dagu/internal/dag/scheduler"
	"github.com/daguflow/dagu/internal/util"
)

var (
	errJobRunning      = errors.New("job already running")
	errJobIsNotRunning = errors.New("job is not running")
	errJobFinished     = errors.New("job already finished")
)

var _ jobCreator = (*jobCreatorImpl)(nil)

type jobCreatorImpl struct {
	Executable string
	WorkDir    string
	Client     client.Client
}

func (jf jobCreatorImpl) CreateJob(workflow *dag.DAG, next time.Time) job {
	return &jobImpl{
		DAG:        workflow,
		Executable: jf.Executable,
		WorkDir:    jf.WorkDir,
		Next:       next,
		Client:     jf.Client,
	}
}

var _ job = (*jobImpl)(nil)

type jobImpl struct {
	DAG        *dag.DAG
	Executable string
	WorkDir    string
	Next       time.Time
	Client     client.Client
}

func (j *jobImpl) GetDAG() *dag.DAG {
	return j.DAG
}

func (j *jobImpl) Start() error {
	latestStatus, err := j.Client.GetLatestStatus(j.DAG)
	if err != nil {
		return err
	}

	if latestStatus.Status == dagscheduler.StatusRunning {
		// already running
		return errJobRunning
	}

	// check the last execution time
	lastExecTime, err := util.ParseTime(latestStatus.StartedAt)
	if err == nil {
		lastExecTime = lastExecTime.Truncate(time.Second * 60)
		if lastExecTime.After(j.Next) || j.Next.Equal(lastExecTime) {
			return errJobFinished
		}
	}
	return j.Client.Start(j.DAG, client.StartOptions{
		Quiet: true,
	})
}

func (j *jobImpl) Stop() error {
	latestStatus, err := j.Client.GetLatestStatus(j.DAG)
	if err != nil {
		return err
	}
	if latestStatus.Status != dagscheduler.StatusRunning {
		return errJobIsNotRunning
	}
	return j.Client.Stop(j.DAG)
}

func (j *jobImpl) Restart() error {
	return j.Client.Restart(j.DAG, client.RestartOptions{
		Quiet: true,
	})
}

func (j *jobImpl) String() string {
	return j.DAG.Name
}
