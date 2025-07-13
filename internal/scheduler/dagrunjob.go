package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	coordinatorclient "github.com/dagu-org/dagu/internal/coordinator/client"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/stringutil"
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
	DAG                      *digraph.DAG
	Executable               string
	WorkDir                  string
	Next                     time.Time
	Schedule                 cron.Schedule
	Client                   dagrun.Manager
	CoordinatorClientFactory *coordinatorclient.Factory
	coordinatorClient        coordinatorclient.Client
	coordinatorClientMu      sync.Mutex
}

// GetDAG returns the DAG associated with this job.
func (j *DAGRunJob) GetDAG(_ context.Context) *digraph.DAG {
	return j.DAG
}

// Start attempts to run the job if it is not already running and is ready.
func (j *DAGRunJob) Start(ctx context.Context) error {
	latestStatus, err := j.Client.GetLatestStatus(ctx, j.DAG)
	if err != nil {
		return err
	}

	// Guard against already running jobs.
	if latestStatus.Status == scheduler.StatusRunning {
		return ErrJobRunning
	}

	// Check if the job is ready to start.
	if err := j.Ready(ctx, latestStatus); err != nil {
		return err
	}

	// Check if we should use distributed execution
	if j.shouldUseDistributedExecution() {
		// Create a unique run ID for this scheduled execution
		runID, err := j.Client.GenDAGRunID(ctx)
		if err != nil {
			return err
		}

		// Create task for coordinator
		task := j.DAG.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			runID,
			digraph.WithWorkerSelector(j.DAG.WorkerSelector),
		)

		return j.dispatchToCoordinator(ctx, task)
	}

	// Job is ready; proceed to start locally.
	return j.Client.StartDAGRunAsync(ctx, j.DAG, dagrun.StartOptions{Quiet: true})
}

// Ready checks whether the job can be safely started based on the latest status.
func (j *DAGRunJob) Ready(ctx context.Context, latestStatus models.DAGRunStatus) error {
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
	if latestStartedAt.After(j.Next) || j.Next.Equal(latestStartedAt) {
		return ErrJobFinished
	}

	// Check if we should skip this run due to a prior successful run.
	return j.skipIfSuccessful(ctx, latestStatus, latestStartedAt)
}

// skipIfSuccessful checks if the DAG has already run successfully in the window since the last scheduled time.
// If so, the current run is skipped.
func (j *DAGRunJob) skipIfSuccessful(ctx context.Context, latestStatus models.DAGRunStatus, latestStartedAt time.Time) error {
	// If skip is not configured, or the DAG is not currently successful, do nothing.
	if !j.DAG.SkipIfSuccessful || latestStatus.Status != scheduler.StatusSuccess {
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
	if latestStatus.Status != scheduler.StatusRunning {
		return ErrJobIsNotRunning
	}
	return j.Client.Stop(ctx, j.DAG, "")
}

// Restart restarts the job unconditionally (quiet mode).
func (j *DAGRunJob) Restart(ctx context.Context) error {
	return j.Client.RestartDAG(ctx, j.DAG, dagrun.RestartOptions{Quiet: true})
}

// String returns a string representation of the job, which is the DAG's name.
func (j *DAGRunJob) String() string {
	return j.DAG.Name
}

// shouldUseDistributedExecution checks if distributed execution should be used.
// Returns true only if coordinator is configured AND the DAG has workerSelector labels.
func (j *DAGRunJob) shouldUseDistributedExecution() bool {
	return j.CoordinatorClientFactory != nil && j.DAG != nil && len(j.DAG.WorkerSelector) > 0
}

// getCoordinatorClient returns the coordinator client, creating it lazily if needed.
// Returns nil if no coordinator is configured.
func (j *DAGRunJob) getCoordinatorClient(ctx context.Context) (coordinatorclient.Client, error) {
	// If no factory configured, distributed execution is disabled
	if j.CoordinatorClientFactory == nil {
		return nil, nil
	}

	// Check if client already exists
	j.coordinatorClientMu.Lock()
	defer j.coordinatorClientMu.Unlock()

	if j.coordinatorClient != nil {
		return j.coordinatorClient, nil
	}

	// Create client
	client, err := j.CoordinatorClientFactory.Build(ctx)
	if err != nil {
		return nil, err
	}

	j.coordinatorClient = client
	logger.Info(ctx, "Coordinator client initialized for distributed execution in DAGRunJob")
	return client, nil
}

// dispatchToCoordinator dispatches a task to the coordinator for distributed execution.
func (j *DAGRunJob) dispatchToCoordinator(ctx context.Context, task *coordinatorv1.Task) error {
	client, err := j.getCoordinatorClient(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to get coordinator client", "err", err)
		return err
	}

	if client == nil {
		// Should not happen if shouldUseDistributedExecution is checked
		return errors.New("coordinator client not available")
	}

	err = client.Dispatch(ctx, task)
	if err != nil {
		logger.Error(ctx, "Failed to dispatch task", "err", err)
		return err
	}

	logger.Info(ctx, "Task dispatched to coordinator",
		"target", task.Target,
		"runID", task.DagRunId,
		"operation", task.Operation.String())

	return nil
}
