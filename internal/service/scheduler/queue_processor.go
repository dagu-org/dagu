package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
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
	queueStore    execution.QueueStore
	dagRunStore   execution.DAGRunStore
	procStore     execution.ProcStore
	dagExecutor   *DAGExecutor
	queues        sync.Map      // map[string]*queue
	queuesConfig  config.Queues // Store config to check for global queues when auto-creating
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
	isGlobal       bool // true if this is a global queue from config, false if DAG-based
	mu             sync.Mutex
}

func (q *queue) maxConc() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.maxConcurrency
}

func (q *queue) setMaxConc(val int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.maxConcurrency = max(val, 1)
}

func (q *queue) isGlobalQueue() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.isGlobal
}

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
	queueStore execution.QueueStore,
	dagRunStore execution.DAGRunStore,
	procStore execution.ProcStore,
	dagExecutor *DAGExecutor,
	queuesConfig config.Queues,
	opts ...QueueProcessorOption,
) *QueueProcessor {
	p := &QueueProcessor{
		queueStore:    queueStore,
		dagRunStore:   dagRunStore,
		procStore:     procStore,
		dagExecutor:   dagExecutor,
		queuesConfig:  queuesConfig, // Store config for later use
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
			isGlobal:       true, // Mark as global queue from config
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

		// init the queues
		activeQueues := make(map[string]struct{})
		for _, queueName := range queueList {
			existingQueue, exists := p.queues.Load(queueName)
			if !exists {
				// Check if this is a global queue from config
				isGlobal := false
				maxConc := 1
				for _, queueConfig := range p.queuesConfig.Config {
					if queueConfig.Name == queueName {
						isGlobal = true
						maxConc = max(queueConfig.MaxActiveRuns, 1)
						logger.Info(ctx, "Initializing global queue from config",
							tag.Queue(queueName),
							tag.MaxConcurrency(maxConc),
						)
						break
					}
				}
				if !isGlobal {
					logger.Info(ctx, "Initializing DAG-based queue (not in config)",
						tag.Queue(queueName),
						tag.MaxConcurrency(maxConc),
					)
				}
				p.queues.Store(queueName, &queue{
					maxConcurrency: maxConc,
					isGlobal:       isGlobal,
				})
			} else {
				// Queue exists, but check if config has been updated
				existingQ := existingQueue.(*queue)
				existingQ.mu.Lock()
				existingIsGlobal := existingQ.isGlobal
				existingQ.mu.Unlock()

				// If it's not a global queue, check if it should be now
				if !existingIsGlobal {
					for _, queueConfig := range p.queuesConfig.Config {
						if queueConfig.Name == queueName {
							// Update to global queue with config value
							newMaxConc := max(queueConfig.MaxActiveRuns, 1)
							existingQ.mu.Lock()
							existingQ.isGlobal = true
							existingQ.maxConcurrency = newMaxConc
							existingQ.mu.Unlock()
							logger.Info(ctx, "Updated queue to global queue from config",
								tag.Queue(queueName),
								tag.MaxConcurrency(newMaxConc),
							)
							break
						}
					}
				}
			}
			activeQueues[queueName] = struct{}{}
		}

		// clean up non active queues (but keep global queues from config)
		nonActive := make(map[string]struct{})
		p.queues.Range(func(key, value any) bool {
			name, ok := key.(string)
			if !ok {
				return true
			}
			q, ok := value.(*queue)
			if !ok {
				return true
			}
			// Don't delete global queues - they should persist even when empty
			if q.isGlobalQueue() {
				return true
			}
			if _, ok := activeQueues[name]; !ok {
				nonActive[name] = struct{}{}
			}
			return true
		})
		for name := range nonActive {
			p.queues.Delete(name)
		}

		// process queue items
		var wg sync.WaitGroup
		for name := range activeQueues {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				ctx := logger.WithValues(ctx, tag.Queue(name))
				p.ProcessQueueItems(ctx, name)
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

	// Get the queue configuration
	v, ok := p.queues.Load(queueName)
	if !ok {
		logger.Warn(ctx, "Queue not found in processor config",
			tag.Queue(queueName),
		)
		return
	}
	q := v.(*queue)

	// Log queue configuration for debugging
	q.mu.Lock()
	isGlobal := q.isGlobal
	q.mu.Unlock()

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

	// For non-global queues, update maxConcurrency from the first queued item's DAG
	// before calculating available slots. This ensures we use the DAG's configured
	// maxActiveRuns instead of the default value of 1.
	if !q.isGlobalQueue() {
		if err := p.updateQueueMaxConcurrency(ctx, q, items[0]); err != nil {
			logger.Warn(ctx, "Failed to update queue max concurrency from DAG",
				tag.Error(err),
			)
			// Continue with current maxConcurrency value
		}
	}

	alive, err := p.procStore.CountAlive(ctx, queueName)
	if err != nil {
		logger.Error(ctx, "Failed to count alive processes",
			tag.Error(err),
			tag.Queue(queueName),
		)
		return
	}

	maxConc := q.maxConc()
	logger.Debug(ctx, "Queue configuration",
		tag.Queue(queueName),
		tag.MaxConcurrency(maxConc),
		slog.Bool("isGlobal", isGlobal),
	)
	free := maxConc - alive
	logger.Debug(ctx, "Queue capacity check",
		tag.Queue(queueName),
		tag.MaxConcurrency(maxConc),
		tag.Alive(alive),
		slog.Int("free", free),
		slog.Int("queued", len(items)),
	)
	if free <= 0 {
		logger.Debug(ctx, "Max concurrency reached",
			tag.MaxConcurrency(maxConc),
			tag.Alive(alive),
		)
		return
	}

	// Select up to 'free' items from the queue to run concurrently
	// This ensures queue-level concurrency: maxConcurrency=n means n DAGs can run at once
	cap := min(free, len(items))
	runnableItems := items[:cap]
	logger.Info(ctx, "Processing batch of items",
		tag.Count(len(runnableItems)),
		tag.MaxConcurrency(maxConc),
		tag.Alive(alive),
		slog.Int("free", free),
	)

	var wg sync.WaitGroup
	for _, item := range runnableItems {
		wg.Add(1)
		go func(ctx context.Context, item execution.QueuedItemData) {
			defer wg.Done()
			if p.processDAG(ctx, item, queueName) {
				// Remove the item from the queue
				data, err := item.Data()
				if err != nil {
					logger.Error(ctx, "Failed to get item data", tag.Error(err))
					return
				}
				if _, err := p.queueStore.DequeueByDAGRunID(ctx, queueName, *data); err != nil {
					logger.Error(ctx, "Failed to dequeue item", tag.Error(err))
				}
			}
		}(ctx, item)
	}
	wg.Wait()
}

func (p *QueueProcessor) processDAG(ctx context.Context, item execution.QueuedItemData, queueName string) bool {
	if p.isClosed() {
		return false
	}

	data, err := item.Data()
	if err != nil {
		logger.Error(ctx, "Failed to get item data", tag.Error(err))
		return false
	}

	ctx = logger.WithValues(ctx, tag.RunID(data.ID))

	runRef := *data
	runID := runRef.ID
	_ = runID // used below
	logger.Debug(ctx, "Processing queue item",
		tag.Name(runRef.Name),
	)

	// Check if the DAG run is already running
	if running, err := p.procStore.IsRunAlive(ctx, queueName, runRef); err != nil {
		logger.Error(ctx, "Failed to check if run is alive", tag.Error(err))
		return false
	} else if running {
		logger.Warn(ctx, "DAG run is already running, discarding")
		return true // Discarded, so it's "processed" from the queue's perspective
	}

	// Fetch the DAG of the dag-run attempt first to get queue configuration
	attempt, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		logger.Error(ctx, "Failed to find run", tag.Error(err))
		// If the attempt doesn't exist at all, mark as discard
		if errors.Is(err, execution.ErrDAGRunIDNotFound) {
			logger.Error(ctx, "DAG run not found, discarding")
			return true
		}
		return false
	}

	if attempt.Hidden() {
		logger.Info(ctx, "DAG run is hidden, discarding")
		return true
	}

	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		if errors.Is(err, execution.ErrCorruptedStatusFile) {
			logger.Error(ctx, "Status file is corrupted, marking as invalid", tag.Error(err))
			return true
		} else {
			logger.Error(ctx, "Failed to read status", tag.Error(err))
		}
		return false
	}

	if st.Status != core.Queued {
		logger.Info(ctx, "Status is not queued, skipping",
			tag.Status(st.Status.String()),
		)
		return true
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG",
			tag.Error(err),
			tag.DAG(runRef.Name),
		)
		return false
	}

	// Update the queue configuration with the latest execution
	// Only update maxConcurrency for DAG-based queues, not global queues
	// Global queues preserve their configured maxConcurrency value
	queueVal, _ := p.queues.Load(queueName)
	queue := queueVal.(*queue)
	queue.mu.Lock()
	isGlobal := queue.isGlobal
	if !isGlobal {
		// For DAG-based queues, update maxConcurrency based on DAG's MaxActiveRuns
		// Use the maximum value to ensure we don't decrease concurrency
		currentMax := queue.maxConcurrency
		newMax := max(dag.MaxActiveRuns, 1)
		if newMax > currentMax {
			queue.maxConcurrency = newMax
		}
	}
	queue.mu.Unlock()
	// For global queues, preserve the configured maxConcurrency - don't update it

	errCh := make(chan error, 1)
	if err := p.dagExecutor.ExecuteDAG(ctx, dag, coordinatorv1.Operation_OPERATION_RETRY, runID); err != nil {
		logger.Error(ctx, "Failed to execute DAG", tag.Error(err))
		errCh <- err
		return false
	}

	// Use exponential backoff for retries for monitoring the execution start
	policy := backoff.NewExponentialBackoffPolicy(p.backoffConfig.InitialInterval)
	policy.MaxInterval = p.backoffConfig.MaxInterval
	policy.MaxRetries = p.backoffConfig.MaxRetries

	var started bool
	operation := func(ctx context.Context) error {
		started, err = p.monitorStartup(ctx, queueName, runRef, errCh)
		return err
	}

	if err := backoff.Retry(ctx, operation, policy, nil); err != nil {
		logger.Error(ctx, "Failed to execute DAG after retries", tag.Error(err))
	}

	// Successfully dispatched/started, remove from queue
	return started
}

func (p *QueueProcessor) wakeUp() {
	select {
	case p.wakeUpCh <- struct{}{}:
	default:
	}
}

func (p *QueueProcessor) monitorStartup(ctx context.Context, queueName string, runRef execution.DAGRunRef, errCh chan error) (bool, error) {
	select {
	case <-ctx.Done():
		logger.Debug(ctx, "Context canceled")
		return false, backoff.PermanentError(ctx.Err())
	case <-p.quit:
		logger.Info(ctx, "Processor is closed")
		return false, backoff.PermanentError(errProcessorClosed)
	case err := <-errCh:
		logger.Info(ctx, "Failed to execute the DAG", tag.Error(err))
		return false, backoff.PermanentError(err)
	default:
	}

	// Check if the process is alive (has heartbeat)
	isAlive, err := p.procStore.IsRunAlive(ctx, queueName, runRef)
	if err != nil {
		logger.Warn(ctx, "Failed to check run liveness",
			tag.Error(err),
			tag.Queue(queueName),
			tag.RunID(runRef.ID),
		)
		// Continue checking on error
	} else if isAlive {
		logger.Info(ctx, "DAG run has started (heartbeat detected)")
		return true, nil
	}

	att, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		logger.Debug(ctx, "Failed to read attempt, keep checking")
		return false, err
	}

	status, err := att.ReadStatus(ctx)
	if err != nil {
		return false, err
	}

	if status.Status != core.Queued && status.Status != core.Running {
		logger.Info(ctx, "DAG execution started or finished",
			tag.Status(status.Status.String()),
		)
		return true, nil
	}

	return false, errNotStarted
}

// updateQueueMaxConcurrency reads the DAG from a queued item and updates
// the queue's maxConcurrency based on the DAG's MaxActiveRuns setting.
// This is used to initialize non-global queues with the correct concurrency
// before calculating how many items to process.
func (p *QueueProcessor) updateQueueMaxConcurrency(ctx context.Context, q *queue, item execution.QueuedItemData) error {
	data, err := item.Data()
	if err != nil {
		return err
	}

	runRef := *data
	attempt, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		return err
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return err
	}

	q.setMaxConc(dag.MaxActiveRuns)
	return nil
}
