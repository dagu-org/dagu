package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	dagscheduler "github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/robfig/cron/v3"
)

var (
	errJobRunning      = errors.New("job already running")
	errJobIsNotRunning = errors.New("job is not running")
	errJobFinished     = errors.New("job already finished")
	errJobSkipped      = errors.New("job skipped")
	errJobSuccess      = errors.New("job already successful")
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
		return errJobRunning
	}

	if err := j.ready(ctx, latestStatus); err != nil {
		return err
	}

	return j.Client.Start(ctx, j.DAG, client.StartOptions{Quiet: true})
}

func (j *jobImpl) ready(ctx context.Context, latestStatus model.Status) error {
	if latestStatus.Status == dagscheduler.StatusRunning {
		// If the job is already running, we should not start it at the same time.
		return errJobRunning
	}

	latestStartedAt, err := stringutil.ParseTime(latestStatus.StartedAt)
	if err != nil {
		// This should not happen, but if it does, we should not skip the job.
		logger.Error(ctx, "failed to parse the last successful run time", "err", err)
		return nil
	}

	// Skip the job if the last successful run time is after the next scheduled time or equal to it.
	latestStartedAt = latestStartedAt.Truncate(time.Minute)
	if latestStartedAt.After(j.Next) || j.Next.Equal(latestStartedAt) {
		return errJobFinished
	}

	return j.skipIfSuccessful(ctx, latestStatus, latestStartedAt)
}

// skipIfSuccessful checks if the DAG has already run successfully since the last scheduled time.
// If the DAG has already run successfully since the last scheduled time, the current run will be skipped.
// For example, if the DAG is scheduled to run every 5 minutes and the last successful run was at 12:00:00,
// the next run is scheduled for 12:05:00. If the DAG runs successfully at 12:03:00, the next run will be skipped.
// This allows users to run the DAG earlier than the scheduled time when needed.
func (j *jobImpl) skipIfSuccessful(ctx context.Context, latestStatus model.Status, latestStartedAt time.Time) error {
	if !j.DAG.SkipIfSuccessful || latestStatus.Status != dagscheduler.StatusSuccess {
		// If the latest status is not successful, or the `skipIfSuccessful`, no need to check the last execution time,
		// because the job should run regardless of the last execution time.
		return nil
	}

	prevExecTime := j.prevExecTime(ctx)
	a := latestStartedAt.After(prevExecTime)
	b := latestStartedAt.Equal(prevExecTime)
	c := latestStartedAt.Before(j.Next)
	println(fmt.Sprintf("a: %v, b: %v, c: %v", a, b, c))
	if (latestStartedAt.After(prevExecTime) || latestStartedAt.Equal(prevExecTime)) && latestStartedAt.Before(j.Next) {
		logger.Infof(ctx, "skipping the job because it has already run successfully at %s", latestStartedAt)
		return errJobSuccess
	}

	return nil
}

func (j *jobImpl) prevExecTime(_ context.Context) time.Time {
	// Calculate the previous schedule time by subtracting the duration of the schedule from the next schedule time.
	nextNextRunTime := j.Schedule.Next(j.Next.Add(time.Second))
	duration := nextNextRunTime.Sub(j.Next)
	return j.Next.Add(-duration)
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
