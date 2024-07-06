package scheduler

import (
	"fmt"
	"os"
	"os/signal"
	"path"
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
	Start entryType = iota
	Stop
	Restart
)

func (e *entry) Invoke() error {
	if e.Job == nil {
		return nil
	}
	switch e.EntryType {
	case Start:
		e.Logger.Info(
			"start job",
			"job",
			e.Job.String(),
			"time",
			e.Next.Format("2006-01-02 15:04:05"),
		)
		return e.Job.Start()
	case Stop:
		e.Logger.Info(
			"stop job",
			"job",
			e.Job.String(),
			"time",
			e.Next.Format("2006-01-02 15:04:05"),
		)
		return e.Job.Stop()
	case Restart:
		e.Logger.Info(
			"restart job",
			"job",
			e.Job.String(),
			"time",
			e.Next.Format("2006-01-02 15:04:05"),
		)
		return e.Job.Restart()
	}
	return nil
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

func (s *Scheduler) Start() error {
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
		}
	}()

	s.logger.Info("starting scheduler")
	s.start()

	return nil
}

func (s *Scheduler) setupLogFile() (err error) {
	filename := path.Join(s.logDir, "scheduler.log")
	dir := path.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	s.logger.Info("setup log", "filename", filename)
	return err
}

func (s *Scheduler) start() {
	t := now().Truncate(time.Second * 60)
	timer := time.NewTimer(0)
	s.running.Store(true)
	for {
		select {
		case <-timer.C:
			s.run(t)
			t = s.nextTick(t)
			timer = time.NewTimer(t.Sub(now()))
		case <-s.stop:
			_ = timer.Stop()
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
		s.stop <- struct{}{}
	}
	s.running.Store(false)
}

var (
	fixedTime time.Time
	lock      sync.RWMutex
)

// setFixedTime sets the fixed time.
// This is used for testing.
func setFixedTime(t time.Time) {
	lock.Lock()
	defer lock.Unlock()
	fixedTime = t
}

// now returns the current time.
func now() time.Time {
	lock.RLock()
	defer lock.RUnlock()
	if fixedTime.IsZero() {
		return time.Now()
	}
	return fixedTime
}
