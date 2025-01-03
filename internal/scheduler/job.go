package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	dagscheduler "github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/robfig/cron/v3"
)

var (
	errJobRunning      = errors.New("job already running")
	errJobIsNotRunning = errors.New("job is not running")
	errJobFinished     = errors.New("job already finished")
	errJobSkipped      = errors.New("job skipped")
)

var _ jobCreator = (*jobCreatorImpl)(nil)

type jobCreatorImpl struct {
	Executable string
	WorkDir    string
	Client     client.Client
}

func (jf jobCreatorImpl) CreateJob(dag *digraph.DAG, next time.Time, schedule cron.Schedule) job {
	return &jobImpl{
		DAG:        dag,
		Executable: jf.Executable,
		WorkDir:    jf.WorkDir,
		Next:       next,
		Schedule:   schedule,
		Client:     jf.Client,
	}
}

var _ job = (*jobImpl)(nil)

type jobImpl struct {
	DAG        *digraph.DAG
	Executable string
	WorkDir    string
	Next       time.Time
	Schedule   cron.Schedule
	Client     client.Client
}

func (j *jobImpl) GetDAG(_ context.Context) *digraph.DAG {
	return j.DAG
}

func (j *jobImpl) Start(ctx context.Context) error {
	latestStatus, err := j.Client.GetLatestStatus(ctx, j.DAG)
	if err != nil {
		return err
	}

	if latestStatus.Status == dagscheduler.StatusRunning {
		// already running
		return errJobRunning
	}

	// check the last execution time
	lastExecTime, err := stringutil.ParseTime(latestStatus.StartedAt)
	if err == nil {
		lastExecTime = lastExecTime.Truncate(time.Second * 60)
		if lastExecTime.After(j.Next) || j.Next.Equal(lastExecTime) {
			return errJobFinished
		}

		// Check the `skipIfSuccessful` is set to true in the DAG configuration.
		// When set to true, Dagu will automatically check the last successful run
		// time against the defined schedule. If the DAG has already run successfully
		// since the last scheduled time, the current run will be skipped.
		if j.DAG.SkipIfSuccessful {
			prev := j.Prev(ctx)
			if lastExecTime.After(prev) || lastExecTime.Equal(prev) {
				// Calculate the previous scheduled time
				lastStartedAt, _ := stringutil.ParseTime(latestStatus.StartedAt)
				return fmt.Errorf("%w: last successful run time: %s is after the previous scheduled time: %s", errJobSkipped, lastStartedAt, prev)
			}
		}
	}

	return j.Client.Start(ctx, j.DAG, client.StartOptions{Quiet: true})
}

func (j *jobImpl) Prev(_ context.Context) time.Time {
	// Since robfig/cron does not provide a way to get the previous schedule time,
	// we need to do it manually.
	// The idea is to get the next schedule time and subtract the duration of the schedule.
	// This will give us the previous schedule time.
	t := j.Schedule.Next(j.Next.Add(time.Second))
	return j.Next.Add(-t.Sub(j.Next))
}

func (j *jobImpl) Stop(ctx context.Context) error {
	latestStatus, err := j.Client.GetLatestStatus(ctx, j.DAG)
	if err != nil {
		return err
	}
	if latestStatus.Status != dagscheduler.StatusRunning {
		return errJobIsNotRunning
	}
	return j.Client.Stop(ctx, j.DAG)
}

func (j *jobImpl) Restart(ctx context.Context) error {
	return j.Client.Restart(ctx, j.DAG, client.RestartOptions{Quiet: true})
}

func (j *jobImpl) String() string {
	return j.DAG.Name
}
