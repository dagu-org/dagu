package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/robfig/cron/v3"
)

// Error variables for job states.
var (
	ErrJobRunning      = errors.New("job already running")
	ErrJobIsNotRunning = errors.New("job is not running")
	ErrJobFinished     = errors.New("job already finished")
	ErrJobSkipped      = errors.New("job skipped")
	ErrJobSuccess      = errors.New("job already successful")
)

var _ Job = (*dagJob)(nil)

type dagJob struct {
	DAG        *digraph.DAG
	Executable string
	WorkDir    string
	Next       time.Time
	Schedule   cron.Schedule
	Client     client.Client
}

// GetDAG returns the DAG associated with this job.
func (job *dagJob) GetDAG(_ context.Context) *digraph.DAG {
	return job.DAG
}

// Start attempts to run the job if it is not already running and is ready.
func (job *dagJob) Start(ctx context.Context) error {
	latestStatus, err := job.Client.GetLatestStatus(ctx, job.DAG)
	if err != nil {
		return err
	}

	// Guard against already running jobs.
	if latestStatus.Status == scheduler.StatusRunning {
		return ErrJobRunning
	}

	// Check if the job is ready to start.
	if err := job.ready(ctx, latestStatus); err != nil {
		return err
	}

	// Job is ready; proceed to start.
	return job.Client.Start(ctx, job.DAG, client.StartOptions{Quiet: true})
}

// ready checks whether the job can be safely started based on the latest status.
func (job *dagJob) ready(ctx context.Context, latestStatus persistence.Status) error {
	// Prevent starting if it's already running.
	if latestStatus.Status == scheduler.StatusRunning {
		return ErrJobRunning
	}

	latestStartedAt, err := stringutil.ParseTime(latestStatus.StartedAt)
	if err != nil {
		// If parsing fails, log and continue (don't skip).
		logger.Error(ctx, "failed to parse the last successful run time", "err", err)
		return nil
	}

	// Skip if the last successful run time is on or after the next scheduled time.
	latestStartedAt = latestStartedAt.Truncate(time.Minute)
	if latestStartedAt.After(job.Next) || job.Next.Equal(latestStartedAt) {
		return ErrJobFinished
	}

	// Check if we should skip this run due to a prior successful run.
	return job.skipIfSuccessful(ctx, latestStatus, latestStartedAt)
}

// skipIfSuccessful checks if the DAG has already run successfully in the window since the last scheduled time.
// If so, the current run is skipped.
func (job *dagJob) skipIfSuccessful(ctx context.Context, latestStatus persistence.Status, latestStartedAt time.Time) error {
	// If skip is not configured, or the DAG is not currently successful, do nothing.
	if !job.DAG.SkipIfSuccessful || latestStatus.Status != scheduler.StatusSuccess {
		return nil
	}

	prevExecTime := job.prevExecTime(ctx)
	if (latestStartedAt.After(prevExecTime) || latestStartedAt.Equal(prevExecTime)) &&
		latestStartedAt.Before(job.Next) {
		logger.Infof(ctx, "skipping the job because it has already run successfully at %s", latestStartedAt)
		return ErrJobSuccess
	}
	return nil
}

// prevExecTime calculates the previous schedule time from 'Next' by subtracting
// the schedule duration between runs.
func (job *dagJob) prevExecTime(_ context.Context) time.Time {
	nextNextRunTime := job.Schedule.Next(job.Next.Add(time.Second))
	duration := nextNextRunTime.Sub(job.Next)
	return job.Next.Add(-duration)
}

// Stop halts a running job if it's currently running.
func (job *dagJob) Stop(ctx context.Context) error {
	latestStatus, err := job.Client.GetLatestStatus(ctx, job.DAG)
	if err != nil {
		return err
	}
	if latestStatus.Status != scheduler.StatusRunning {
		return ErrJobIsNotRunning
	}
	return job.Client.Stop(ctx, job.DAG)
}

// Restart restarts the job unconditionally (quiet mode).
func (job *dagJob) Restart(ctx context.Context) error {
	return job.Client.Restart(ctx, job.DAG, client.RestartOptions{Quiet: true})
}

// String returns a string representation of the job, which is the DAG's name.
func (job *dagJob) String() string {
	return job.DAG.Name
}
