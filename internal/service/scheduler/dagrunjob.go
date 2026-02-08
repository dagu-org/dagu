package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
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
		logger.Error(ctx, "Failed to fetch latest DAG status",
			tag.DAG(j.DAG.Name),
			tag.Error(err),
		)
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
	runID, err := runtime.GenRunID()
	if err != nil {
		return err
	}

	// Pass j.Next as the scheduled time so live runs also record scheduledTime
	return j.DAGExecutor.HandleJob(ctx, j.DAG, coordinatorv1.Operation_OPERATION_START, runID, core.TriggerTypeScheduler, j.Next)
}

// Ready checks whether the job can be safely started based on the latest status.
func (j *DAGRunJob) Ready(ctx context.Context, latestStatus exec.DAGRunStatus) error {
	if latestStatus.Status == core.Running {
		return ErrJobRunning
	}

	ctx = logger.WithValues(ctx, tag.DAG(j.DAG.Name))

	latestStartedAt, err := stringutil.ParseTime(latestStatus.StartedAt)
	if err != nil {
		// If parsing fails, log and continue (don't skip).
		logger.Error(ctx, "Failed to parse the last successful run time", tag.Error(err))
		return nil
	}

	// Consider queued time as well, if available.
	if latestStatus.QueuedAt != "" {
		queuedAt, err := stringutil.ParseTime(latestStatus.QueuedAt)
		if err == nil && queuedAt.Before(latestStartedAt) {
			latestStartedAt = queuedAt
		}
	}

	// Skip if the last successful run time is on or after the next scheduled time.
	latestStartedAt = latestStartedAt.Truncate(time.Minute)
	if !latestStartedAt.Before(j.Next) {
		return ErrJobFinished
	}

	// Check if we should skip this run due to a prior successful run.
	return j.skipIfSuccessful(ctx, latestStatus, latestStartedAt)
}

// skipIfSuccessful checks if the DAG has already run successfully in the window since the last scheduled time.
// If so, the current run is skipped.
func (j *DAGRunJob) skipIfSuccessful(ctx context.Context, latestStatus exec.DAGRunStatus, latestStartedAt time.Time) error {
	// If skip is not configured, or the DAG is not currently successful, do nothing.
	if !j.DAG.SkipIfSuccessful || latestStatus.Status != core.Succeeded {
		return nil
	}

	prevExecTime := j.PrevExecTime()
	if !latestStartedAt.Before(prevExecTime) && latestStartedAt.Before(j.Next) {
		logger.Info(ctx, "Skipping job due to successful prior run",
			slog.String("start-time", latestStartedAt.Format(time.RFC3339)))
		return ErrJobSuccess
	}
	return nil
}

// PrevExecTime calculates the previous schedule time from 'Next' by subtracting
// the schedule duration between runs.
func (j *DAGRunJob) PrevExecTime() time.Time {
	nextNextRunTime := j.Schedule.Next(j.Next.Add(time.Second))
	scheduleDuration := nextNextRunTime.Sub(j.Next)
	return j.Next.Add(-scheduleDuration)
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
