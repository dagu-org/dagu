package scheduler

import (
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/service/core/scheduler/job"
	"github.com/dagu-dev/dagu/service/core/scheduler/scheduler"
)

type jobFactory struct {
	Command       string
	WorkDir       string
	EngineFactory engine.Factory
}

func (jf jobFactory) NewJob(d *dag.DAG, next time.Time) scheduler.Job {
	return &job.Job{
		DAG:           d,
		Command:       jf.Command,
		WorkDir:       jf.WorkDir,
		Next:          next,
		EngineFactory: jf.EngineFactory,
	}
}
