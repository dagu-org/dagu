package scheduler

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/service/scheduler/entry"
	"github.com/dagu-dev/dagu/service/scheduler/job"
	"time"
)

type jobFactory struct {
	Command string
	WorkDir string
}

func (jf jobFactory) NewJob(dag *dag.DAG, next time.Time) entry.Job {
	return &job.Job{
		DAG:     dag,
		Command: jf.Command,
		WorkDir: jf.WorkDir,
		Next:    next,
	}
}
