// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/logger"
	dsclient "github.com/dagu-org/dagu/internal/persistence/client"
	"github.com/dagu-org/dagu/internal/persistence/model"
)

type Scheduler struct {
	entryReader entryReader
	queueReader queueReader
	logDir      string
	stop        chan struct{}
	running     atomic.Bool
	logger      logger.Logger
}

func New(cfg *config.Config, lg logger.Logger, cli client.Client) *Scheduler {
	dataStore := dsclient.NewDataStores(
		cfg.DAGs,
		cfg.DataDir,
		cfg.QueueDir,
		cfg.StatsDir,
		cfg.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: cfg.LatestStatusToday,
		},
	)

	return newScheduler(newSchedulerArgs{
		EntryReader: newEntryReader(newEntryReaderArgs{
			Client:  cli,
			DagsDir: cfg.DAGs,
			JobCreator: &jobCreatorImpl{
				WorkDir:    cfg.WorkDir,
				Client:     cli,
				Executable: cfg.Executable,
			},
			Logger: lg,
		}),
		QueueReader: newQueueReader(newQueueReaderArgs{
			QueueDir:  cfg.QueueDir,
			Logger:    lg,
			DataStore: dataStore,
		}),
		Logger: lg,
		LogDir: cfg.LogDir,
	})
}

type entryReader interface {
	Start(done chan any)
	Read(now time.Time) ([]*entry, error)
}

type queueReader interface {
	Start(done chan any)
	ReadFileQueue() ([]*model.Queue, error)
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
		return "Start"
	case entryTypeStop:
		return "Stop"
	case entryTypeRestart:
		return "Restart"
	default:
		return "Unknown"
	}
}

func (e *entry) Invoke() error {
	if e.Job == nil {
		return nil
	}

	e.Logger.Info(
		"Workflow operation started",
		"operation", e.EntryType.String(),
		"workflow", e.Job.String(),
		"next", e.Next.Format(time.RFC3339),
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
	QueueReader queueReader
	Logger      logger.Logger
	LogDir      string
}

func newScheduler(args newSchedulerArgs) *Scheduler {
	return &Scheduler{
		entryReader: args.EntryReader,
		queueReader: args.QueueReader,

		logDir: args.LogDir,
		stop:   make(chan struct{}),
		logger: args.Logger,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	sig := make(chan os.Signal, 1)
	done := make(chan any)
	defer close(done)
	queuesig := make(chan any)

	s.entryReader.Start(done)
	s.queueReader.Start(queuesig)

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
	if err != nil {
		s.logger.Error("Scheduler failed to read workflow entries", "error", err)
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
			if err := e.Invoke(); err != nil {
				if errors.Is(err, errJobFinished) {
					s.logger.Info("Workflow is already finished", "workflow", e.Job)
				} else if errors.Is(err, errJobRunning) {
					s.logger.Info("Workflow is already running", "workflow", e.Job)
				} else {
					s.logger.Error(
						"Workflow execution failed",
						"workflow", e.Job,
						"operation", e.EntryType.String(),
						"error", err,
					)
				}
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
