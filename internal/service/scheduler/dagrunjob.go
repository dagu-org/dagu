package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
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
// After the TickPlanner refactor, Start logic moved to the planner.
// DAGRunJob now only handles stop and restart operations.
type DAGRunJob struct {
	DAG         *core.DAG
	Next        time.Time
	Schedule    cron.Schedule
	Client      runtime.Manager
	DAGExecutor *DAGExecutor
}

// GetDAG returns the DAG associated with this job.
func (j *DAGRunJob) GetDAG(_ context.Context) *core.DAG {
	return j.DAG
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

// Ready checks whether the job can be safely started based on the latest status.
// Retained for backward compatibility with tests that exercise guard logic directly.
func (j *DAGRunJob) Ready(_ context.Context, latestStatus exec.DAGRunStatus) error {
	if latestStatus.Status == core.Running {
		return ErrJobRunning
	}
	return nil
}

// PrevExecTime calculates the previous schedule time from 'Next' by subtracting
// the schedule duration between runs.
// Retained for backward compatibility with tests.
func (j *DAGRunJob) PrevExecTime() time.Time {
	return computePrevExecTime(j.Next, core.Schedule{Parsed: j.Schedule})
}
