package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/util"
)

type Scheduler struct {
	entryReader entryReader
	logDir      string
	stop        chan struct{}
	running     atomic.Bool
	logger      logger.Logger
}

type entryReader interface {
	Start(done chan any)
	Read(now time.Time) ([]*entry, error)
}

type entry struct {
	Next      time.Time
	Job       job
	EntryType entryType
	Logger    logger.Logger
}

type job interface {
	GetDAG() *dag.DAG
	Start() error
	Stop() error
	Restart() error
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
		return "start"
	case entryTypeStop:
		return "stop"
	case entryTypeRestart:
		return "restart"
	default:
		return "unknown"
	}
}

func (e *entry) Invoke() error {
	if e.Job == nil {
		return nil
	}

	logMsg := fmt.Sprintf("%s job", e.EntryType)
	e.Logger.Info(logMsg,
		"job", e.Job.String(),
		"time", e.Next.Format(time.RFC3339),
	)

	switch e.EntryType {
	case entryTypeStart:
		return e.Job.Start()
	case entryTypeStop:
		return e.Job.Stop()
	case entryTypeRestart:
		return e.Job.Restart()
	default:
		return fmt.Errorf("unknown entry type: %v", e.EntryType)
	}
}

type newSchedulerArgs struct {
	EntryReader entryReader
	Logger      logger.Logger
	LogDir      string
}

func newScheduler(args newSchedulerArgs) *Scheduler {
	return &Scheduler{
		entryReader: args.EntryReader,
		logDir:      args.LogDir,
		stop:        make(chan struct{}),
		logger:      args.Logger,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	if err := s.setupLogFile(); err != nil {
		return fmt.Errorf("setup log file: %w", err)
	}

	sig := make(chan os.Signal, 1)
	done := make(chan any)
	defer close(done)

	s.entryReader.Start(done)

	signal.Notify(
		sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT,
	)

	go func() {
		select {
		case <-done:
			return
		case <-sig:
			s.Stop()
		case <-ctx.Done():
			s.Stop()
		}
	}()

	s.start()

	return nil
}

func (s *Scheduler) setupLogFile() error {
	filename := filepath.Join(s.logDir, "scheduler.log")
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	s.logger.Info("Scheduler log file", "file", filename)
	return nil
}

func (s *Scheduler) start() {
	// TODO: refactor this to use a ticker
	t := now().Truncate(time.Minute)
	timer := time.NewTimer(0)

	s.running.Store(true)
	for {
		select {
		case <-timer.C:
			s.run(t)
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

func (s *Scheduler) run(now time.Time) {
	entries, err := s.entryReader.Read(now.Add(-time.Second))
	util.LogErr("failed to read entries", err)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Next.Before(entries[j].Next)
	})
	for _, e := range entries {
		t := e.Next
		if t.After(now) {
			break
		}
		go func(e *entry) {
			err := e.Invoke()
			if err != nil {
				s.logger.Error(
					"failed to invoke entryreader", "entryreader",
					e.Job,
					"error",
					err,
				)
			}
		}(e)
	}
}

func (*Scheduler) nextTick(now time.Time) time.Time {
	return now.Add(time.Minute).Truncate(time.Second * 60)
}

func (s *Scheduler) Stop() {
	if !s.running.Load() {
		return
	}
	if s.stop != nil {
		close(s.stop)
	}
	s.running.Store(false)
	s.logger.Info("Scheduler stopped")
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
