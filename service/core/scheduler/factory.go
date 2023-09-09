package scheduler

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/service/core/scheduler/job"
	"github.com/dagu-dev/dagu/service/core/scheduler/scheduler"
	"time"
)

type jobFactory struct {
	Command       string
	WorkDir       string
	EngineFactory engine.Factory
}

func (jf jobFactory) NewJob(dag *dag.DAG, next time.Time) scheduler.Job {
	return &job.Job{
		DAG:           dag,
		Command:       jf.Command,
		WorkDir:       jf.WorkDir,
		Next:          next,
		EngineFactory: jf.EngineFactory,
	}
}
