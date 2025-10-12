package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
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
	DAG         *core.DAG
	Next        time.Time
	Schedule    cron.Schedule
	Client      runtime.Manager
	DAGExecutor *DAGExecutor
}

// Start attempts to run the job if it is not already running and is ready.
func (j *DAGRunJob) Start(ctx context.Context) error {
	latestStatus, err := j.Client.GetLatestStatus(ctx, j.DAG)
	if err != nil {
		return err
	}

	// Guard against already running jobs.
	if latestStatus.Status == core.Running {
		return ErrJobRunning
	}

	// Check if the job is ready to start.
	if err := j.Ready(ctx, latestStatus); err != nil {
		return err
	}

	// Create a unique run ID for this scheduled execution
	runID, err := j.Client.GenDAGRunID(ctx)
	if err != nil {
		return err
	}

	// Handle the job execution (implements persistence-first for distributed execution)
	return j.DAGExecutor.HandleJob(ctx, j.DAG, coordinatorv1.Operation_OPERATION_START, runID)
}

// Ready checks whether the job can be safely started based on the latest status.
func (j *DAGRunJob) Ready(ctx context.Context, latestStatus execution.DAGRunStatus) error {
	// Prevent starting if it's already running.
	if latestStatus.Status == core.Running {
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
	if latestStartedAt.After(j.Next) || j.Next.Equal(latestStartedAt) {
		return ErrJobFinished
	}

	// Check if we should skip this run due to a prior successful run.
	return j.skipIfSuccessful(ctx, latestStatus, latestStartedAt)
}

// skipIfSuccessful checks if the DAG has already run successfully in the window since the last scheduled time.
// If so, the current run is skipped.
func (j *DAGRunJob) skipIfSuccessful(ctx context.Context, latestStatus execution.DAGRunStatus, latestStartedAt time.Time) error {
	// If skip is not configured, or the DAG is not currently successful, do nothing.
	if !j.DAG.SkipIfSuccessful || latestStatus.Status != core.Success {
		return nil
	}

	prevExecTime := j.PrevExecTime(ctx)
	if (latestStartedAt.After(prevExecTime) || latestStartedAt.Equal(prevExecTime)) &&
		latestStartedAt.Before(j.Next) {
		logger.Infof(ctx, "skipping the job because it has already run successfully at %s", latestStartedAt)
		return ErrJobSuccess
	}
	return nil
}

// PrevExecTime calculates the previous schedule time from 'Next' by subtracting
// the schedule duration between runs.
func (j *DAGRunJob) PrevExecTime(_ context.Context) time.Time {
	nextNextRunTime := j.Schedule.Next(j.Next.Add(time.Second))
	duration := nextNextRunTime.Sub(j.Next)
	return j.Next.Add(-duration)
}

// Stop halts a running job if it's currently running.
func (j *DAGRunJob) Stop(ctx context.Context) error {
	latestStatus, err := j.Client.GetLatestStatus(ctx, j.DAG)
	if err != nil {
		return err
	}
	if latestStatus.Status != core.Running {
		return ErrJobIsNotRunning
	}
	return j.Client.Stop(ctx, j.DAG, "")
}

// Restart restarts the job unconditionally.
func (j *DAGRunJob) Restart(ctx context.Context) error {
	return j.DAGExecutor.Restart(ctx, j.DAG)
}

// String returns a string representation of the job, which is the DAG's name.
func (j *DAGRunJob) String() string {
	return j.DAG.Name
}
