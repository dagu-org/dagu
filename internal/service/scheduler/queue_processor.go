package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/backoff"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

var (
	errProcessorClosed = errors.New("processor closed")
	errNotStarted      = errors.New("execution not started")
)

// BackoffConfig holds configuration for exponential backoff retry logic.
type BackoffConfig struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	MaxRetries      int
}

// DefaultBackoffConfig returns the default backoff configuration.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     60 * time.Second,
		MaxRetries:      10,
	}
}

// QueueProcessor is responsible for processing queued DAG runs.
type QueueProcessor struct {
	queueStore    exec.QueueStore
	dagRunStore   exec.DAGRunStore
	procStore     exec.ProcStore
	dagExecutor   *DAGExecutor
	queues        sync.Map // map[string]*queue
	wakeUpCh      chan struct{}
	quit          chan struct{}
	wg            sync.WaitGroup
	stopOnce      sync.Once
	prevTime      time.Time
	lock          sync.Mutex
	backoffConfig BackoffConfig
}

type queue struct {
	maxConcurrency int
	isGlobal       bool // true if this queue is defined in config (global queue)
	inflight       atomic.Int32
	mu             sync.Mutex
}

func (q *queue) getMaxConcurrency() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.maxConcurrency
}

func (q *queue) isGlobalQueue() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.isGlobal
}

func (q *queue) getInflight() int {
	return int(q.inflight.Load())
}

func (q *queue) incInflight() { q.inflight.Add(1) }
func (q *queue) decInflight() { q.inflight.Add(-1) }

// QueueProcessorOption is a functional option for configuring QueueProcessor.
type QueueProcessorOption func(*QueueProcessor)

// WithBackoffConfig sets a custom backoff configuration for the processor.
func WithBackoffConfig(cfg BackoffConfig) QueueProcessorOption {
	return func(p *QueueProcessor) {
		p.backoffConfig = cfg
	}
}

// NewQueueProcessor creates a new QueueProcessor.
func NewQueueProcessor(
	queueStore exec.QueueStore,
	dagRunStore exec.DAGRunStore,
	procStore exec.ProcStore,
	dagExecutor *DAGExecutor,
	queuesConfig config.Queues,
	opts ...QueueProcessorOption,
) *QueueProcessor {
	p := &QueueProcessor{
		queueStore:    queueStore,
		dagRunStore:   dagRunStore,
		procStore:     procStore,
		dagExecutor:   dagExecutor,
		wakeUpCh:      make(chan struct{}, 1),
		quit:          make(chan struct{}),
		prevTime:      time.Now(),
		backoffConfig: DefaultBackoffConfig(),
	}

	for _, opt := range opts {
		opt(p)
	}

	for _, queueConfig := range queuesConfig.Config {
		conc := max(queueConfig.MaxActiveRuns, 1)
		p.queues.Store(queueConfig.Name, &queue{
			maxConcurrency: conc,
			isGlobal:       true, // Queues from config are global queues
		})
	}

	return p
}

// Start starts the queue processor.
func (p *QueueProcessor) Start(ctx context.Context, notifyCh <-chan struct{}) {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Start the main loop of the processor
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.loop(ctx)
	}()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.quit:
				return
			case <-notifyCh:
				p.wakeUp()
			}
		}
	}()

	p.wakeUp() // initial execution
}

// Stop stops the queue processor.
func (p *QueueProcessor) Stop() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.stopOnce.Do(func() {
		close(p.quit)
		p.wg.Wait()
	})
}

func (p *QueueProcessor) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.quit:
			return
		case <-p.wakeUpCh:
		case <-time.After(30 * time.Second):
			// wake up the queue processor on interval in case event is missed
		}

		// Prevent too frequent execution
		select {
		case <-ctx.Done():
			return
		case <-p.quit:
			return
		case <-time.After(time.Until(p.prevTime.Add(3 * time.Second))):
			p.prevTime = time.Now()
		}

		// Now process each queue
		queueList, err := p.queueStore.QueueList(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to get queue list", tag.Error(err))
			continue
		}

		// Initialize queues that don't exist yet
		activeQueues := make(map[string]struct{}, len(queueList))
		for _, queueName := range queueList {
			if _, ok := p.queues.Load(queueName); !ok {
				p.queues.Store(queueName, &queue{
					maxConcurrency: 1,
					isGlobal:       false,
				})
			}
			activeQueues[queueName] = struct{}{}
		}

		// Remove inactive non-global queues
		p.removeInactiveQueues(activeQueues)

		// Process each queue concurrently
		var wg sync.WaitGroup
		for name := range activeQueues {
			wg.Add(1)
			go func(queueName string) {
				defer wg.Done()
				queueCtx := logger.WithValues(ctx, tag.Queue(queueName))
				p.ProcessQueueItems(queueCtx, queueName)
			}(name)
		}
		wg.Wait()
	}
}

func (p *QueueProcessor) isClosed() bool {
	select {
	case <-p.quit:
		return true
	default:
		return false
	}
}

// ProcessQueueItems processes items in the specified queue.
// It returns true if there are more items to process in the queue.
func (p *QueueProcessor) ProcessQueueItems(ctx context.Context, queueName string) {
	if p.isClosed() {
		return
	}

	v, ok := p.queues.Load(queueName)
	if !ok {
		logger.Warn(ctx, "Queue not found in processor config")
		return
	}
	q := v.(*queue)
	logger.Debug(ctx, "Processing queue", tag.MaxConcurrency(q.getMaxConcurrency()))

	items, err := p.queueStore.List(ctx, queueName)
	if err != nil {
		logger.Error(ctx, "Failed to get queued items", tag.Error(err))
		return
	}

	if len(items) == 0 {
		logger.Debug(ctx, "No item found")
		return
	}

	defer p.wakeUp()

	aliveCount, err := p.procStore.CountAlive(ctx, queueName)
	if err != nil {
		logger.Error(ctx, "Failed to count alive processes", tag.Error(err), tag.Queue(queueName))
		return
	}

	maxConcurrency := q.getMaxConcurrency()
	inflightCount := q.getInflight()
	freeSlots := maxConcurrency - aliveCount - inflightCount

	logger.Debug(ctx, "Queue capacity check",
		tag.MaxConcurrency(maxConcurrency),
		tag.Alive(aliveCount),
		tag.Count(freeSlots),
	)

	if freeSlots <= 0 {
		logger.Debug(ctx, "Max concurrency reached",
			tag.MaxConcurrency(maxConcurrency),
			tag.Alive(aliveCount),
		)
		return
	}

	batchSize := min(freeSlots, len(items))
	runnableItems := items[:batchSize]
	logger.Info(ctx, "Processing batch of items",
		tag.Count(len(runnableItems)),
		tag.MaxConcurrency(maxConcurrency),
		tag.Alive(aliveCount),
	)

	var wg sync.WaitGroup
	for _, item := range runnableItems {
		wg.Add(1)
		go func(queuedItem exec.QueuedItemData) {
			defer wg.Done()
			if !p.processDAG(ctx, queuedItem, queueName, q.incInflight, q.decInflight) {
				return
			}
			data, err := queuedItem.Data()
			if err != nil {
				logger.Error(ctx, "Failed to get item data", tag.Error(err))
				return
			}
			if _, err := p.queueStore.DequeueByDAGRunID(ctx, queueName, *data); err != nil {
				logger.Error(ctx, "Failed to dequeue item", tag.Error(err))
			}
		}(item)
	}
	wg.Wait()
}

func (p *QueueProcessor) processDAG(ctx context.Context, item exec.QueuedItemData, queueName string, incInflight, decInflight func()) bool {
	if p.isClosed() {
		return false
	}

	data, err := item.Data()
	if err != nil {
		logger.Error(ctx, "Failed to get item data", tag.Error(err))
		return false
	}

	runRef := *data
	runID := runRef.ID
	ctx = logger.WithValues(ctx, tag.RunID(runID))
	logger.Debug(ctx, "Processing queue item", tag.Name(runRef.Name))

	running, err := p.procStore.IsRunAlive(ctx, queueName, runRef)
	if err != nil {
		logger.Error(ctx, "Failed to check if run is alive", tag.Error(err))
		return false
	}
	if running {
		logger.Warn(ctx, "DAG run is already running, discarding")
		return true
	}

	attempt, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		if errors.Is(err, exec.ErrDAGRunIDNotFound) {
			logger.Error(ctx, "DAG run not found, discarding")
			return true
		}
		logger.Error(ctx, "Failed to find run", tag.Error(err))
		return false
	}

	if attempt.Hidden() {
		logger.Info(ctx, "DAG run is hidden, discarding")
		return true
	}

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		if errors.Is(err, exec.ErrCorruptedStatusFile) {
			logger.Error(ctx, "Status file is corrupted, marking as invalid", tag.Error(err))
			return true
		}
		logger.Error(ctx, "Failed to read status", tag.Error(err))
		return false
	}

	if status.Status != core.Queued {
		logger.Info(ctx, "Status is not queued, skipping", tag.Status(status.Status.String()))
		return true
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG", tag.Error(err), tag.DAG(runRef.Name))
		return false
	}

	incInflight()
	defer decInflight()

	go func() {
		scheduledTime, _ := stringutil.ParseTime(status.ScheduledTime)
		if err := p.dagExecutor.ExecuteDAG(ctx, dag, coordinatorv1.Operation_OPERATION_RETRY, runID, status, status.TriggerType, scheduledTime); err != nil {
			logger.Error(ctx, "Failed to execute DAG", tag.Error(err))
		}
	}()

	return p.waitForStartup(ctx, queueName, runRef)
}

func (p *QueueProcessor) wakeUp() {
	select {
	case p.wakeUpCh <- struct{}{}:
	default:
	}
}

// removeInactiveQueues removes queues that are no longer active, preserving global queues from config.
func (p *QueueProcessor) removeInactiveQueues(activeQueues map[string]struct{}) {
	var toDelete []string
	p.queues.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok {
			return true
		}
		q, ok := value.(*queue)
		if !ok || q.isGlobalQueue() {
			return true
		}
		if _, active := activeQueues[name]; !active {
			toDelete = append(toDelete, name)
		}
		return true
	})
	for _, name := range toDelete {
		p.queues.Delete(name)
	}
}

// waitForStartup waits for the DAG execution to start using exponential backoff.
func (p *QueueProcessor) waitForStartup(ctx context.Context, queueName string, runRef exec.DAGRunRef) bool {
	policy := backoff.NewExponentialBackoffPolicy(p.backoffConfig.InitialInterval)
	policy.MaxInterval = p.backoffConfig.MaxInterval
	policy.MaxRetries = p.backoffConfig.MaxRetries

	var started bool
	operation := func(ctx context.Context) error {
		var err error
		started, err = p.checkStartupStatus(ctx, queueName, runRef)
		return err
	}

	if err := backoff.Retry(ctx, operation, policy, nil); err != nil {
		logger.Error(ctx, "Failed to execute DAG after retries", tag.Error(err))
	}

	return started
}

// checkStartupStatus checks if the DAG execution has started.
func (p *QueueProcessor) checkStartupStatus(ctx context.Context, queueName string, runRef exec.DAGRunRef) (bool, error) {
	if err := p.checkContextAndQuit(ctx); err != nil {
		return false, err
	}

	isAlive, err := p.procStore.IsRunAlive(ctx, queueName, runRef)
	if err != nil {
		logger.Warn(ctx, "Failed to check run liveness", tag.Error(err), tag.Queue(queueName), tag.RunID(runRef.ID))
	} else if isAlive {
		logger.Info(ctx, "DAG run has started (heartbeat detected)")
		return true, nil
	}

	attempt, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		logger.Debug(ctx, "Failed to read attempt, keep checking")
		return false, err
	}

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return false, err
	}

	if status.Status != core.Queued && status.Status != core.Running {
		logger.Info(ctx, "DAG execution started or finished", tag.Status(status.Status.String()))
		return true, nil
	}

	return false, errNotStarted
}

// checkContextAndQuit returns a permanent error if context is done or processor is closed.
func (p *QueueProcessor) checkContextAndQuit(ctx context.Context) error {
	select {
	case <-ctx.Done():
		logger.Debug(ctx, "Context canceled")
		return backoff.PermanentError(ctx.Err())
	case <-p.quit:
		logger.Info(ctx, "Processor is closed")
		return backoff.PermanentError(errProcessorClosed)
	default:
		return nil
	}
}
