package runner

import (
	"errors"
	"time"

	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

type Job interface {
	Run() error
	String() string
}

type job struct {
	DAG       *config.Config
	Config    *admin.Config
	StartTime time.Time
}

var _ Job = (*job)(nil)

var (
	ErrJobRunning  = errors.New("job already running")
	ErrJobFinished = errors.New("job already finished")
)

func (j *job) Run() error {
	c := controller.New(j.DAG)
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
			if t.After(j.StartTime) || j.StartTime.Equal(t) {
				return ErrJobFinished
			}
		}
		// should not be here
	}
	return c.Start(j.Config.Command, j.Config.WorkDir, "")
}

func (j *job) String() string {
	return j.DAG.Name
}
