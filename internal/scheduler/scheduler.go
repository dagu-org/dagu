package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/logger"
)

// Job is the interface for the actual DAG.
type Job interface {
	// Start starts the DAG.
	Start(ctx context.Context) error
	// Stop stops the DAG.
	Stop(ctx context.Context) error
	// Restart restarts the DAG.
	Restart(ctx context.Context) error
}

type Scheduler struct {
	manager  JobManager
	logDir   string
	stopChan chan struct{}
	running  atomic.Bool
	location *time.Location
}

func New(cfg *config.Config, manager JobManager) *Scheduler {
	timeLoc := cfg.Location
	if timeLoc == nil {
		timeLoc = time.Local
	}

	return &Scheduler{
		logDir:   cfg.Paths.LogDir,
		stopChan: make(chan struct{}),
		location: timeLoc,
		manager:  manager,
	}
}

// ScheduleType is the type of schedule (start, stop, restart).
type ScheduleType int

const (
	ScheduleTypeStart ScheduleType = iota
	ScheduleTypeStop
	ScheduleTypeRestart
)

func (s ScheduleType) String() string {
	switch s {
	case ScheduleTypeStart:
		return "Start"

	case ScheduleTypeStop:
		return "Stop"

	case ScheduleTypeRestart:
		return "Restart"

	default:
		// Should never happen.
		return "Unknown"

	}
}

// invoke invokes the job based on the schedule type.
func (s *ScheduledJob) invoke(ctx context.Context) error {
	if s.Job == nil {
		logger.Error(ctx, "job is nil", "job", s.Job)
		return nil
	}

	logger.Info(ctx, "starting operation", "type", s.Type.String(), "job", s.Job)

	switch s.Type {
	case ScheduleTypeStart:
		return s.Job.Start(ctx)

	case ScheduleTypeStop:
		return s.Job.Stop(ctx)

	case ScheduleTypeRestart:
		return s.Job.Restart(ctx)

	default:
		return fmt.Errorf("unknown schedule type: %v", s.Type)

	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	sig := make(chan os.Signal, 1)

	done := make(chan any)
	defer close(done)

	if err := s.manager.Start(ctx, done); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	signal.Notify(
		sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT,
	)

	go func() {
		select {
		case <-done:
			return

		case <-sig:
			s.Stop(ctx)

		case <-ctx.Done():
			s.Stop(ctx)

		}
	}()

	logger.Info(ctx, "Scheduler started")
	s.start(ctx)

	return nil
}

func (s *Scheduler) start(ctx context.Context) {
	t := now().Truncate(time.Minute)
	timer := time.NewTimer(0)

	s.running.Store(true)

	for {
		select {
		case <-timer.C:
			s.run(ctx, t)
			t = s.nextTick(t)
			_ = timer.Stop()
			timer.Reset(t.Sub(now()))

		case <-s.stopChan:
			if !timer.Stop() {
				<-timer.C
			}
			return

		}
	}
}

func (s *Scheduler) run(ctx context.Context, now time.Time) {
	jobs, err := s.manager.Next(ctx, now.Add(-time.Second).In(s.location))
	if err != nil {
		logger.Error(ctx, "failed to get next jobs", "err", err)
		return
	}

	// Sort the jobs by the next scheduled time.
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Next.Before(jobs[j].Next)
	})

	for _, job := range jobs {
		if job.Next.After(now) {
			break
		}

		go func(job *ScheduledJob) {
			if err := job.invoke(ctx); err != nil {
				if errors.Is(err, ErrJobFinished) {
					logger.Info(ctx, "job is already finished", "job", job.Job, "err", err)
				} else if errors.Is(err, ErrJobRunning) {
					logger.Info(ctx, "job is already running", "job", job.Job, "err", err)
				} else if errors.Is(err, ErrJobSkipped) {
					logger.Info(ctx, "job is skipped", "job", job.Job, "err", err)
				} else {
					logger.Error(ctx, "job failed", "job", job.Job, "err", err)
				}
			}
		}(job)
	}
}

func (*Scheduler) nextTick(now time.Time) time.Time {
	return now.Add(time.Minute).Truncate(time.Second * 60)
}

func (s *Scheduler) Stop(ctx context.Context) {
	if !s.running.Load() {
		return
	}

	if s.stopChan != nil {
		close(s.stopChan)
	}

	s.running.Store(false)
	logger.Info(ctx, "Scheduler stopped")
}

var (
	// fixedTime is the fixed time used for testing.
	fixedTime     time.Time
	fixedTimeLock sync.RWMutex
)

// setFixedTime sets the fixed time for testing.
func setFixedTime(t time.Time) {
	fixedTimeLock.Lock()
	defer fixedTimeLock.Unlock()

	fixedTime = t
}

// now returns the current time.
func now() time.Time {
	fixedTimeLock.RLock()
	defer fixedTimeLock.RUnlock()

	if fixedTime.IsZero() {
		return time.Now()
	}

	return fixedTime
}
