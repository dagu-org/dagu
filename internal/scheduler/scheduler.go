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
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
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
	hm           history.DAGRunManager
	er           EntryReader
	logDir       string
	stopChan     chan struct{}
	running      atomic.Bool
	location     *time.Location
	historyStore models.DAGRunStore
	queueStore   models.QueueStore
	procStore    models.ProcStore
	cancel       context.CancelFunc
	lock         sync.Mutex
	queueConfigs sync.Map
}

type queueConfig struct {
	MaxConcurrency int
}

func New(
	cfg *config.Config,
	er EntryReader,
	hm history.DAGRunManager,
	hs models.DAGRunStore,
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
		go s.handleQueue(ctx, queueCh, done)
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
	wg.Add(1)
	go func() {
		defer wg.Done()
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
			data := item.Data()
			logger.Info(ctx, "Received item from queue", "data", data)
			var (
				dag       *digraph.DAG
				attempt   models.DAGRunAttempt
				status    *models.DAGRunStatus
				err       error
				done      bool
				startedAt time.Time
			)

			alive, err := s.procStore.CountAlive(ctx, data.Name)
			if err != nil {
				logger.Error(ctx, "Failed to count alive processes", "err", err, "data", data)
				goto SEND_RESULT
			}

			if alive >= s.getQueueConfig(data.Name).MaxConcurrency {
				// If there are alive processes, skip this item
				goto SEND_RESULT
			}

			// Fetch the dag of the workflow
			attempt, err = s.historyStore.FindAttempt(ctx, data)
			if err != nil {
				logger.Error(ctx, "Failed to find run", "err", err, "data", data)
				goto SEND_RESULT
			}

			status, err = attempt.ReadStatus(ctx)
			if err != nil {
				logger.Error(ctx, "Failed to read status", "err", err, "data", data)
				goto SEND_RESULT
			}

			if status.Status != scheduler.StatusQueued {
				// If the status is not queued, skip this item
				logger.Info(ctx, "Skipping item from queue", "data", data, "status", status.Status)
				done = true
				goto SEND_RESULT
			}

			dag, err = attempt.ReadDAG(ctx)
			if err != nil {
				logger.Error(ctx, "Failed to read dag", "err", err, "data", data)
				goto SEND_RESULT
			}

			// Update the queue configuration with the latest execution
			s.queueConfigs.Store(data.Name, queueConfig{
				MaxConcurrency: max(dag.MaxActiveRuns, 1),
			})

			startedAt = time.Now()
			if err := s.hm.RetryDAGRun(ctx, dag, data.ID); err != nil {
				logger.Error(ctx, "Failed to retry dag", "err", err, "data", data)
				goto SEND_RESULT
			}

			// For now we need to wait for the DAG started
		WAIT_FOR_RUN:
			for {
				// Check if the dag is running
				attempt, err = s.historyStore.FindAttempt(ctx, data)
				if err != nil {
					logger.Error(ctx, "Failed to find run", "err", err, "data", data)
				}
				status, err := attempt.ReadStatus(ctx)
				if err != nil {
					logger.Error(ctx, "Failed to read status", "err", err, "data", data)
					goto SEND_RESULT
				}
				if status.Status != scheduler.StatusQueued {
					done = true
					break WAIT_FOR_RUN
				}
				if time.Since(startedAt) > 5*time.Second {
					logger.Error(ctx, "Timeout waiting for run to start", "data", data)
					break WAIT_FOR_RUN
				}
			}

			if done {
				logger.Info(ctx, "Successfully processed item from queue", "data", data)
			}

		SEND_RESULT:
			item.Result <- done
		}
	}
}

// SetQueueConfig sets the queue configuration for a specific queue.
func (s *Scheduler) getQueueConfig(name string) queueConfig {
	cfg, ok := s.queueConfigs.Load(name)
	if !ok {
		return queueConfig{MaxConcurrency: 1}
	}

	queueCfg, ok := cfg.(queueConfig)
	if !ok {
		return queueConfig{MaxConcurrency: 1}
	}

	return queueCfg
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
	if !s.running.CompareAndSwap(true, false) {
		return
	}

	close(s.stopChan)

	s.lock.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.lock.Unlock()

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
