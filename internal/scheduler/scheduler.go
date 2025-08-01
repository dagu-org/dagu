package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	dagstatus "github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/persistence/dirlock"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
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
	hm                  dagrun.Manager
	er                  EntryReader
	logDir              string
	stopChan            chan struct{}
	running             atomic.Bool
	location            *time.Location
	dagRunStore         models.DAGRunStore
	queueStore          models.QueueStore
	procStore           models.ProcStore
	cancel              context.CancelFunc
	lock                sync.Mutex
	queueConfigs        sync.Map
	config              *config.Config
	dirLock             dirlock.DirLock // File-based lock to prevent multiple scheduler instances
	dagExecutor         *DAGExecutor
	healthServer        *HealthServer // Health check server for monitoring
	disableHealthServer bool          // Disable health server when running from start-all
	heartbeatCancel     context.CancelFunc
	heartbeatDone       chan struct{}
}

type queueConfig struct {
	MaxConcurrency int
}

func New(
	cfg *config.Config,
	er EntryReader,
	drm dagrun.Manager,
	drs models.DAGRunStore,
	qs models.QueueStore,
	ps models.ProcStore,
	_ models.ServiceMonitor, // Currently unused but kept for API compatibility
	coordinatorCli digraph.Dispatcher,
) (*Scheduler, error) {
	timeLoc := cfg.Global.Location
	if timeLoc == nil {
		timeLoc = time.Local
	}

	dirLock := dirlock.New(filepath.Join(cfg.Paths.DataDir, "scheduler", "locks"),
		&dirlock.LockOptions{
			StaleThreshold: cfg.Scheduler.LockStaleThreshold,
			RetryInterval:  cfg.Scheduler.LockRetryInterval,
		})

	// Create DAG executor
	dagExecutor := NewDAGExecutor(coordinatorCli, drm)

	// Create health server
	healthServer := NewHealthServer(cfg.Scheduler.Port)

	return &Scheduler{
		logDir:       cfg.Paths.LogDir,
		stopChan:     make(chan struct{}),
		location:     timeLoc,
		er:           er,
		hm:           drm,
		dagRunStore:  drs,
		queueStore:   qs,
		procStore:    ps,
		config:       cfg,
		dirLock:      dirLock,
		dagExecutor:  dagExecutor,
		healthServer: healthServer,
	}, nil
}

// DisableHealthServer disables the health check server (used when running from start-all)
func (s *Scheduler) DisableHealthServer() {
	s.disableHealthServer = true
}

func (s *Scheduler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	defer cancel()

	// Start health check server only if not disabled
	if !s.disableHealthServer {
		if err := s.healthServer.Start(ctx); err != nil {
			return fmt.Errorf("failed to start health check server: %w", err)
		}
	}

	// Acquire directory lock first to prevent multiple scheduler instances
	logger.Info(ctx, "Waiting to acquire scheduler lock")
	if err := s.dirLock.Lock(ctx); err != nil {
		return fmt.Errorf("failed to acquire scheduler lock: %w", err)
	}

	logger.Info(ctx, "Acquired scheduler lock")

	// Ensure lock is always released
	defer func() {
		if err := s.dirLock.Unlock(); err != nil {
			logger.Error(ctx, "Failed to release scheduler lock in defer", "err", err)
		} else {
			logger.Info(ctx, "Released scheduler lock in defer")
		}
	}()

	sig := make(chan os.Signal, 1)

	// Set scheduler as running
	setSchedulerRunning(true)
	defer setSchedulerRunning(false)

	done := make(chan any)
	defer close(done)

	// Start the DAG file watcher
	if err := s.er.Start(ctx, done); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	// Start queue reader only if queues are enabled
	var qr models.QueueReader
	var wgQueue sync.WaitGroup
	if s.config.Queues.Enabled {
		queueCh := make(chan models.QueuedItem, 1)
		wgQueue.Add(1)

		go func() {
			defer wgQueue.Done()
			s.handleQueue(ctx, queueCh, done)
		}()

		qr = s.queueStore.Reader(ctx)
		if err := qr.Start(ctx, queueCh); err != nil {
			return fmt.Errorf("failed to start queue reader: %w", err)
		}
	}

	// Handle OS signals for graceful shutdown
	signal.Notify(
		sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT,
	)

	// Start heartbeat for the scheduler lock with its own context
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	s.heartbeatCancel = heartbeatCancel
	s.heartbeatDone = make(chan struct{})

	go func() {
		defer close(s.heartbeatDone)
		s.startHeartbeat(heartbeatCtx)
	}()

	// Go routine to handle OS signals and context cancellation
	wgQueue.Add(1)
	go func() {
		defer wgQueue.Done()
		select {
		case <-done:
			if qr != nil {
				qr.Stop(ctx)
			}
			s.Stop(ctx)
			return

		case <-sig:
			if qr != nil {
				qr.Stop(ctx)
			}
			s.Stop(ctx)

		case <-ctx.Done():
			if qr != nil {
				qr.Stop(ctx)
			}
			s.Stop(ctx)

		}
	}()

	logger.Info(ctx, "Scheduler started")

	// Start the scheduler loop (it blocks)
	s.start(ctx)

	wgQueue.Wait()

	return nil
}

func (s *Scheduler) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 7)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.dirLock.Heartbeat(ctx); err != nil {
				logger.Error(ctx, "Failed to send heartbeat for scheduler lock", "err", err)
			}

		case <-ctx.Done():
			ticker.Stop()
			logger.Info(ctx, "Stopping scheduler heartbeat due to context cancellation")
			return
		}
	}
}

// handleQueue processes queued DAG runs from the persistence layer.
// This is the second phase of the persistence-first architecture:
// - Phase 1: Scheduled jobs are enqueued by HandleJob (status=QUEUED)
// - Phase 2: This handler picks up queued items and executes/dispatches them
//
// The handler uses OPERATION_RETRY for all executions, which means "retry the dispatch"
// rather than "retry a failed execution". This allows the system to retry dispatching
// to the coordinator if it was temporarily unavailable.
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
				result    = models.QueuedItemProcessingResultRetry
				startedAt time.Time
				queueName string
				queueCfg  queueConfig
				alive     int
			)

			// Fetch the DAG of the dag-run attempt first to get queue configuration
			attempt, err = s.dagRunStore.FindAttempt(ctx, data)
			if err != nil {
				logger.Error(ctx, "Failed to find run", "err", err, "data", data)
				// If the attempt doesn't exist at all, mark as discard
				if errors.Is(err, models.ErrDAGRunIDNotFound) {
					logger.Error(ctx, "DAG run not found, marking as discard", "data", data)
					result = models.QueuedItemProcessingResultDiscard
				}
				goto SEND_RESULT
			}

			if attempt.Hidden() {
				logger.Info(ctx, "DAG run is hidden, marking as discard", "data", data)
				result = models.QueuedItemProcessingResultDiscard
				goto SEND_RESULT
			}

			dag, err = attempt.ReadDAG(ctx)
			if err != nil {
				logger.Error(ctx, "Failed to read dag", "err", err, "data", data)
				goto SEND_RESULT
			}

			alive, err = s.procStore.CountAlive(ctx, dag.QueueProcName())
			if err != nil {
				logger.Error(ctx, "Failed to count alive processes", "err", err, "data", data)
				goto SEND_RESULT
			}

			status, err = attempt.ReadStatus(ctx)
			if err != nil {
				if errors.Is(err, models.ErrCorruptedStatusFile) {
					logger.Error(ctx, "Status file is corrupted, marking as invalid", "err", err, "data", data)
					result = models.QueuedItemProcessingResultDiscard
				} else {
					logger.Error(ctx, "Failed to read status", "err", err, "data", data)
				}
				goto SEND_RESULT
			}

			if status.Status != dagstatus.Queued {
				logger.Info(ctx, "Skipping item from queue", "data", data, "status", status.Status)
				result = models.QueuedItemProcessingResultDiscard
				goto SEND_RESULT
			}

			// Determine the queue name for this DAG
			queueName = s.getQueueNameForDAG(dag)

			// Check concurrency limits based on queue configuration
			queueCfg = s.getQueueConfigByName(queueName, dag)
			if alive >= queueCfg.MaxConcurrency {
				logger.Info(ctx, "Queue concurrency limit reached", "queue", queueName, "limit", queueCfg.MaxConcurrency, "alive", alive)
				goto SEND_RESULT
			}

			// Update the queue configuration with the latest execution
			s.queueConfigs.Store(data.Name, queueConfig{
				MaxConcurrency: max(dag.MaxActiveRuns, 1),
			})

			startedAt = time.Now()

			// Execute the DAG that was previously enqueued.
			// IMPORTANT: We use OPERATION_RETRY here, which means "retry the dispatch", not "retry a failed execution".
			// This is part of the persistence-first approach:
			// 1. Scheduled jobs (via HandleJob) enqueue with status=QUEUED
			// 2. Queue handler (here) picks up and dispatches with OPERATION_RETRY
			// 3. For distributed execution, this dispatches to coordinator
			// 4. For local execution, this runs the DAG
			// The RETRY operation ensures the queue handler can retry dispatch if coordinator is temporarily down.
			if err := s.dagExecutor.ExecuteDAG(ctx, dag, coordinatorv1.Operation_OPERATION_RETRY, data.ID); err != nil {
				logger.Error(ctx, "Failed to execute dag", "err", err, "data", data)
				goto SEND_RESULT
			}

			// For now we need to wait for the DAG started
		WAIT_FOR_RUN:
			for {
				// Check if the dag is running
				attempt, err = s.dagRunStore.FindAttempt(ctx, data)
				if err != nil {
					logger.Error(ctx, "Failed to find run", "err", err, "data", data)
					continue
				}
				status, err := attempt.ReadStatus(ctx)
				if err != nil {
					if errors.Is(err, models.ErrCorruptedStatusFile) {
						logger.Error(ctx, "Status file became corrupted during wait, marking as invalid", "err", err, "data", data)
					} else {
						logger.Error(ctx, "Failed to read status", "err", err, "data", data)
					}
					result = models.QueuedItemProcessingResultDiscard
					goto SEND_RESULT
				}
				if status.Status != dagstatus.Queued {
					logger.Info(ctx, "DAG run is no longer queued", "data", data, "status", status.Status)
					result = models.QueuedItemProcessingResultDiscard
					break WAIT_FOR_RUN
				}
				if time.Since(startedAt) > 10*time.Second {
					logger.Error(ctx, "Timeout waiting for run to start", "data", data)
					break WAIT_FOR_RUN
				}
				select {
				case <-time.After(500 * time.Millisecond):
				case <-ctx.Done():
					logger.Info(ctx, "Context cancelled while waiting for run to start", "data", data)
					return
				}
			}

			if result == models.QueuedItemProcessingResultSuccess {
				logger.Info(ctx, "Successfully processed item from queue", "data", data)
			}

		SEND_RESULT:
			item.Result <- result
		}
	}
}

// getQueueNameForDAG determines the queue name for a given DAG.
// It returns the DAG's explicitly assigned queue name, or the DAG name if none is specified.
func (s *Scheduler) getQueueNameForDAG(dag *digraph.DAG) string {
	if dag.Queue != "" {
		return dag.Queue
	}
	return dag.Name
}

// getQueueConfigByName gets the queue configuration by queue name.
// It checks global queue configurations first, then falls back to DAG's maxActiveRuns.
func (s *Scheduler) getQueueConfigByName(queueName string, dag *digraph.DAG) queueConfig {
	// Check global queue configurations
	for _, queueCfg := range s.config.Queues.Config {
		if queueCfg.Name == queueName {
			return queueConfig{MaxConcurrency: max(queueCfg.MaxActiveRuns, 1)}
		}
	}

	// Fallback to DAG's maxActiveRuns if no global queue config is found
	if dag.MaxActiveRuns > 0 {
		return queueConfig{MaxConcurrency: dag.MaxActiveRuns}
	}

	// Default configuration if no specific queue config or DAG setting is found
	return queueConfig{MaxConcurrency: 1}
}

// start starts the scheduler.
// It runs in a loop, checking for jobs to run every minute.
func (s *Scheduler) start(ctx context.Context) {
	t := Now().Truncate(time.Minute)
	timer := time.NewTimer(0)

	s.running.Store(true)
	setSchedulerRunning(true)

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

// IsRunning returns whether the scheduler is currently running.
func (s *Scheduler) IsRunning() bool {
	return s.running.Load()
}

// Stop stops the scheduler.
func (s *Scheduler) Stop(ctx context.Context) {
	if !s.running.CompareAndSwap(true, false) {
		return
	}
	setSchedulerRunning(false)

	close(s.stopChan)

	s.lock.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.lock.Unlock()

	if s.heartbeatCancel != nil {
		logger.Info(ctx, "Stopping scheduler heartbeat")
		s.heartbeatCancel()
	}

	// Stop health check server if it was started
	if s.healthServer != nil && !s.disableHealthServer {
		if err := s.healthServer.Stop(ctx); err != nil {
			logger.Error(ctx, "Failed to stop health check server", "err", err)
		}
	}

	// Close DAG executor to release gRPC connections
	if s.dagExecutor != nil {
		s.dagExecutor.Close(ctx)
	}

	if err := s.dirLock.Unlock(); err != nil {
		logger.Error(ctx, "Failed to release scheduler lock in Stop", "err", err)
	} else {
		logger.Info(ctx, "Released scheduler lock in Stop")
	}

	logger.Info(ctx, "Scheduler stopped")
}

// run executes the scheduled jobs at the current time.
func (s *Scheduler) run(ctx context.Context, now time.Time) {
	// Ensure the lock is held while running jobs
	if !s.dirLock.IsHeldByMe() {
		logger.Error(ctx, "Scheduler lock is not held, cannot run jobs")
		return
	}

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
