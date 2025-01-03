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

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
)

type Scheduler struct {
	entryReader entryReader
	logDir      string
	stop        chan struct{}
	running     atomic.Bool
	location    *time.Location
}

// TODO: refactor to remove ctx from the constructor
func New(cfg *config.Config, cli client.Client) *Scheduler {
	jobCreator := &jobCreatorImpl{
		WorkDir:    cfg.WorkDir,
		Client:     cli,
		Executable: cfg.Paths.Executable,
	}
	entryReader := newEntryReader(cfg.Paths.DAGsDir, jobCreator, cli)
	return newScheduler(entryReader, cfg.Paths.LogDir, cfg.Location)
}

type entryReader interface {
	Start(ctx context.Context, done chan any) error
	Read(ctx context.Context, now time.Time) ([]*entry, error)
}

type entry struct {
	Next      time.Time
	Job       job
	EntryType entryType
}

type job interface {
	GetDAG(ctx context.Context) *digraph.DAG
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	String() string
}

type entryType int

const (
	entryTypeStart entryType = iota
	entryTypeStop
	entryTypeRestart
)

func (e entryType) String() string {
	switch e {
	case entryTypeStart:
		return "Start"
	case entryTypeStop:
		return "Stop"
	case entryTypeRestart:
		return "Restart"
	default:
		return "Unknown"
	}
}

func (e *entry) Invoke(ctx context.Context) error {
	if e.Job == nil {
		return nil
	}

	logger.Info(ctx, "DAG operation started", "operation", e.EntryType.String(), "DAG", e.Job.String(), "next", e.Next.Format(time.RFC3339))

	switch e.EntryType {
	case entryTypeStart:
		return e.Job.Start(ctx)
	case entryTypeStop:
		return e.Job.Stop(ctx)
	case entryTypeRestart:
		return e.Job.Restart(ctx)
	default:
		return fmt.Errorf("unknown entry type: %v", e.EntryType)
	}
}

func newScheduler(entryReader entryReader, logDir string, location *time.Location) *Scheduler {
	if location == nil {
		location = time.Local
	}
	return &Scheduler{
		entryReader: entryReader,
		logDir:      logDir,
		stop:        make(chan struct{}),
		location:    location,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	sig := make(chan os.Signal, 1)
	done := make(chan any)
	defer close(done)

	if err := s.entryReader.Start(ctx, done); err != nil {
		return fmt.Errorf("failed to start entry reader: %w", err)
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
	// TODO: refactor this to use a ticker
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
		case <-s.stop:
			if !timer.Stop() {
				<-timer.C
			}
			return
		}
	}
}

func (s *Scheduler) run(ctx context.Context, now time.Time) {
	entries, err := s.entryReader.Read(ctx, now.Add(-time.Second).In(s.location))
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
		go func(e *entry) {
			if err := e.Invoke(ctx); err != nil {
				if errors.Is(err, errJobFinished) {
					logger.Info(ctx, "DAG is already finished", "DAG", e.Job, "err", err)
				} else if errors.Is(err, errJobRunning) {
					logger.Info(ctx, "DAG is already running", "DAG", e.Job, "err", err)
				} else if errors.Is(err, errJobSkipped) {
					logger.Info(ctx, "DAG is skipped", "DAG", e.Job, "err", err)
				} else {
					logger.Error(ctx, "DAG execution failed", "DAG", e.Job, "operation", e.EntryType.String(), "err", err)
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
	if s.stop != nil {
		close(s.stop)
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
