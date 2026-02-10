package scheduler

import (
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/robfig/cron/v3"
)

// DAGRunJob represents a DAG with its scheduling metadata.
type DAGRunJob struct {
	DAG      *core.DAG
	Next     time.Time
	Schedule cron.Schedule
}

// String returns the DAG's name.
func (j *DAGRunJob) String() string {
	return j.DAG.Name
}

// PrevExecTime calculates the previous schedule time from 'Next' by subtracting
// the schedule duration between runs.
func (j *DAGRunJob) PrevExecTime() time.Time {
	return computePrevExecTime(j.Next, core.Schedule{Parsed: j.Schedule})
}
