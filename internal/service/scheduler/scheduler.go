package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/dagu-org/dagu/internal/common/logger/tag"
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

// Clock is a function that returns the current time.
// It can be replaced for testing purposes.
type Clock func() time.Time

type Scheduler struct {
	runtimeManager      runtime.Manager
	entryReader         EntryReader
	logDir              string
	quit                chan any
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
	lock           sync.Mutex
	clock          Clock // Clock function for getting current time
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
	timeLoc := cfg.Core.Location
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
		quit:            make(chan any),
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
		clock:           time.Now, // Default to real time
	}, nil
}

// SetClock sets a custom clock function for testing purposes.
// This must be called before Start().
func (s *Scheduler) SetClock(clock Clock) {
	s.clock = clock
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
			logger.Error(ctx, "Failed to register with service registry", tag.Error(err))
			// Continue anyway - service registry is not critical
		} else {
			logger.Info(ctx, "Registered with service registry as inactive",
				tag.ServiceID(s.instanceID),
				tag.Host(hostname),
				tag.Port(s.config.Scheduler.Port),
			)
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
			logger.Error(ctx, "Failed to update status to active", tag.Error(err))
		} else {
			logger.Info(ctx, "Updated scheduler status to active")
		}
	}

	sig := make(chan os.Signal, 1)

	// Start the DAG file watcher
	queueWatcher := s.queueStore.QueueWatcher(ctx)
	notifyCh, err := queueWatcher.Start(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to start queue watcher", tag.Error(err))
		return err
	}
	s.queueProcessor.Start(ctx, notifyCh)

	// Handle OS signals for graceful shutdown
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.startHeartbeat(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.startZombieDetector(ctx)
	}()

	if err := s.entryReader.Init(ctx); err != nil {
		logger.Error(ctx, "Failed to initialize entry reader", tag.Error(err))
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.entryReader.Start(ctx)
	}()

	logger.Info(ctx, "Scheduler started")

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

	// Create zombie detector while holding lock
	s.lock.Lock()
	s.zombieDetector = NewZombieDetector(
		s.dagRunStore,
		s.procStore,
		s.config.Scheduler.ZombieDetectionInterval,
	)
	zd := s.zombieDetector
	s.lock.Unlock()

	logger.Info(ctx, "Started zombie detector",
		tag.Interval(s.config.Scheduler.ZombieDetectionInterval),
	)

	// Start blocks, so call it after releasing the lock
	zd.Start(ctx)
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
				logger.Error(ctx, "Failed to send heartbeat for scheduler lock", tag.Error(err))
			}
		}
	}
}

// cronLoop runs the main scheduler loop to invoke jobs at scheduled times.
func (s *Scheduler) cronLoop(ctx context.Context, sig chan os.Signal) {
	tickTime := s.clock().Truncate(time.Minute)

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
			timer.Reset(tickTime.Sub(s.clock()))
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
		wg.Add(3)

		close(s.quit)

		go func() {
			defer wg.Done()
			s.queueProcessor.Stop()
		}()

		go func(ctx context.Context) {
			defer wg.Done()
			s.stopCron(ctx)
		}(ctx)

		go func() {
			defer wg.Done()
			s.entryReader.Stop()
		}()

		if s.zombieDetector != nil {
			s.zombieDetector.Stop(ctx)
		}

		if err := s.dirLock.Unlock(); err != nil {
			logger.Error(ctx, "Failed to release scheduler lock in Stop", tag.Error(err))
		}

		wg.Wait()
	})
}

func (s *Scheduler) stopCron(ctx context.Context) {
	// Update status to inactive before stopping
	if s.serviceRegistry != nil {
		if err := s.serviceRegistry.UpdateStatus(ctx, execution.ServiceNameScheduler, execution.ServiceStatusInactive); err != nil {
			logger.Error(ctx, "Failed to update status to inactive", tag.Error(err))
		}
	}

	// Stop health check server if it was started
	if s.healthServer != nil && !s.disableHealthServer {
		if err := s.healthServer.Stop(ctx); err != nil {
			logger.Error(ctx, "Failed to stop health check server", tag.Error(err))
		}
	}

	// Close DAG executor to release gRPC connections
	if s.dagExecutor != nil {
		s.dagExecutor.Close(ctx)
	}

	// Unregister from service registry
	if s.serviceRegistry != nil {
		s.serviceRegistry.Unregister(ctx)
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
		logger.Error(ctx, "Failed to get next jobs", tag.Error(err))
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
			tag.Job(fmt.Sprintf("%v", job.Job)),
			slog.String("jobType", job.Type.String()),
			slog.String("scheduledTime", job.Next.Format(time.RFC3339)),
		)

		// Launch job execution in goroutine
		go func(ctx context.Context, job *ScheduledJob) {
			if err := job.invoke(ctx); err != nil {
				switch {
				case errors.Is(err, ErrJobFinished):
					logger.Info(ctx, "Job already completed")
				case errors.Is(err, ErrJobRunning):
					logger.Info(ctx, "Job already in progress")
				case errors.Is(err, ErrJobSkipped):
					logger.Info(ctx, "Job execution skipped",
						tag.Reason(err.Error()),
					)
				default:
					logger.Error(ctx, "Job execution failed",
						tag.Error(err),
						tag.Type(fmt.Sprintf("%T", err)),
					)
				}
			} else {
				logger.Info(ctx, "Job completed successfully")
			}
		}(jobCtx, job)
	}
}

// invoke invokes the job based on the schedule type.
func (s *ScheduledJob) invoke(ctx context.Context) error {
	if s.Job == nil {
		return fmt.Errorf("job is nil")
	}

	logger.Info(ctx, "Starting operation",
		tag.Type(s.Type.String()),
		tag.Job(fmt.Sprintf("%v", s.Job)),
	)

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
