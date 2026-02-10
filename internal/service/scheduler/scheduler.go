package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/dirlock"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

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
	dagRunStore         exec.DAGRunStore
	queueStore          exec.QueueStore
	procStore           exec.ProcStore
	config              *config.Config
	dirLock             dirlock.DirLock // File-based lock to prevent multiple scheduler instances
	dagExecutor         *DAGExecutor
	healthServer        *HealthServer // Health check server for monitoring
	serviceRegistry     exec.ServiceRegistry
	disableHealthServer bool            // Disable health server when running from start-all
	zombieDetector      *ZombieDetector // Zombie DAG run detector
	instanceID          string          // Unique instance identifier for service registry
	queueProcessor      *QueueProcessor // Processor for queued DAG runs
	planner             *TickPlanner    // Unified scheduling decision module
	stopOnce            sync.Once
	lock                sync.Mutex
	clock               Clock // Clock function for getting current time
}

// New constructs a Scheduler from the provided stores, runtime manager,
// service registry, and dispatcher.
func New(
	cfg *config.Config,
	er EntryReader,
	drm runtime.Manager,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	procStore exec.ProcStore,
	reg exec.ServiceRegistry,
	coordinatorCli exec.Dispatcher,
	watermarkStore WatermarkStore,
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
	dagExecutor := NewDAGExecutor(coordinatorCli, subCmdBuilder, cfg.DefaultExecMode)
	healthServer := NewHealthServer(cfg.Scheduler.Port)
	processor := NewQueueProcessor(
		queueStore,
		dagRunStore,
		procStore,
		dagExecutor,
		cfg.Queues,
	)
	defaultClock := Clock(time.Now)

	// Resolve IsSuspended once at construction time and wire the event channel.
	eventCh := make(chan DAGChangeEvent)
	var isSuspended IsSuspendedFunc
	if impl, ok := er.(*entryReaderImpl); ok {
		isSuspended = impl.dagStore.IsSuspended
		impl.setEvents(eventCh)
	}

	planner := NewTickPlanner(TickPlannerConfig{
		WatermarkStore:  watermarkStore,
		IsSuspended:     isSuspended,
		GetLatestStatus: drm.GetLatestStatus,
		IsRunning: func(ctx context.Context, dag *core.DAG) (bool, error) {
			count, err := procStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
			if err != nil {
				return false, err
			}
			return count > 0, nil
		},
		GenRunID: drm.GenDAGRunID,
		Dispatch: func(ctx context.Context, dag *core.DAG, runID string, triggerType core.TriggerType) error {
			return dagExecutor.HandleJob(
				ctx, dag,
				coordinatorv1.Operation_OPERATION_START,
				runID, triggerType,
			)
		},
		Stop: func(ctx context.Context, dag *core.DAG) error {
			return drm.Stop(ctx, dag, "")
		},
		Restart: func(ctx context.Context, dag *core.DAG) error {
			return dagExecutor.Restart(ctx, dag)
		},
		Clock:    defaultClock,
		Location: timeLoc,
		Events:   eventCh,
	})

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
		planner:         planner,
		clock:           defaultClock,
	}, nil
}

// SetClock sets a custom clock function for testing purposes.
// This must be called before Start().
func (s *Scheduler) SetClock(clock Clock) {
	s.clock = clock
	s.planner.cfg.Clock = clock
}

// SetRestartFunc overrides the planner's restart function for testing purposes.
// This must be called before Start().
func (s *Scheduler) SetRestartFunc(fn RestartFunc) {
	s.planner.cfg.Restart = fn
}

// SetStopFunc overrides the planner's stop function for testing purposes.
// This must be called before Start().
func (s *Scheduler) SetStopFunc(fn StopFunc) {
	s.planner.cfg.Stop = fn
}

// SetGetLatestStatusFunc overrides the planner's GetLatestStatus function for testing purposes.
// This must be called before Start().
func (s *Scheduler) SetGetLatestStatusFunc(fn GetLatestStatusFunc) {
	s.planner.cfg.GetLatestStatus = fn
}

// SetDispatchFunc overrides the planner's Dispatch function for testing purposes.
// This must be called before Start().
func (s *Scheduler) SetDispatchFunc(fn DispatchFunc) {
	s.planner.cfg.Dispatch = fn
}

// DisableHealthServer disables the health check server (used when running from start-all)
func (s *Scheduler) DisableHealthServer() {
	s.disableHealthServer = true
}

func (s *Scheduler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if s.instanceID == "" {
		hostname, _ := os.Hostname()
		s.instanceID = fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().Unix())
	}

	if s.serviceRegistry != nil {
		hostname, _ := os.Hostname()
		hostInfo := exec.HostInfo{
			ID:        s.instanceID,
			Host:      hostname,
			Port:      s.config.Scheduler.Port, // Health check port (0 if disabled)
			Status:    exec.ServiceStatusInactive,
			StartedAt: time.Now(),
		}
		if err := s.serviceRegistry.Register(ctx, exec.ServiceNameScheduler, hostInfo); err != nil {
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

	if !s.disableHealthServer {
		if err := s.healthServer.Start(ctx); err != nil {
			return fmt.Errorf("failed to start health check server: %w", err)
		}
	}

	logger.Info(ctx, "Waiting to acquire scheduler lock")
	if err := s.dirLock.Lock(ctx); err != nil {
		return fmt.Errorf("failed to acquire scheduler lock: %w", err)
	}

	logger.Info(ctx, "Acquired scheduler lock")

	if s.serviceRegistry != nil {
		if err := s.serviceRegistry.UpdateStatus(ctx, exec.ServiceNameScheduler, exec.ServiceStatusActive); err != nil {
			logger.Error(ctx, "Failed to update status to active", tag.Error(err))
		} else {
			logger.Info(ctx, "Updated scheduler status to active")
		}
	}

	sig := make(chan os.Signal, 1)

	queueWatcher := s.queueStore.QueueWatcher(ctx)
	notifyCh, err := queueWatcher.Start(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to start queue watcher", tag.Error(err))
		return err
	}
	s.queueProcessor.Start(ctx, notifyCh)

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

	if err := s.planner.Init(ctx, s.entryReader.DAGs()); err != nil {
		logger.Error(ctx, "Failed to initialize tick planner", tag.Error(err))
	}

	s.planner.Start(ctx)

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

			// Plan and dispatch all schedules (start, stop, restart)
			for _, run := range s.planner.Plan(ctx, tickTime) {
				s.dispatchRun(ctx, run)
			}
			s.planner.Advance(tickTime)

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

		// Stop the producer (entryReader) synchronously BEFORE the consumer (planner).
		// This ensures er.quit is closed before drainEvents exits, so any in-flight
		// sendEvent unblocks via the select on er.quit.
		s.entryReader.Stop()

		if s.zombieDetector != nil {
			s.zombieDetector.Stop(ctx)
		}

		s.planner.Stop(ctx)

		if err := s.dirLock.Unlock(); err != nil {
			logger.Error(ctx, "Failed to release scheduler lock in Stop", tag.Error(err))
		}

		wg.Wait()
	})
}

func (s *Scheduler) stopCron(ctx context.Context) {
	if s.serviceRegistry != nil {
		if err := s.serviceRegistry.UpdateStatus(ctx, exec.ServiceNameScheduler, exec.ServiceStatusInactive); err != nil {
			logger.Error(ctx, "Failed to update status to inactive", tag.Error(err))
		}
	}

	if s.healthServer != nil && !s.disableHealthServer {
		if err := s.healthServer.Stop(ctx); err != nil {
			logger.Error(ctx, "Failed to stop health check server", tag.Error(err))
		}
	}

	if s.dagExecutor != nil {
		s.dagExecutor.Close(ctx)
	}

	if s.serviceRegistry != nil {
		s.serviceRegistry.Unregister(ctx)
	}

	logger.Info(ctx, "Scheduler stopped")
}

// dispatchRun dispatches a planned run in a goroutine.
func (s *Scheduler) dispatchRun(ctx context.Context, run PlannedRun) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error(ctx, "Run dispatch panicked",
					tag.DAG(run.DAG.Name),
					tag.Error(panicToError(r)),
				)
			}
		}()
		s.planner.DispatchRun(ctx, run)
	}()
}

