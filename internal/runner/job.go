package runner

import (
	"errors"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"time"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

type Job interface {
	Start() error
	Stop() error
	Restart() error
	String() string
}

type job struct {
	DAG    *dag.DAG
	Config *config.Config
	Next   time.Time
}

var _ Job = (*job)(nil)

var (
	ErrJobRunning      = errors.New("job already running")
	ErrJobIsNotRunning = errors.New("job is not running")
	ErrJobFinished     = errors.New("job already finished")
)

func (j *job) Start() error {
	c := controller.New(j.DAG, jsondb.New())
	s, err := c.GetLastStatus()
	if err != nil {
		return err
	}
	switch s.Status {
	case scheduler.SchedulerStatus_Running:
		// already running
		return ErrJobRunning
	case scheduler.SchedulerStatus_None:
	default:
		// check the last execution time
		t, err := utils.ParseTime(s.StartedAt)
		if err == nil {
			t = t.Truncate(time.Second * 60)
			if t.After(j.Next) || j.Next.Equal(t) {
				return ErrJobFinished
			}
		}
		// should not be here
	}
	return c.Start(j.Config.Command, j.Config.WorkDir, "")
}

func (j *job) Stop() error {
	c := controller.New(j.DAG, jsondb.New())
	s, err := c.GetLastStatus()
	if err != nil {
		return err
	}
	if s.Status != scheduler.SchedulerStatus_Running {
		return ErrJobIsNotRunning
	}
	return c.Stop()
}

func (j *job) Restart() error {
	c := controller.New(j.DAG, jsondb.New())
	return c.Restart(j.Config.Command, j.Config.WorkDir)
}

func (j *job) String() string {
	return j.DAG.Name
}
