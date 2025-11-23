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
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
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
	runtimeManager      runtime.Manager
	entryReader         EntryReader
	logDir              string
	quit                chan struct{}
	running             atomic.Bool
	location            *time.Location
	dagRunStore         execution.DAGRunStore
	queueStore          execution.QueueStore
	procStore           execution.ProcStore
	config              *config.Config
	dirLock             dirlock.DirLock // File-based lock to prevent multiple scheduler instances
	dagExecutor         *DAGExecutor
	healthServer        *HealthServer // Health check server for monitoring
	serviceRegistry     execution.ServiceRegistry
	disableHealthServer bool            // Disable health server when running from start-all
	zombieDetector      *ZombieDetector // Zombie DAG run detector
	instanceID          string          // Unique instance identifier for service registry
	// queueProcessor is the processor for queued DAG runs
	queueProcessor *QueueProcessor
	stopOnce       sync.Once
	lockReleased   atomic.Bool // Tracks if lock was released to prevent double unlock
	lock           sync.Mutex
}

// New creates a new Scheduler.
func New(
	cfg *config.Config,
	er EntryReader,
	drm runtime.Manager,
	dagRunStore execution.DAGRunStore,
	queueStore execution.QueueStore,
	procStore execution.ProcStore,
	reg execution.ServiceRegistry,
	coordinatorCli execution.Dispatcher,
) (*Scheduler, error) {
	timeLoc := cfg.Global.Location
	if timeLoc == nil {
		timeLoc = time.Local
	}
	lockOpts := &dirlock.LockOptions{
		StaleThreshold: cfg.Scheduler.LockStaleThreshold,
		RetryInterval:  cfg.Scheduler.LockRetryInterval,
	}
	lockDir := filepath.Join(cfg.Paths.DataDir, "scheduler", "locks")
	dirLock := dirlock.New(lockDir, lockOpts)
	subCmdBuilder := runtime.NewSubCmdBuilder(cfg)
	dagExecutor := NewDAGExecutor(coordinatorCli, subCmdBuilder)
	healthServer := NewHealthServer(cfg.Scheduler.Port)
	processor := NewQueueProcessor(
		queueStore,
		dagRunStore,
		procStore,
		dagExecutor,
		cfg.Queues,
	)
	return &Scheduler{
		logDir:          cfg.Paths.LogDir,
		quit:            make(chan struct{}),
		location:        timeLoc,
		entryReader:     er,
		runtimeManager:  drm,
		dagRunStore:     dagRunStore,
		queueStore:      queueStore,
		procStore:       procStore,
		config:          cfg,
		dirLock:         dirLock,
		dagExecutor:     dagExecutor,
		healthServer:    healthServer,
		serviceRegistry: reg,
		queueProcessor:  processor,
	}, nil
}

// DisableHealthServer disables the health check server (used when running from start-all)
func (s *Scheduler) DisableHealthServer() {
	s.disableHealthServer = true
}

func (s *Scheduler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
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
		if s.lockReleased.Swap(true) {
			return // Already released by Stop()
		}
		if err := s.dirLock.Unlock(); err != nil {
			logger.Error(ctx, "Failed to release scheduler lock in defer", "err", err)
		} else {
			logger.Info(ctx, "Released scheduler lock in defer")
		}
	}()

	sig := make(chan os.Signal, 1)

	// Start the DAG file watcher
	queueWatcher := s.queueStore.QueueWatcher(ctx)
	notifyCh, err := queueWatcher.Start(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to start queue watcher")
		return err
	}
	s.queueProcessor.Start(ctx, notifyCh)

	// Handle OS signals for graceful shutdown
	signal.Notify(
		sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT,
	)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.startHeartbeat(ctx)
	}()

	s.startZombieDetector(ctx)

	logger.Info(ctx, "Scheduler started")

	// Start the scheduler loop (it blocks)
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		s.cronLoop(ctx, sig)
	}(ctx)
	wg.Wait()

	return nil
}

func (s *Scheduler) startZombieDetector(ctx context.Context) {
	if s.config.Scheduler.ZombieDetectionInterval <= 0 {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.zombieDetector = NewZombieDetector(
		s.dagRunStore,
		s.procStore,
		s.config.Scheduler.ZombieDetectionInterval,
	)
	s.zombieDetector.Start(ctx)
	logger.Info(ctx, "Started zombie detector", "interval", s.config.Scheduler.ZombieDetectionInterval)
}

func (s *Scheduler) startHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 7)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.dirLock.Heartbeat(ctx); err != nil {
				logger.Error(ctx, "Failed to send heartbeat for scheduler lock", "err", err)
			}
		}
	}
}

// cronLoop runs the main scheduler loop to invoke jobs at scheduled times.
func (s *Scheduler) cronLoop(ctx context.Context, sig chan os.Signal) {
	tickTime := Now().Truncate(time.Minute)

	timer := time.NewTimer(0)
	defer timer.Stop()

	s.running.Store(true)
	defer s.running.Store(false)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sig:
			return
		case <-s.quit:
			return
		case <-timer.C:
			_ = timer.Stop()
			s.invokeJobs(ctx, tickTime)
			tickTime = s.NextTick(tickTime)
			timer.Reset(tickTime.Sub(Now()))
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
	s.lock.Lock()
	defer s.lock.Unlock()

	s.stopOnce.Do(func() {
		var wg sync.WaitGroup
		wg.Add(2)

		close(s.quit)

		go func() {
			defer wg.Done()
			s.queueProcessor.Stop()
		}()

		go func(ctx context.Context) {
			defer wg.Done()
			s.stopCron(ctx)
		}(ctx)

		if s.zombieDetector != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.zombieDetector.Stop(ctx)
			}()
		}

		wg.Wait()
	})
}

func (s *Scheduler) stopCron(ctx context.Context) {
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

	if !s.lockReleased.Swap(true) {
		if err := s.dirLock.Unlock(); err != nil {
			logger.Error(ctx, "Failed to release scheduler lock in Stop", "err", err)
		} else {
			logger.Info(ctx, "Released scheduler lock in Stop")
		}
	}

	// Unregister from service registry
	if s.serviceRegistry != nil {
		s.serviceRegistry.Unregister(ctx)
		logger.Info(ctx, "Unregistered from service registry")
	}

	logger.Info(ctx, "Scheduler stopped")
}

// invokeJobs executes the scheduled jobs at the current time.
func (s *Scheduler) invokeJobs(ctx context.Context, now time.Time) {
	// Ensure the lock is held while running jobs
	if !s.dirLock.IsHeldByMe() {
		logger.Error(ctx, "Scheduler lock is not held, cannot run jobs")
		return
	}

	// Get jobs scheduled to run at or before the current time
	// Subtract a small buffer to avoid edge cases with exact timing
	jobs, err := s.entryReader.Next(ctx, now.Add(-time.Second).In(s.location))
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

		// Launch job execution in goroutine
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
		return fmt.Errorf("job is nil")
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
