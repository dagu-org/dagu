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

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/dirlock"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
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
	hm                  runtime.Manager
	er                  EntryReader
	logDir              string
	stopChan            chan struct{}
	running             atomic.Bool
	location            *time.Location
	dagRunStore         execution.DAGRunStore
	queueStore          execution.QueueStore
	procStore           execution.ProcStore
	cancel              context.CancelFunc
	lock                sync.Mutex
	queueConfigs        sync.Map
	config              *config.Config
	dirLock             dirlock.DirLock // File-based lock to prevent multiple scheduler instances
	dagExecutor         *DAGExecutor
	healthServer        *HealthServer // Health check server for monitoring
	serviceRegistry     execution.ServiceRegistry
	disableHealthServer bool // Disable health server when running from start-all
	heartbeatCancel     context.CancelFunc
	heartbeatDone       chan struct{}
	zombieDetector      *ZombieDetector // Zombie DAG run detector
	instanceID          string          // Unique instance identifier for service registry
}

type queueConfig struct {
	MaxConcurrency int
}

func New(
	cfg *config.Config,
	er EntryReader,
	drm runtime.Manager,
	drs execution.DAGRunStore,
	qs execution.QueueStore,
	ps execution.ProcStore,
	reg execution.ServiceRegistry,
	coordinatorCli execution.Dispatcher,
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
	dagExecutor := NewDAGExecutor(coordinatorCli, runtime.NewSubCmdBuilder(cfg))

	// Create health server
	healthServer := NewHealthServer(cfg.Scheduler.Port)

	return &Scheduler{
		logDir:          cfg.Paths.LogDir,
		stopChan:        make(chan struct{}),
		location:        timeLoc,
		er:              er,
		hm:              drm,
		dagRunStore:     drs,
		queueStore:      qs,
		procStore:       ps,
		config:          cfg,
		dirLock:         dirLock,
		dagExecutor:     dagExecutor,
		healthServer:    healthServer,
		serviceRegistry: reg,
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

	// Generate instance ID if not already set
	if s.instanceID == "" {
		hostname, _ := os.Hostname()
		s.instanceID = fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().Unix())
	}

	// Register with service registry as inactive initially
	if s.serviceRegistry != nil {
		hostname, _ := os.Hostname()
		hostInfo := execution.HostInfo{
			ID:        s.instanceID,
			Host:      hostname,
			Port:      s.config.Scheduler.Port, // Health check port (0 if disabled)
			Status:    execution.ServiceStatusInactive,
			StartedAt: time.Now(),
		}
		if err := s.serviceRegistry.Register(ctx, execution.ServiceNameScheduler, hostInfo); err != nil {
			logger.Error(ctx, "Failed to register with service registry", "err", err)
			// Continue anyway - service registry is not critical
		} else {
			logger.Info(ctx, "Registered with service registry as inactive",
				"instance_id", s.instanceID,
				"host", hostname,
				"port", s.config.Scheduler.Port)
		}
	}

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

	// Update status to active after acquiring lock
	if s.serviceRegistry != nil {
		if err := s.serviceRegistry.UpdateStatus(ctx, execution.ServiceNameScheduler, execution.ServiceStatusActive); err != nil {
			logger.Error(ctx, "Failed to update status to active", "err", err)
		} else {
			logger.Info(ctx, "Updated scheduler status to active")
		}
	}

	// Ensure lock is always released
	defer func() {
		if err := s.dirLock.Unlock(); err != nil {
			logger.Error(ctx, "Failed to release scheduler lock in defer", "err", err)
		} else {
			logger.Info(ctx, "Released scheduler lock in defer")
		}
	}()

	sig := make(chan os.Signal, 1)

	done := make(chan any)
	defer close(done)

	// Start the DAG file watcher
	if err := s.er.Start(ctx, done); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	// Start queue reader only if queues are enabled
	var qr execution.QueueReader
	var wgQueue sync.WaitGroup
	if s.config.Queues.Enabled {
		queueCh := make(chan execution.QueuedItem, 1)
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

	// Start zombie detector if enabled
	if s.config.Scheduler.ZombieDetectionInterval > 0 {
		s.zombieDetector = NewZombieDetector(
			s.dagRunStore,
			s.procStore,
			s.config.Scheduler.ZombieDetectionInterval,
		)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Error(ctx, "Zombie detector panicked", "panic", r)
				}
			}()
			s.zombieDetector.Start(ctx)
		}()
		logger.Info(ctx, "Started zombie detector", "interval", s.config.Scheduler.ZombieDetectionInterval)
	} else {
		logger.Info(ctx, "Zombie detector disabled")
	}

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
func (s *Scheduler) handleQueue(ctx context.Context, ch chan execution.QueuedItem, done chan any) {
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
				dag       *core.DAG
				attempt   execution.DAGRunAttempt
				st        *execution.DAGRunStatus
				err       error
				result    = execution.QueuedItemProcessingResultRetry
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
				if errors.Is(err, execution.ErrDAGRunIDNotFound) {
					logger.Error(ctx, "DAG run not found, marking as discard", "data", data)
					result = execution.QueuedItemProcessingResultDiscard
				}
				goto SEND_RESULT
			}

			if attempt.Hidden() {
				logger.Info(ctx, "DAG run is hidden, marking as discard", "data", data)
				result = execution.QueuedItemProcessingResultDiscard
				goto SEND_RESULT
			}

			dag, err = attempt.ReadDAG(ctx)
			if err != nil {
				logger.Error(ctx, "Failed to read dag", "err", err, "data", data)
				goto SEND_RESULT
			}

			alive, err = s.procStore.CountAlive(ctx, dag.ProcGroup())
			if err != nil {
				logger.Error(ctx, "Failed to count alive processes", "err", err, "data", data)
				goto SEND_RESULT
			}

			st, err = attempt.ReadStatus(ctx)
			if err != nil {
				if errors.Is(err, execution.ErrCorruptedStatusFile) {
					logger.Error(ctx, "Status file is corrupted, marking as invalid", "err", err, "data", data)
					result = execution.QueuedItemProcessingResultDiscard
				} else if ctx.Err() != nil {
					logger.Debug(ctx, "Context is cancelled", "err", err)
				} else {
					logger.Error(ctx, "Failed to read status", "err", err, "data", data)
				}
				goto SEND_RESULT
			}

			if st.Status != core.Queued {
				logger.Info(ctx, "Skipping item from queue", "data", data, "status", st.Status)
				result = execution.QueuedItemProcessingResultDiscard
				goto SEND_RESULT
			}

			// Check concurrency limits based on queue configuration
			queueCfg = s.getQueueConfig(dag.ProcGroup(), dag)
			if queueCfg.MaxConcurrency > 0 && alive >= queueCfg.MaxConcurrency {
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

			// Wait until the DAG to be alive
			time.Sleep(500 * time.Millisecond)

			// Wait for the DAG to be picked up by checking process heartbeat
		WAIT_FOR_RUN:
			for {
				// Check if the process is alive (has heartbeat)
				isAlive, err := s.procStore.IsRunAlive(ctx, dag.ProcGroup(), execution.DAGRunRef{Name: dag.Name, ID: data.ID})
				if err != nil {
					logger.Error(ctx, "Failed to check if run is alive", "err", err, "data", data)
					// Continue checking on error, don't immediately fail
				} else if isAlive {
					// Process has started and has heartbeat
					logger.Info(ctx, "DAG run has started (heartbeat detected)", "data", data)
					result = execution.QueuedItemProcessingResultSuccess
					break WAIT_FOR_RUN
				}

				// Check timeout
				if time.Since(startedAt) > 30*time.Second {
					logger.Error(ctx, "Cancelling due to timeout waiting for the run to be alive (10sec)", "data", data)

					// Somehow it's failed to execute. Mark it failed and discard from queue.
					if err := s.markStatusFailed(ctx, attempt); err != nil {
						logger.Error(ctx, "Failed to mark the status cancelled")
					}

					logger.Info(ctx, "Discard the queue item due to timeout", "data", data)
					result = execution.QueuedItemProcessingResultDiscard
					break WAIT_FOR_RUN
				}

				// Check status if it's already finished
				st, err = attempt.ReadStatus(ctx)
				if err != nil {
					logger.Error(ctx, "Failed to read status. Is it corrupted?", "err", err, "data", data)
					result = execution.QueuedItemProcessingResultDiscard
					break WAIT_FOR_RUN
				}

				if st.Status != core.Queued {
					logger.Info(ctx, "Looks like the DAG is already executed", "data", data, "status", st.Status.String())
					result = execution.QueuedItemProcessingResultDiscard
					break WAIT_FOR_RUN
				}

				select {
				case <-time.After(500 * time.Millisecond):
				case <-ctx.Done():
					logger.Info(ctx, "Context cancelled while waiting for run to start", "data", data)
					return
				}
			}

			if result == execution.QueuedItemProcessingResultSuccess {
				logger.Info(ctx, "Successfully processed item from queue", "data", data)
			}

		SEND_RESULT:
			item.Result <- result
		}
	}
}

func (s *Scheduler) markStatusFailed(ctx context.Context, attempt execution.DAGRunAttempt) error {
	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status to update status: %w", err)
	}
	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("failed to open attempt: %w", err)
	}
	defer func() {
		if err := attempt.Close(ctx); err != nil {
			logger.Error(ctx, "Failed to close attempt", "err", err)
		}
	}()
	if st.Status != core.Queued {
		logger.Info(ctx, "Tried to mark a queued item 'cancelled' but it's different status now", "status", st.Status.String())
		return nil
	}
	st.Status = core.Cancel // Mark it cancel
	if err := attempt.Write(ctx, *st); err != nil {
		return fmt.Errorf("failed to open attempt: %w", err)
	}
	return nil
}

// getQueueConfigByName gets the queue configuration by queue name.
// It checks global queue configurations first, then falls back to DAG's maxActiveRuns.
func (s *Scheduler) getQueueConfig(queueName string, dag *core.DAG) queueConfig {
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

	// Update status to inactive before stopping
	if s.serviceRegistry != nil {
		if err := s.serviceRegistry.UpdateStatus(ctx, execution.ServiceNameScheduler, execution.ServiceStatusInactive); err != nil {
			logger.Error(ctx, "Failed to update status to inactive", "err", err)
		}
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

	// Unregister from service registry
	if s.serviceRegistry != nil {
		s.serviceRegistry.Unregister(ctx)
		logger.Info(ctx, "Unregistered from service registry")
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
