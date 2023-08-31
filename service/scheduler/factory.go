package scheduler

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/service/scheduler/entry"
	"github.com/dagu-dev/dagu/service/scheduler/job"
	"time"
)

type jobFactory struct {
	cfg *config.Config
}

func (jf jobFactory) NewJob(dag *dag.DAG, next time.Time) entry.Job {
	return &job.Job{
		DAG:    dag,
		Config: jf.cfg,
		Next:   next,
	}
}
