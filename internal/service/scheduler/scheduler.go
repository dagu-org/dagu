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

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/dirlock"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// Job is the interface for the actual DAG.
type Job interface {
	// GetDAG returns the DAG associated with this job.
	GetDAG(ctx context.Context) *core.DAG
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
	// queueProcessor is the processor for queued DAG runs
	queueProcessor *QueueProcessor
	watermarkStore core.WatermarkStore // Persists scheduler watermark for catch-up
	watermarkState *core.SchedulerState // In-memory watermark state
	watermarkDirty atomic.Bool          // Dirty flag for periodic flush
	dagQueues      map[string]*DAGQueue // Per-DAG queues for catch-up (only for DAGs with CatchupWindow > 0)
	stopOnce       sync.Once
	lock           sync.Mutex
	clock          Clock // Clock function for getting current time
}

// New constructs a Scheduler configured with the provided stores, runtime manager,
// service registry, and dispatcher.
//
// It determines the scheduler time location from cfg.Core.Location (falls back to
// time.Local), creates a directory lock, DAG executor, health server, and queue
// processor, and sets the default clock to the real-time clock.
//
// The returned Scheduler is ready for startup; any initialization errors are
// returned.
func New(
	cfg *config.Config,
	er EntryReader,
	drm runtime.Manager,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	procStore exec.ProcStore,
	reg exec.ServiceRegistry,
	coordinatorCli exec.Dispatcher,
	watermarkStore core.WatermarkStore,
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
		watermarkStore:  watermarkStore,
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
		if err := s.serviceRegistry.UpdateStatus(ctx, exec.ServiceNameScheduler, exec.ServiceStatusActive); err != nil {
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.startWatermarkFlusher(ctx)
	}()

	// Load watermark state for catch-up
	if s.watermarkStore != nil {
		state, err := s.watermarkStore.Load(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to load watermark state", tag.Error(err))
			// Non-fatal: continue without watermark (no catch-up)
		} else {
			s.watermarkState = state
			logger.Info(ctx, "Loaded scheduler watermark",
				slog.Time("lastTick", state.LastTick),
				slog.Int("dagCount", len(state.DAGs)),
			)
		}
	}

	if err := s.entryReader.Init(ctx); err != nil {
		logger.Error(ctx, "Failed to initialize entry reader", tag.Error(err))
		return err
	}

	// Initialize catch-up queues for DAGs with CatchupWindow > 0
	s.initCatchupQueues(ctx, &wg)

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
			s.updateWatermarkTick(tickTime)
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

		// Close catch-up queues (signals consumers to exit)
		s.closeCatchupQueues()

		// Final watermark flush on shutdown
		s.flushWatermark(ctx)

		if err := s.dirLock.Unlock(); err != nil {
			logger.Error(ctx, "Failed to release scheduler lock in Stop", tag.Error(err))
		}

		wg.Wait()
	})
}

func (s *Scheduler) stopCron(ctx context.Context) {
	// Update status to inactive before stopping
	if s.serviceRegistry != nil {
		if err := s.serviceRegistry.UpdateStatus(ctx, exec.ServiceNameScheduler, exec.ServiceStatusInactive); err != nil {
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

		// For start jobs, route through per-DAG queue if one exists
		if job.Type == ScheduleTypeStart {
			if dag := job.Job.GetDAG(jobCtx); dag != nil {
				if q, ok := s.dagQueues[dag.Name]; ok {
					q.Send(QueueItem{
						DAG:           dag,
						ScheduledTime: job.Next,
						TriggerType:   core.TriggerTypeScheduler,
						ScheduleType:  job.Type,
					})
					continue
				}
			}
		}

		// No queue — dispatch directly as before
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

// updateWatermarkTick updates the last tick time in the in-memory watermark state.
func (s *Scheduler) updateWatermarkTick(tickTime time.Time) {
	if s.watermarkState == nil {
		return
	}
	s.watermarkState.LastTick = tickTime
	s.watermarkDirty.Store(true)
}

// startWatermarkFlusher periodically flushes dirty watermark state to disk.
// Runs every 5 seconds, writing at most 12 times per minute regardless of DAG count.
func (s *Scheduler) startWatermarkFlusher(ctx context.Context) {
	if s.watermarkStore == nil {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.quit:
			return
		case <-ticker.C:
			s.flushWatermark(ctx)
		}
	}
}

// flushWatermark writes the watermark state to disk if dirty.
func (s *Scheduler) flushWatermark(ctx context.Context) {
	if s.watermarkStore == nil || s.watermarkState == nil {
		return
	}
	if !s.watermarkDirty.CompareAndSwap(true, false) {
		return
	}
	if err := s.watermarkStore.Save(ctx, s.watermarkState); err != nil {
		logger.Error(ctx, "Failed to flush watermark state", tag.Error(err))
		// Re-mark as dirty so we try again next cycle
		s.watermarkDirty.Store(true)
	}
}

// initCatchupQueues creates per-DAG queues for DAGs with CatchupWindow > 0,
// computes missed intervals, enqueues catch-up items, and starts consumer goroutines.
func (s *Scheduler) initCatchupQueues(ctx context.Context, wg *sync.WaitGroup) {
	if s.watermarkState == nil {
		return
	}

	now := s.clock()
	dags := s.entryReader.DAGs()
	s.dagQueues = make(map[string]*DAGQueue, len(dags))

	var totalMissed int

	for _, dag := range dags {
		if dag.CatchupWindow <= 0 {
			continue
		}

		dagName := dag.Name

		// Compute replay boundary
		var lastTick time.Time
		var lastScheduledTime time.Time

		lastTick = s.watermarkState.LastTick
		if wm, ok := s.watermarkState.DAGs[dagName]; ok {
			lastScheduledTime = wm.LastScheduledTime
		}

		replayFrom := ComputeReplayFrom(dag.CatchupWindow, lastTick, lastScheduledTime, now)
		missed := ComputeMissedIntervals(dag.Schedule, replayFrom, now)

		if len(missed) == 0 {
			continue
		}

		totalMissed += len(missed)

		logger.Info(ctx, "Catch-up planned",
			tag.DAG(dagName),
			slog.Int("missedCount", len(missed)),
			slog.Time("replayFrom", replayFrom),
			slog.Time("replayTo", now),
		)

		// Create queue with enough buffer for catch-up items plus some headroom for live ticks
		q := NewDAGQueue(dagName, dag.OverlapPolicy)
		s.dagQueues[dagName] = q

		// Enqueue catch-up items
		for _, t := range missed {
			q.Send(QueueItem{
				DAG:           dag,
				ScheduledTime: t,
				TriggerType:   core.TriggerTypeCatchUp,
				ScheduleType:  ScheduleTypeStart,
			})
		}

		// Start consumer goroutine
		procGroup := dag.ProcGroup()
		dispatch := s.makeDispatchFunc(ctx)
		isRunning := s.makeIsRunningFunc(procGroup)

		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Start(ctx, dispatch, isRunning)
		}()
	}

	if totalMissed > 0 {
		logger.Info(ctx, "Catch-up initialization complete",
			slog.Int("dagCount", len(s.dagQueues)),
			slog.Int("totalMissedRuns", totalMissed),
		)
	}
}

// makeDispatchFunc returns a DispatchFunc that dispatches a queue item via the DAG executor.
func (s *Scheduler) makeDispatchFunc(_ context.Context) DispatchFunc {
	return func(ctx context.Context, item QueueItem) error {
		runID, err := s.runtimeManager.GenDAGRunID(ctx)
		if err != nil {
			return fmt.Errorf("failed to generate run ID: %w", err)
		}

		logger.Info(ctx, "Dispatching queued run",
			tag.DAG(item.DAG.Name),
			tag.RunID(runID),
			slog.String("triggerType", item.TriggerType.String()),
			slog.String("scheduledTime", item.ScheduledTime.Format(time.RFC3339)),
		)

		err = s.dagExecutor.HandleJob(
			ctx,
			item.DAG,
			coordinatorv1.Operation_OPERATION_START,
			runID,
			item.TriggerType,
		)

		if err == nil {
			// Update watermark state for this DAG
			s.updateWatermarkDAG(item.DAG.Name, item.ScheduledTime)
		}

		return err
	}
}

// makeIsRunningFunc returns an IsRunningFunc that checks if a DAG has any active run.
func (s *Scheduler) makeIsRunningFunc(procGroup string) IsRunningFunc {
	return func(ctx context.Context, dagName string) (bool, error) {
		count, err := s.procStore.CountAliveByDAGName(ctx, procGroup, dagName)
		if err != nil {
			return false, err
		}
		return count > 0, nil
	}
}

// updateWatermarkDAG updates the per-DAG watermark after a dispatch.
func (s *Scheduler) updateWatermarkDAG(dagName string, scheduledTime time.Time) {
	if s.watermarkState == nil {
		return
	}
	if s.watermarkState.DAGs == nil {
		s.watermarkState.DAGs = make(map[string]core.DAGWatermark)
	}
	s.watermarkState.DAGs[dagName] = core.DAGWatermark{
		LastScheduledTime: scheduledTime,
	}
	s.watermarkDirty.Store(true)
}

// closeCatchupQueues closes all per-DAG catch-up queues.
func (s *Scheduler) closeCatchupQueues() {
	for _, q := range s.dagQueues {
		q.Close()
	}
}
