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
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
)

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

type Job interface {
	GetDAG(ctx context.Context) *digraph.DAG
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	String() string
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
		return "Unknown"

	}
}

func (s *ScheduledJob) Invoke(ctx context.Context) error {
	if s.Job == nil {
		return nil
	}

	logger.Info(ctx, "DAG operation started", "operation", s.Type.String(), "DAG", s.Job.String(), "next", s.Next.Format(time.RFC3339))

	switch s.Type {
	case ScheduleTypeStart:
		return s.Job.Start(ctx)

	case ScheduleTypeStop:
		return s.Job.Stop(ctx)

	case ScheduleTypeRestart:
		return s.Job.Restart(ctx)

	default:
		return fmt.Errorf("unknown entry type: %v", s.Type)

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
	entries, err := s.manager.Next(ctx, now.Add(-time.Second).In(s.location))
	if err != nil {
		logger.Error(ctx, "Scheduler failed to read DAG entries", "err", err)
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Next.Before(entries[j].Next)
	})
	for _, e := range entries {
		t := e.Next
		if t.After(now) {
			break
		}
		go func(e *ScheduledJob) {
			if err := e.Invoke(ctx); err != nil {
				if errors.Is(err, ErrJobFinished) {
					logger.Info(ctx, "DAG is already finished", "DAG", e.Job, "err", err)
				} else if errors.Is(err, ErrJobRunning) {
					logger.Info(ctx, "DAG is already running", "DAG", e.Job, "err", err)
				} else if errors.Is(err, ErrJobSkipped) {
					logger.Info(ctx, "DAG is skipped", "DAG", e.Job, "err", err)
				} else {
					logger.Error(ctx, "DAG execution failed", "DAG", e.Job, "operation", e.Type.String(), "err", err)
				}
			}
		}(e)
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
	fixedTime time.Time
	timeLock  sync.RWMutex
)

// setFixedTime sets the fixed time.
// This is used for testing.
func setFixedTime(t time.Time) {
	timeLock.Lock()
	defer timeLock.Unlock()
	fixedTime = t
}

// now returns the current time.
func now() time.Time {
	timeLock.RLock()
	defer timeLock.RUnlock()
	if fixedTime.IsZero() {
		return time.Now()
	}
	return fixedTime
}
