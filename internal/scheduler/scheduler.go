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
	"github.com/dagu-org/dagu/internal/history"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
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
	hm           history.Manager
	er           EntryReader
	logDir       string
	stopChan     chan struct{}
	running      atomic.Bool
	location     *time.Location
	historyStore models.HistoryStore
	queueStore   models.QueueStore
	procStore    models.ProcStore
	cancel       context.CancelFunc
	lock         sync.Mutex
}

func New(
	cfg *config.Config,
	er EntryReader,
	hm history.Manager,
	hs models.HistoryStore,
	qs models.QueueStore,
	ps models.ProcStore,
) *Scheduler {
	timeLoc := cfg.Global.Location
	if timeLoc == nil {
		timeLoc = time.Local
	}

	return &Scheduler{
		logDir:       cfg.Paths.LogDir,
		stopChan:     make(chan struct{}),
		location:     timeLoc,
		er:           er,
		hm:           hm,
		historyStore: hs,
		queueStore:   qs,
		procStore:    ps,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	sig := make(chan os.Signal, 1)

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	defer cancel()

	done := make(chan any)
	defer close(done)

	// Start the DAG file watcher
	if err := s.er.Start(ctx, done); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	// Start queue reader
	queueCh := make(chan models.QueuedItem, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		s.handleQueue(ctx, queueCh, done)
	}()

	qr := s.queueStore.Reader(ctx)
	if err := qr.Start(ctx, queueCh); err != nil {
		return fmt.Errorf("failed to start queue reader: %w", err)
	}

	// Handle OS signals for graceful shutdown
	signal.Notify(
		sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT,
	)

	// Go routine to handle OS signals and context cancellation
	go func() {
		select {
		case <-done:
			qr.Stop(ctx)
			s.Stop(ctx)
			return

		case <-sig:
			qr.Stop(ctx)
			s.Stop(ctx)

		case <-ctx.Done():
			qr.Stop(ctx)
			s.Stop(ctx)

		}
	}()

	logger.Info(ctx, "Scheduler started")

	// Start the scheduler loop (it blocks)
	s.start(ctx)

	wg.Wait()
	return nil
}

func (s *Scheduler) handleQueue(ctx context.Context, ch chan models.QueuedItem, done chan any) {
	for {
		select {
		case <-done:
			logger.Info(ctx, "Stopping queue handler due to manager shutdown")
			return

		case <-ctx.Done():
			logger.Info(ctx, "Stopping queue handler due to context cancellation")
			return

		case item := <-ch:
			if item == nil {
				logger.Info(ctx, "Received nil item from queue")
				continue
			}

			data := item.Data()
			logger.Info(ctx, "Received item from queue", "data", data)

			// Fetch the dag of the workflow
			history, err := s.historyStore.FindRun(ctx, data)
			if err != nil {
				logger.Error(ctx, "Failed to find run", "err", err, "data", data)
				continue
			}

			dag, err := history.ReadDAG(ctx)
			if err != nil {
				logger.Error(ctx, "Failed to read dag", "err", err, "data", data)
				continue
			}

			if err := s.hm.RetryDAG(ctx, dag, data.WorkflowID); err != nil {
				logger.Error(ctx, "Failed to retry dag", "err", err, "data", data)
				continue
			}

			logger.Info(ctx, "Successfully processed item from queue", "data", data)
		}
	}
}

// start starts the scheduler.
// It runs in a loop, checking for jobs to run every minute.
func (s *Scheduler) start(ctx context.Context) {
	t := Now().Truncate(time.Minute)
	timer := time.NewTimer(0)

	s.running.Store(true)

	for {
		select {
		case <-timer.C:
			s.run(ctx, t)
			t = s.NextTick(t)
			_ = timer.Stop()
			timer.Reset(t.Sub(Now()))

		case <-s.stopChan:
			if !timer.Stop() {
				<-timer.C
			}
			return

		}
	}
}

// NextTick returns the next tick time for the scheduler.
func (*Scheduler) NextTick(now time.Time) time.Time {
	return now.Add(time.Minute).Truncate(time.Second * 60)
}

// Stop stops the scheduler.
func (s *Scheduler) Stop(ctx context.Context) {
	if !s.running.Load() {
		return
	}

	if s.stopChan != nil {
		close(s.stopChan)
	}

	s.lock.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.lock.Unlock()

	s.running.Store(false)
	logger.Info(ctx, "Scheduler stopped")
}

// run executes the scheduled jobs at the current time.
func (s *Scheduler) run(ctx context.Context, now time.Time) {
	// Get jobs scheduled to run at or before the current time
	// Subtract a small buffer to avoid edge cases with exact timing
	jobs, err := s.er.Next(ctx, now.Add(-time.Second).In(s.location))
	if err != nil {
		logger.Error(ctx, "Failed to get next jobs", "err", err)
		return
	}

	// Sort the jobs by the next scheduled time for predictable execution order
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Next.Before(jobs[j].Next)
	})

	for _, job := range jobs {
		if job.Next.After(now) {
			break
		}

		// Create a child context for this specific job execution
		jobCtx := logger.WithValues(ctx,
			"jobType", job.Type.String(),
			"scheduledTime", job.Next.Format(time.RFC3339))

		// Launch job with bounded concurrency
		go func(ctx context.Context, job *ScheduledJob) {
			if err := job.invoke(ctx); err != nil {
				switch {
				case errors.Is(err, ErrJobFinished):
					logger.Info(ctx, "Job already completed", "job", job.Job)
				case errors.Is(err, ErrJobRunning):
					logger.Info(ctx, "Job already in progress", "job", job.Job)
				case errors.Is(err, ErrJobSkipped):
					logger.Info(ctx, "Job execution skipped", "job", job.Job, "reason", err.Error())
				default:
					logger.Error(ctx, "Job execution failed",
						"job", job.Job,
						"err", err,
						"errorType", fmt.Sprintf("%T", err))
				}
			} else {
				logger.Info(ctx, "Job completed successfully", "job", job.Job)
			}
		}(jobCtx, job)
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

var (
	// fixedTime is the fixed time used for testing.
	fixedTime     time.Time
	fixedTimeLock sync.RWMutex
)

// SetFixedTime sets the fixed time for testing.
func SetFixedTime(t time.Time) {
	fixedTimeLock.Lock()
	defer fixedTimeLock.Unlock()

	fixedTime = t
}

// Now returns the current time.
func Now() time.Time {
	fixedTimeLock.RLock()
	defer fixedTimeLock.RUnlock()

	if fixedTime.IsZero() {
		return time.Now()
	}

	return fixedTime
}
