package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

var (
	InitialBackoffInterval = 500 * time.Millisecond
	MaxBackoffInterval     = 60 * time.Second
	MaxBackoffRetries      = 10
)

var (
	errProcessorClosed = errors.New("processor closed")
	errNotStarted      = errors.New("execution not started")
)

// QueueProcessor is responsible for processing queued DAG runs.
type QueueProcessor struct {
	queueStore  execution.QueueStore
	dagRunStore execution.DAGRunStore
	procStore   execution.ProcStore
	dagExecutor *DAGExecutor
	queues      sync.Map // map[string]*queue
	wakeUpCh    chan struct{}
	quit        chan struct{}
	wg          sync.WaitGroup
	stopOnce    sync.Once
	prevTime    time.Time
	lock        sync.Mutex
}

type queue struct {
	maxConcurrency int
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

// NewQueueProcessor creates a new QueueProcessor.
func NewQueueProcessor(
	queueStore execution.QueueStore,
	dagRunStore execution.DAGRunStore,
	procStore execution.ProcStore,
	dagExecutor *DAGExecutor,
	queuesConfig config.Queues,
) *QueueProcessor {
	p := &QueueProcessor{
		queueStore:  queueStore,
		dagRunStore: dagRunStore,
		procStore:   procStore,
		dagExecutor: dagExecutor,
		wakeUpCh:    make(chan struct{}, 1),
		quit:        make(chan struct{}),
		prevTime:    time.Now(),
	}

	for _, queueConfig := range queuesConfig.Config {
		conc := max(queueConfig.MaxActiveRuns, 1)
		p.queues.Store(queueConfig.Name, &queue{
			maxConcurrency: conc,
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
			logger.Error(ctx, "queue: Failed to get queue list", "err", err)
			continue
		}

		// init the queues
		activeQueues := make(map[string]struct{})
		for _, queueName := range queueList {
			if _, ok := p.queues.Load(queueName); !ok {
				p.queues.Store(queueName, &queue{
					maxConcurrency: 1,
				})
			}
			activeQueues[queueName] = struct{}{}
		}

		// clean up non active queues
		nonActive := make(map[string]struct{})
		p.queues.Range(func(key, _ any) bool {
			name, ok := key.(string)
			if !ok {
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
				ctx := logger.WithValues(ctx, "name", name)
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
		logger.Warn(ctx, "queue: Queue not found in processor config", "queue", queueName)
		return
	}
	q := v.(*queue)

	items, err := p.queueStore.List(ctx, queueName)
	if err != nil {
		logger.Error(ctx, "queue: Failed to get queued items", "queue", queueName, "err", err)
		return
	}

	if len(items) == 0 {
		logger.Debug(ctx, "queue: no item found")
		return
	}

	defer p.wakeUp()

	alive, err := p.procStore.CountAlive(ctx, queueName)
	if err != nil {
		logger.Error(ctx, "queue: Failed to count alive processes", "err", err)
		return
	}

	maxConc := q.maxConc()
	free := maxConc - alive
	if free <= 0 {
		logger.Debug(ctx, "queue: Max concurrency reached", "max", maxConc, "alive", alive)
		return
	}

	cap := min(free, len(items))
	runnableItems := items[:cap]
	logger.Info(ctx, "queue: Processing batch of items", "count", len(runnableItems), "maxConcurrency", maxConc, "alive", alive)

	var wg sync.WaitGroup
	for _, item := range runnableItems {
		wg.Add(1)
		go func(ctx context.Context, item execution.QueuedItemData) {
			defer wg.Done()
			ctx = logger.WithValues(ctx, "runId", item.Data().ID)
			if p.processDAG(ctx, item, queueName) {
				// Remove the item from the queue
				if _, err := p.queueStore.DequeueByDAGRunID(ctx, queueName, item.Data().ID); err != nil {
					logger.Error(ctx, "queue: Failed to dequeue item", "runId", item.Data().ID, "err", err)
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

	runRef := item.Data()
	runID := runRef.ID
	logger.Info(ctx, "queue: Received item", "name", runRef.Name, "runId", runID)

	// Check if the DAG run is already running
	if running, err := p.procStore.IsRunAlive(ctx, queueName, runRef); err != nil {
		logger.Error(ctx, "queue: Failed to check if run is alive", "runId", runID, "err", err)
		return false
	} else if running {
		logger.Warn(ctx, "queue: DAG run is already running, discarding", "runId", runID)
		return true // Discarded, so it's "processed" from the queue's perspective
	}

	// Fetch the DAG of the dag-run attempt first to get queue configuration
	attempt, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		logger.Error(ctx, "queue: Failed to find run", "err", err)
		// If the attempt doesn't exist at all, mark as discard
		if errors.Is(err, execution.ErrDAGRunIDNotFound) {
			logger.Error(ctx, "queue: DAG run not found, discarding", "runId", runID)
			return true
		}
		return false
	}

	if attempt.Hidden() {
		logger.Info(ctx, "queue: DAG run is hidden, discarding", "runId", runID)
		return true
	}

	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		if errors.Is(err, execution.ErrCorruptedStatusFile) {
			logger.Error(ctx, "queue: Status file is corrupted, marking as invalid", "err", err, "runId", runID)
			return true
		} else {
			logger.Error(ctx, "queue: Failed to read status", "err", err, "runId", runID)
		}
		return false
	}

	if st.Status != core.Queued {
		logger.Info(ctx, "queue: Status is not queued, skipping", "status", st.Status, "runId", runID)
		return true
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "queue: Failed to read dag", "err", err, "runId", runID)
		return false
	}

	// Update the queue configuration with the latest execution
	queueVal, _ := p.queues.Load(queueName)
	queue := queueVal.(*queue)
	queue.setMaxConc(dag.MaxActiveRuns)

	errCh := make(chan error, 1)
	if err := p.dagExecutor.ExecuteDAG(ctx, dag, coordinatorv1.Operation_OPERATION_RETRY, runID); err != nil {
		logger.Error(ctx, "queue: Failed to execute dag", "runId", runID, "err", err)
		errCh <- err
		return false
	}

	// Use exponential backoff for retries for monitoring the execution start
	policy := backoff.NewExponentialBackoffPolicy(InitialBackoffInterval)
	policy.MaxInterval = MaxBackoffInterval
	policy.MaxRetries = MaxBackoffRetries

	var started bool
	operation := func(ctx context.Context) error {
		started, err = p.monitorStartup(ctx, queueName, runRef, nil)
		return err
	}

	if err := backoff.Retry(ctx, operation, policy, nil); err != nil {
		logger.Error(ctx, "queue: Failed to execute dag after retries", "runId", runID, "err", err)
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
		logger.Info(ctx, "queue: Context canceled")
		return false, backoff.PermanentError(ctx.Err())
	case <-p.quit:
		logger.Info(ctx, "queue: Processor is closed")
		return false, backoff.PermanentError(errProcessorClosed)
	case err := <-errCh:
		logger.Info(ctx, "queue: Failed to execute the DAG", "err", err)
		return false, backoff.PermanentError(err)
	default:
	}

	// Check if the process is alive (has heartbeat)
	isAlive, err := p.procStore.IsRunAlive(ctx, queueName, runRef)
	if err != nil {
		// Continue checking on error
	} else if isAlive {
		logger.Info(ctx, "queue: DAG run has started (heartbeat detected)")
		return true, nil
	}

	att, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		logger.Debug(ctx, "queue: Failed to read attempt. Keep checking")
		return false, err
	}

	status, err := att.ReadStatus(ctx)
	if err != nil {
		return false, err
	}

	if status.Status != core.Queued && status.Status != core.Running {
		logger.Info(ctx, "queue: DAG execution started or finished", "status", status.Status.String())
		return true, nil
	}

	return false, errNotStarted
}
