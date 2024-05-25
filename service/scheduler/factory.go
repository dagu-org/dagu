package scheduler

import (
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/service/scheduler/job"
	"github.com/dagu-dev/dagu/service/scheduler/scheduler"
)

type jobFactory struct {
	Executable    string
	WorkDir       string
	EngineFactory engine.Factory
}

func (jf jobFactory) NewJob(d *dag.DAG, next time.Time) scheduler.Job {
	return &job.Job{
		DAG:           d,
		Executable:    jf.Executable,
		WorkDir:       jf.WorkDir,
		Next:          next,
		EngineFactory: jf.EngineFactory,
	}
}
