# Scheduler Decomposition Example

This document provides concrete examples of how to decompose the large Scheduler struct.

## Current State (scheduler.go - 841 lines)

```go
// Current: God object with 25+ fields and mixed concerns
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
    pendingMu           sync.Mutex
    pendingRuns         map[string]int
    config              *config.Config
    dirLock             dirlock.DirLock
    dagExecutor         *DAGExecutor
    healthServer        *HealthServer
    serviceRegistry     execution.ServiceRegistry
    disableHealthServer bool
    heartbeatCancel     context.CancelFunc
    heartbeatDone       chan struct{}
    zombieDetector      *ZombieDetector
    instanceID          string
}

// Current: 20+ methods handling different concerns
func (s *Scheduler) Start(ctx context.Context) error { /* 100+ lines */ }
func (s *Scheduler) Stop(ctx context.Context) error { /* 50+ lines */ }
func (s *Scheduler) ScheduleDAG(ctx context.Context, dag *DAG) error { /* 80+ lines */ }
func (s *Scheduler) enqueue(ctx context.Context, run *DAGRun) error { /* 60+ lines */ }
func (s *Scheduler) dequeue(ctx context.Context) (*DAGRun, error) { /* 70+ lines */ }
func (s *Scheduler) getQueueStatus(name string) (*QueueStatus, error) { /* 40+ lines */ }
func (s *Scheduler) checkHealth() HealthStatus { /* 30+ lines */ }
func (s *Scheduler) detectZombies(ctx context.Context) error { /* 50+ lines */ }
// ... 12+ more methods
```

## Proposed State (Multiple Focused Components)

### 1. Core Scheduler (scheduler_core.go - ~200 lines)

```go
package scheduler

import (
    "context"
    "time"

    "github.com/dagu-org/dagu/internal/service/scheduler/queue"
    "github.com/dagu-org/dagu/internal/service/scheduler/health"
    "github.com/dagu-org/dagu/internal/service/scheduler/zombie"
    "github.com/dagu-org/dagu/internal/service/scheduler/executor"
)

// Scheduler orchestrates DAG scheduling with minimal dependencies
type Scheduler struct {
    queue      queue.Manager
    health     health.Monitor
    zombie     zombie.Detector
    executor   executor.Executor
    registry   ServiceRegistry
    config     *Config
    location   *time.Location
    
    // Lifecycle management
    cancel     context.CancelFunc
    stopChan   chan struct{}
    running    atomic.Bool
}

// Config holds scheduler configuration
type Config struct {
    LogDir              string
    LockStaleThreshold  time.Duration
    LockRetryInterval   time.Duration
    HealthCheckPort     int
    ZombieCheckInterval time.Duration
    MaxConcurrentDAGs   int
}

// New creates a new scheduler with injected dependencies
func New(
    queueMgr queue.Manager,
    healthMon health.Monitor,
    zombieDet zombie.Detector,
    exec executor.Executor,
    registry ServiceRegistry,
    cfg *Config,
) *Scheduler {
    return &Scheduler{
        queue:    queueMgr,
        health:   healthMon,
        zombie:   zombieDet,
        executor: exec,
        registry: registry,
        config:   cfg,
        location: time.Local,
        stopChan: make(chan struct{}),
    }
}

// Start begins the scheduler lifecycle
func (s *Scheduler) Start(ctx context.Context) error {
    if !s.running.CompareAndSwap(false, true) {
        return ErrAlreadyRunning
    }
    
    ctx, s.cancel = context.WithCancel(ctx)
    
    // Start sub-components
    if err := s.health.Start(ctx); err != nil {
        return fmt.Errorf("failed to start health monitor: %w", err)
    }
    
    if err := s.zombie.Start(ctx); err != nil {
        return fmt.Errorf("failed to start zombie detector: %w", err)
    }
    
    if err := s.queue.Start(ctx); err != nil {
        return fmt.Errorf("failed to start queue manager: %w", err)
    }
    
    // Start scheduling loop
    go s.scheduleLoop(ctx)
    
    return nil
}

// Stop gracefully shuts down the scheduler
func (s *Scheduler) Stop(ctx context.Context) error {
    if !s.running.CompareAndSwap(true, false) {
        return ErrNotRunning
    }
    
    // Signal stop
    close(s.stopChan)
    if s.cancel != nil {
        s.cancel()
    }
    
    // Stop sub-components
    var errs []error
    if err := s.queue.Stop(ctx); err != nil {
        errs = append(errs, err)
    }
    if err := s.health.Stop(); err != nil {
        errs = append(errs, err)
    }
    if err := s.zombie.Stop(); err != nil {
        errs = append(errs, err)
    }
    
    return errors.Join(errs...)
}

// ScheduleDAG schedules a DAG for execution
func (s *Scheduler) ScheduleDAG(ctx context.Context, dag *DAG) error {
    if !s.running.Load() {
        return ErrNotRunning
    }
    
    run := &DAGRun{
        ID:        generateID(),
        DAG:       dag,
        Status:    StatusPending,
        CreatedAt: time.Now().In(s.location),
    }
    
    // Enqueue through queue manager
    if err := s.queue.Enqueue(ctx, run); err != nil {
        return fmt.Errorf("failed to enqueue DAG run: %w", err)
    }
    
    return nil
}

// scheduleLoop is the main scheduling loop
func (s *Scheduler) scheduleLoop(ctx context.Context) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-s.stopChan:
            return
        case <-ticker.C:
            if err := s.processQueue(ctx); err != nil {
                // Log error but continue
                log.Error("failed to process queue", "error", err)
            }
        }
    }
}

// processQueue processes pending DAG runs
func (s *Scheduler) processQueue(ctx context.Context) error {
    run, err := s.queue.Dequeue(ctx)
    if err != nil {
        if errors.Is(err, queue.ErrEmpty) {
            return nil // Empty queue is not an error
        }
        return err
    }
    
    if run == nil {
        return nil
    }
    
    // Execute through executor
    if err := s.executor.Execute(ctx, run); err != nil {
        return fmt.Errorf("failed to execute DAG run: %w", err)
    }
    
    return nil
}

// GetStatus returns the current scheduler status
func (s *Scheduler) GetStatus(ctx context.Context) (*Status, error) {
    return &Status{
        Running:     s.running.Load(),
        Health:      s.health.GetHealth(),
        QueueDepth:  s.queue.GetDepth(ctx),
        ZombieCount: s.zombie.GetCount(ctx),
    }, nil
}
```

### 2. Queue Manager (queue/manager.go - ~150 lines)

```go
package queue

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// Manager handles DAG run queueing with concurrency control
type Manager interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Enqueue(ctx context.Context, run *DAGRun) error
    Dequeue(ctx context.Context) (*DAGRun, error)
    GetDepth(ctx context.Context) int
    GetStatus(ctx context.Context, name string) (*Status, error)
}

// Config holds queue manager configuration
type Config struct {
    DefaultMaxConcurrency int
    QueueTimeout          time.Duration
}

type manager struct {
    store         Store
    configs       sync.Map // map[string]*QueueConfig
    pending       sync.Map // map[string]int
    running       atomic.Bool
    config        *Config
}

// QueueConfig holds per-queue configuration
type QueueConfig struct {
    MaxConcurrency int
}

// NewManager creates a new queue manager
func NewManager(store Store, cfg *Config) Manager {
    return &manager{
        store:  store,
        config: cfg,
    }
}

// Start begins queue processing
func (m *manager) Start(ctx context.Context) error {
    if !m.running.CompareAndSwap(false, true) {
        return ErrAlreadyRunning
    }
    return nil
}

// Stop stops queue processing
func (m *manager) Stop(ctx context.Context) error {
    if !m.running.CompareAndSwap(true, false) {
        return ErrNotRunning
    }
    return nil
}

// Enqueue adds a DAG run to the queue
func (m *manager) Enqueue(ctx context.Context, run *DAGRun) error {
    if !m.running.Load() {
        return ErrNotRunning
    }
    
    // Check queue capacity
    queueName := run.Queue
    if queueName == "" {
        queueName = "default"
    }
    
    if !m.canEnqueue(queueName) {
        return ErrQueueFull
    }
    
    // Store in queue
    if err := m.store.Add(ctx, queueName, run); err != nil {
        return fmt.Errorf("failed to add to queue: %w", err)
    }
    
    // Update pending count
    m.incrementPending(queueName)
    
    return nil
}

// Dequeue retrieves the next DAG run from the queue
func (m *manager) Dequeue(ctx context.Context) (*DAGRun, error) {
    if !m.running.Load() {
        return nil, ErrNotRunning
    }
    
    // Get next run from store
    run, err := m.store.Next(ctx)
    if err != nil {
        if errors.Is(err, ErrEmpty) {
            return nil, nil
        }
        return nil, err
    }
    
    // Update pending count
    if run != nil {
        m.decrementPending(run.Queue)
    }
    
    return run, nil
}

// GetDepth returns the total queue depth
func (m *manager) GetDepth(ctx context.Context) int {
    depth, _ := m.store.Count(ctx)
    return depth
}

// GetStatus returns the status of a specific queue
func (m *manager) GetStatus(ctx context.Context, name string) (*Status, error) {
    cfg := m.getQueueConfig(name)
    pending := m.getPendingCount(name)
    
    return &Status{
        Name:           name,
        MaxConcurrency: cfg.MaxConcurrency,
        Pending:        pending,
    }, nil
}

// canEnqueue checks if the queue can accept more runs
func (m *manager) canEnqueue(queueName string) bool {
    cfg := m.getQueueConfig(queueName)
    pending := m.getPendingCount(queueName)
    return pending < cfg.MaxConcurrency
}

// getQueueConfig returns the configuration for a queue
func (m *manager) getQueueConfig(name string) *QueueConfig {
    if cfg, ok := m.configs.Load(name); ok {
        return cfg.(*QueueConfig)
    }
    return &QueueConfig{
        MaxConcurrency: m.config.DefaultMaxConcurrency,
    }
}

// incrementPending increments the pending count for a queue
func (m *manager) incrementPending(queueName string) {
    val, _ := m.pending.LoadOrStore(queueName, 0)
    count := val.(int)
    m.pending.Store(queueName, count+1)
}

// decrementPending decrements the pending count for a queue
func (m *manager) decrementPending(queueName string) {
    val, ok := m.pending.Load(queueName)
    if !ok {
        return
    }
    count := val.(int)
    if count > 0 {
        m.pending.Store(queueName, count-1)
    }
}

// getPendingCount returns the pending count for a queue
func (m *manager) getPendingCount(queueName string) int {
    val, ok := m.pending.Load(queueName)
    if !ok {
        return 0
    }
    return val.(int)
}

// Store interface for queue persistence
type Store interface {
    Add(ctx context.Context, queue string, run *DAGRun) error
    Next(ctx context.Context) (*DAGRun, error)
    Count(ctx context.Context) (int, error)
}
```

### 3. Health Monitor (health/monitor.go - ~100 lines)

```go
package health

import (
    "context"
    "fmt"
    "net/http"
    "sync/atomic"
    "time"
)

// Monitor provides health checking for the scheduler
type Monitor interface {
    Start(ctx context.Context) error
    Stop() error
    GetHealth() Status
    RecordCheck(name string, healthy bool)
}

// Status represents health check status
type Status struct {
    Healthy    bool
    Checks     map[string]bool
    LastCheck  time.Time
}

type monitor struct {
    server     *http.Server
    port       int
    checks     sync.Map
    healthy    atomic.Bool
    lastCheck  atomic.Value
}

// NewMonitor creates a new health monitor
func NewMonitor(port int) Monitor {
    return &monitor{
        port: port,
    }
}

// Start starts the health check server
func (m *monitor) Start(ctx context.Context) error {
    m.healthy.Store(true)
    m.lastCheck.Store(time.Now())
    
    mux := http.NewServeMux()
    mux.HandleFunc("/health", m.healthHandler)
    mux.HandleFunc("/readiness", m.readinessHandler)
    
    m.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", m.port),
        Handler: mux,
    }
    
    go func() {
        if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Error("health server error", "error", err)
        }
    }()
    
    return nil
}

// Stop stops the health check server
func (m *monitor) Stop() error {
    if m.server == nil {
        return nil
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    return m.server.Shutdown(ctx)
}

// GetHealth returns the current health status
func (m *monitor) GetHealth() Status {
    checks := make(map[string]bool)
    m.checks.Range(func(key, value interface{}) bool {
        checks[key.(string)] = value.(bool)
        return true
    })
    
    return Status{
        Healthy:   m.healthy.Load(),
        Checks:    checks,
        LastCheck: m.lastCheck.Load().(time.Time),
    }
}

// RecordCheck records a health check result
func (m *monitor) RecordCheck(name string, healthy bool) {
    m.checks.Store(name, healthy)
    m.lastCheck.Store(time.Now())
    
    // Update overall health
    allHealthy := true
    m.checks.Range(func(key, value interface{}) bool {
        if !value.(bool) {
            allHealthy = false
            return false
        }
        return true
    })
    m.healthy.Store(allHealthy)
}

// healthHandler handles /health endpoint
func (m *monitor) healthHandler(w http.ResponseWriter, r *http.Request) {
    status := m.GetHealth()
    if status.Healthy {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    fmt.Fprintf(w, `{"healthy": %t, "lastCheck": "%s"}`,
        status.Healthy, status.LastCheck.Format(time.RFC3339))
}

// readinessHandler handles /readiness endpoint
func (m *monitor) readinessHandler(w http.ResponseWriter, r *http.Request) {
    // Always ready if server is running
    w.WriteHeader(http.StatusOK)
    fmt.Fprint(w, `{"ready": true}`)
}
```

### 4. Zombie Detector (zombie/detector.go - ~100 lines)

```go
package zombie

import (
    "context"
    "sync/atomic"
    "time"
)

// Detector detects and handles zombie DAG runs
type Detector interface {
    Start(ctx context.Context) error
    Stop() error
    Scan(ctx context.Context) ([]ZombieRun, error)
    GetCount(ctx context.Context) int
}

// ZombieRun represents a detected zombie run
type ZombieRun struct {
    ID        string
    DAGName   string
    StartedAt time.Time
    Reason    string
}

type detector struct {
    store    Store
    interval time.Duration
    running  atomic.Bool
    cancel   context.CancelFunc
    count    atomic.Int64
}

// Store interface for zombie detection
type Store interface {
    FindStaleRuns(ctx context.Context, threshold time.Duration) ([]*DAGRun, error)
    FindOrphanedRuns(ctx context.Context) ([]*DAGRun, error)
    MarkAsZombie(ctx context.Context, runID string) error
}

// NewDetector creates a new zombie detector
func NewDetector(store Store, interval time.Duration) Detector {
    return &detector{
        store:    store,
        interval: interval,
    }
}

// Start begins zombie detection
func (d *detector) Start(ctx context.Context) error {
    if !d.running.CompareAndSwap(false, true) {
        return ErrAlreadyRunning
    }
    
    ctx, d.cancel = context.WithCancel(ctx)
    go d.detectLoop(ctx)
    
    return nil
}

// Stop stops zombie detection
func (d *detector) Stop() error {
    if !d.running.CompareAndSwap(true, false) {
        return ErrNotRunning
    }
    
    if d.cancel != nil {
        d.cancel()
    }
    
    return nil
}

// Scan scans for zombie runs
func (d *detector) Scan(ctx context.Context) ([]ZombieRun, error) {
    var zombies []ZombieRun
    
    // Find stale runs (running too long)
    staleRuns, err := d.store.FindStaleRuns(ctx, 24*time.Hour)
    if err != nil {
        return nil, fmt.Errorf("failed to find stale runs: %w", err)
    }
    
    for _, run := range staleRuns {
        zombies = append(zombies, ZombieRun{
            ID:        run.ID,
            DAGName:   run.DAG.Name,
            StartedAt: run.StartedAt,
            Reason:    "stale",
        })
        
        if err := d.store.MarkAsZombie(ctx, run.ID); err != nil {
            log.Error("failed to mark zombie", "runID", run.ID, "error", err)
        }
    }
    
    // Find orphaned runs (no process)
    orphanedRuns, err := d.store.FindOrphanedRuns(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to find orphaned runs: %w", err)
    }
    
    for _, run := range orphanedRuns {
        zombies = append(zombies, ZombieRun{
            ID:        run.ID,
            DAGName:   run.DAG.Name,
            StartedAt: run.StartedAt,
            Reason:    "orphaned",
        })
        
        if err := d.store.MarkAsZombie(ctx, run.ID); err != nil {
            log.Error("failed to mark zombie", "runID", run.ID, "error", err)
        }
    }
    
    d.count.Store(int64(len(zombies)))
    return zombies, nil
}

// GetCount returns the current zombie count
func (d *detector) GetCount(ctx context.Context) int {
    return int(d.count.Load())
}

// detectLoop is the main detection loop
func (d *detector) detectLoop(ctx context.Context) {
    ticker := time.NewTicker(d.interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if _, err := d.Scan(ctx); err != nil {
                log.Error("zombie scan failed", "error", err)
            }
        }
    }
}
```

## Benefits of Decomposition

### 1. Improved Testability
- Each component can be tested independently
- Easy to create mocks for interfaces
- Focused unit tests

### 2. Better Separation of Concerns
- Each component has a single responsibility
- Clear interfaces between components
- Easier to understand

### 3. Enhanced Maintainability
- Smaller files are easier to navigate
- Changes are localized
- Less risk of regression

### 4. Flexibility
- Components can be replaced independently
- Easier to add new features
- Better support for different deployment modes

## Migration Path

### Step 1: Extract Interfaces
```bash
# Create new packages
mkdir -p internal/service/scheduler/{queue,health,zombie,executor}

# Define interfaces first
# Implement adapters for existing code
```

### Step 2: Create Implementations
```bash
# Implement new components
# Write comprehensive tests
# Ensure feature parity
```

### Step 3: Wire Together
```bash
# Update scheduler constructor
# Add dependency injection
# Update tests
```

### Step 4: Gradual Rollout
```bash
# Feature flag for new implementation
# Monitor metrics
# Gradually increase traffic
# Remove old code when stable
```

## Testing Strategy

### Unit Tests
```go
func TestQueueManager_Enqueue(t *testing.T) {
    tests := []struct {
        name    string
        run     *DAGRun
        wantErr bool
    }{
        {
            name:    "enqueue new run",
            run:     &DAGRun{ID: "test-1"},
            wantErr: false,
        },
        {
            name:    "enqueue when queue full",
            run:     &DAGRun{ID: "test-2"},
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := NewMockStore(t)
            mgr := NewManager(store, &Config{})
            
            err := mgr.Enqueue(context.Background(), tt.run)
            if (err != nil) != tt.wantErr {
                t.Errorf("Enqueue() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Tests
```go
func TestScheduler_Integration(t *testing.T) {
    // Setup real components
    queueStore := setupQueueStore(t)
    queueMgr := queue.NewManager(queueStore, &queue.Config{})
    
    healthMon := health.NewMonitor(8080)
    zombieDet := zombie.NewDetector(queueStore, time.Minute)
    exec := executor.NewExecutor()
    
    scheduler := NewScheduler(queueMgr, healthMon, zombieDet, exec, nil, &Config{})
    
    // Test full flow
    ctx := context.Background()
    require.NoError(t, scheduler.Start(ctx))
    defer scheduler.Stop(ctx)
    
    dag := &DAG{Name: "test-dag"}
    require.NoError(t, scheduler.ScheduleDAG(ctx, dag))
    
    // Wait for execution
    time.Sleep(2 * time.Second)
    
    status, err := scheduler.GetStatus(ctx)
    require.NoError(t, err)
    assert.True(t, status.Health.Healthy)
}
```

## Performance Considerations

- **Memory**: Each component manages its own state, reducing contention
- **Concurrency**: Interfaces allow concurrent operations
- **Scalability**: Components can scale independently
- **Latency**: Minimal overhead from abstraction

## Conclusion

This decomposition transforms a 841-line god object into focused, testable components. Each component has a clear responsibility and clean interfaces, making the codebase more maintainable and extensible.
