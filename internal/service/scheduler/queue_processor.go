package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/backoff"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
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

		// init the queues
		activeQueues := make(map[string]struct{})
		for _, queueName := range queueList {
			if _, ok := p.queues.Load(queueName); !ok {
				p.queues.Store(queueName, &queue{
					maxConcurrency: 1,
					isGlobal:       false, // Dynamically created queues are DAG-based (not global)
				})
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
		logger.Warn(ctx, "Queue not found in processor config")
		return
	}
	q := v.(*queue)
	logger.Debug(ctx, "Processing queue",
		tag.MaxConcurrency(q.maxConc()),
	)

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

	alive, err := p.procStore.CountAlive(ctx, queueName)
	if err != nil {
		logger.Error(ctx, "Failed to count alive processes",
			tag.Error(err),
			tag.Queue(queueName),
		)
		return
	}

	maxConc := q.maxConc()
	free := maxConc - alive
	logger.Debug(ctx, "Queue capacity check",
		tag.MaxConcurrency(maxConc),
		tag.Alive(alive),
		tag.Count(free),
	)
	if free <= 0 {
		logger.Debug(ctx, "Max concurrency reached",
			tag.MaxConcurrency(maxConc),
			tag.Alive(alive),
		)
		return
	}

	cap := min(free, len(items))
	runnableItems := items[:cap]
	logger.Info(ctx, "Processing batch of items",
		tag.Count(len(runnableItems)),
		tag.MaxConcurrency(maxConc),
		tag.Alive(alive),
	)

	var wg sync.WaitGroup
	for _, item := range runnableItems {
		wg.Add(1)
		go func(ctx context.Context, item exec.QueuedItemData) {
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

func (p *QueueProcessor) processDAG(ctx context.Context, item exec.QueuedItemData, queueName string) bool {
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
		if errors.Is(err, exec.ErrDAGRunIDNotFound) {
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
		if errors.Is(err, exec.ErrCorruptedStatusFile) {
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

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		if err := p.dagExecutor.ExecuteDAG(ctx, dag, coordinatorv1.Operation_OPERATION_RETRY, runID, st); err != nil {
			logger.Error(ctx, "Failed to execute DAG", tag.Error(err))
		}
	}()

	// Use exponential backoff for retries for monitoring the execution start
	policy := backoff.NewExponentialBackoffPolicy(p.backoffConfig.InitialInterval)
	policy.MaxInterval = p.backoffConfig.MaxInterval
	policy.MaxRetries = p.backoffConfig.MaxRetries

	var started bool
	operation := func(ctx context.Context) error {
		started, err = p.monitorStartup(ctx, queueName, runRef)
		return err
	}

	if err := backoff.Retry(ctx, operation, policy, nil); err != nil {
		logger.Error(ctx, "Failed to execute DAG after retries", tag.Error(err))
	}

	wg.Wait()

	// Successfully dispatched/started, remove from queue
	return started
}

func (p *QueueProcessor) wakeUp() {
	select {
	case p.wakeUpCh <- struct{}{}:
	default:
	}
}

func (p *QueueProcessor) monitorStartup(ctx context.Context, queueName string, runRef exec.DAGRunRef) (bool, error) {
	select {
	case <-ctx.Done():
		logger.Debug(ctx, "Context canceled")
		return false, backoff.PermanentError(ctx.Err())
	case <-p.quit:
		logger.Info(ctx, "Processor is closed")
		return false, backoff.PermanentError(errProcessorClosed)
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
