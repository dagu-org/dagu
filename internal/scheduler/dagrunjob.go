package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
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

var _ Job = (*DAGRunJob)(nil)

// DAGRunJob represents a job that runs a DAG.
type DAGRunJob struct {
	DAG        *digraph.DAG
	Executable string
	WorkDir    string
	Next       time.Time
	Schedule   cron.Schedule
	Client     history.DAGRunManager
}

// GetDAG returns the DAG associated with this job.
func (d *DAGRunJob) GetDAG(_ context.Context) *digraph.DAG {
	return d.DAG
}

// Start attempts to run the job if it is not already running and is ready.
func (d *DAGRunJob) Start(ctx context.Context) error {
	latestStatus, err := d.Client.GetLatestStatus(ctx, d.DAG)
	if err != nil {
		return err
	}

	// Guard against already running jobs.
	if latestStatus.Status == scheduler.StatusRunning {
		return ErrJobRunning
	}

	// Check if the job is ready to start.
	if err := d.Ready(ctx, latestStatus); err != nil {
		return err
	}

	// Job is ready; proceed to start.
	return d.Client.StartDAGRun(ctx, d.DAG, history.StartOptions{Quiet: true})
}

// Ready checks whether the job can be safely started based on the latest status.
func (d *DAGRunJob) Ready(ctx context.Context, latestStatus models.DAGRunStatus) error {
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
	if latestStartedAt.After(d.Next) || d.Next.Equal(latestStartedAt) {
		return ErrJobFinished
	}

	// Check if we should skip this run due to a prior successful run.
	return d.skipIfSuccessful(ctx, latestStatus, latestStartedAt)
}

// skipIfSuccessful checks if the DAG has already run successfully in the window since the last scheduled time.
// If so, the current run is skipped.
func (d *DAGRunJob) skipIfSuccessful(ctx context.Context, latestStatus models.DAGRunStatus, latestStartedAt time.Time) error {
	// If skip is not configured, or the DAG is not currently successful, do nothing.
	if !d.DAG.SkipIfSuccessful || latestStatus.Status != scheduler.StatusSuccess {
		return nil
	}

	prevExecTime := d.PrevExecTime(ctx)
	if (latestStartedAt.After(prevExecTime) || latestStartedAt.Equal(prevExecTime)) &&
		latestStartedAt.Before(d.Next) {
		logger.Infof(ctx, "skipping the job because it has already run successfully at %s", latestStartedAt)
		return ErrJobSuccess
	}
	return nil
}

// PrevExecTime calculates the previous schedule time from 'Next' by subtracting
// the schedule duration between runs.
func (d *DAGRunJob) PrevExecTime(_ context.Context) time.Time {
	nextNextRunTime := d.Schedule.Next(d.Next.Add(time.Second))
	duration := nextNextRunTime.Sub(d.Next)
	return d.Next.Add(-duration)
}

// Stop halts a running job if it's currently running.
func (d *DAGRunJob) Stop(ctx context.Context) error {
	latestStatus, err := d.Client.GetLatestStatus(ctx, d.DAG)
	if err != nil {
		return err
	}
	if latestStatus.Status != scheduler.StatusRunning {
		return ErrJobIsNotRunning
	}
	return d.Client.Stop(ctx, d.DAG, "")
}

// Restart restarts the job unconditionally (quiet mode).
func (d *DAGRunJob) Restart(ctx context.Context) error {
	return d.Client.RestartDAG(ctx, d.DAG, history.RestartOptions{Quiet: true})
}

// String returns a string representation of the job, which is the DAG's name.
func (d *DAGRunJob) String() string {
	return d.DAG.Name
}
