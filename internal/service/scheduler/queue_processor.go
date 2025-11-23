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
	errNotStarted      = errors.New("not started")
	errProcessorClosed = errors.New("processor closed")
)

type queueProcessor struct {
	queueStore   execution.QueueStore
	dagRunStore  execution.DAGRunStore
	procStore    execution.ProcStore
	queuesConfig config.Queues
	queues       sync.Map
	dagExecutor  *DAGExecutor
	quit         chan struct{}
	done         chan struct{}
	wg           sync.WaitGroup
	wait         chan struct{}
	previousTime time.Time
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

func newQueueProcessor(
	queueStore execution.QueueStore,
	dagRunStore execution.DAGRunStore,
	procStore execution.ProcStore,
	dagExecutor *DAGExecutor,
	queuesConfig config.Queues,
) *queueProcessor {
	p := &queueProcessor{
		queueStore:   queueStore,
		dagRunStore:  dagRunStore,
		procStore:    procStore,
		dagExecutor:  dagExecutor,
		queuesConfig: queuesConfig,
		quit:         make(chan struct{}),
		done:         make(chan struct{}),
		wait:         make(chan struct{}, 1),
	}

	for name, queueConfig := range queuesConfig.Config {
		conc := max(queueConfig.MaxActiveRuns, 1)
		p.queues.Store(name, &queue{
			maxConcurrency: conc,
		})
	}

	return p
}

func (p *queueProcessor) start(ctx context.Context, notifyCh <-chan struct{}) {
	p.previousTime = time.Now()

	// Start the main loop of the processor
	p.wg.Add(1)
	go func(ctx context.Context) {
		defer p.wg.Done()
		p.loop(ctx)
	}(ctx)

	// Start a file watcher to wake up the processor loop when a new item added
	p.wg.Add(1)
	go func(ctx context.Context) {
		defer p.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.quit:
				return
			case <-time.After(30 * time.Second):
				// wake up the queue processor on interval in case event is missed
				p.wakeUp()
			case <-notifyCh:
				p.wakeUp()
			}
		}
	}(ctx)

	p.wait <- struct{}{} // initial execution
}

func (p *queueProcessor) stop() {
	close(p.done)
	p.quit <- struct{}{}
	p.wg.Wait()
}

func (p *queueProcessor) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.quit:
			return
		case <-p.wait:
		}

		// Wait for a few seconds to avoid busy loop
		sincePrev := time.Since(p.previousTime)
		select {
		case <-ctx.Done():
			return
		case <-p.quit:
			return
		case <-time.After(max(0, time.Second*3-sincePrev)):
		}
		p.previousTime = time.Now()

		// Now process each queue
		queueList, err := p.queueStore.QueueList(ctx)
		if err != nil {
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
			go func(ctx context.Context, name string) {
				defer wg.Done()
				ctx = logger.WithValues(ctx, "name", name)
				p.processQueueItems(ctx, name)
			}(ctx, name)
		}
		wg.Wait()
	}
}

func (p *queueProcessor) processQueueItems(ctx context.Context, queueName string) {
	if p.isClosed(ctx) {
		return
	}

	items, err := p.queueStore.List(ctx, queueName)
	if err != nil {
		logger.Error(ctx, "queue: Failed to get queued items", "queue", queueName, "err", err)
	}

	if len(items) == 0 {
		logger.Debug(ctx, "queue: no item found")
	}

	v, _ := p.queues.Load(queueName) // must exist
	queue := v.(*queue)

	alive, err := p.procStore.CountAlive(ctx, queueName)
	if err != nil {
		logger.Error(ctx, "queue: Failed to count alive processes", "err", err)
		return
	}

	maxConc := queue.maxConc()
	free := maxConc - alive
	if free <= 0 {
		logger.Debug(ctx, "queue: Max concurrency reached", "max", queue.maxConcurrency, "alive", alive)
		p.wakeUp() // wake up later to process remaining items
		return
	}

	var wg sync.WaitGroup
	cap := min(free, len(items))
	if cap < len(items) {
		p.wakeUp() // wake up later to process remaining items
	}
	runnableItems := items[:cap]
	logger.Info(ctx, "queue: Processing batch of items", "count", len(runnableItems), "maxConcurrency", maxConc, "alive", alive)

	for _, item := range runnableItems {
		wg.Add(1)
		go func(ctx context.Context, runRef execution.DAGRunRef) {
			defer wg.Done()
			ctx = logger.WithValues(ctx, "runId", runRef.ID)
			ret := p.processDAG(ctx, queueName, runRef)
			if !ret {
				logger.Warn(ctx, "queue: Process DAG unfinished, keep the item in the queue")
				return
			}
			_, err := p.queueStore.DequeueByDAGRunID(ctx, queueName, runRef.ID)
			if err != nil && !errors.Is(err, execution.ErrQueueItemNotFound) {
				logger.Error(ctx, "queue: Failed to dequeue item", "err", err)
			}
		}(ctx, item.Data())
	}

	logger.Debug(ctx, "queue: Wait for the batch")
	wg.Wait()
}

func (p *queueProcessor) processDAG(ctx context.Context, queueName string, runRef execution.DAGRunRef) bool {
	if p.isClosed(ctx) {
		return false
	}

	logger.Info(ctx, "queue: Received item", "name", runRef.Name, "runId", runRef.ID)
	// Fetch the DAG of the dag-run attempt first to get queue configuration
	attempt, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		logger.Error(ctx, "queue: Failed to find run", "err", err)
		// If the attempt doesn't exist at all, mark as discard
		if errors.Is(err, execution.ErrDAGRunIDNotFound) {
			logger.Error(ctx, "queue: DAG run not found, discarding")
			return true
		}
		return false
	}

	if attempt.Hidden() {
		logger.Info(ctx, "queue: DAG run is hidden, discarding")
		return true
	}

	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		if errors.Is(err, execution.ErrCorruptedStatusFile) {
			logger.Error(ctx, "queue: Status file is corrupted, marking as invalid", "err", err)
			return true
		} else {
			logger.Error(ctx, "queue: Failed to read status", "err", err)
		}
		return false
	}

	if st.Status != core.Queued {
		logger.Info(ctx, "queue: Status is not queued, skipping", "status", st.Status)
		return true
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "queue: Failed to read dag", "err", err)
		return false
	}

	// Update the queue configuration with the latest execution
	queueVal, _ := p.queues.Load(queueName)
	queue := queueVal.(*queue)
	queue.setMaxConc(dag.MaxActiveRuns)

	errCh := make(chan error)
	defer close(errCh)

	go func() {
		defer func() {
			err := recover()
			logger.Error(ctx, "Panic recovered on executing a DAG", "err", err)
		}()

		err := p.dagExecutor.ExecuteDAG(ctx, dag, coordinatorv1.Operation_OPERATION_RETRY, runRef.ID)
		if err != nil {
			logger.Error(ctx, "queue: Failed to execute dag", "err", err)
		}
		select {
		case <-errCh:
		default:
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	// Successfully dispatched/started, remove from queue
	logger.Info(ctx, "queue: Triggered execution")

	// Wait for the DAG to be picked up by checking process heartbeat
	// This is just for monitoring/logging, not critical for queue logic anymore
	policy := backoff.NewExponentialBackoffPolicy(500 * time.Millisecond)
	policy.MaxInterval = time.Second * 10
	policy.MaxRetries = 7

	var done bool
	if err := backoff.Retry(ctx, func(ctx context.Context) error {
		var err error
		done, err = p.monitorStartup(ctx, queueName, runRef, errCh)
		return err
	}, policy, func(_ error) bool {
		return !p.isClosed(ctx)
	}); err != nil {
		return false
	}

	return done
}

func (p *queueProcessor) monitorStartup(ctx context.Context, queueName string, runRef execution.DAGRunRef, errCh chan error) (bool, error) {
	select {
	case <-ctx.Done():
		logger.Info(ctx, "queue: Context canceled")
		return false, ctx.Err()
	case <-p.done:
		logger.Info(ctx, "queue: Processor is closed")
		return false, errProcessorClosed
	case err := <-errCh:
		logger.Info(ctx, "queue: Failed to execute the DAG", "err", err)
		return false, err
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
		return true, err
	}

	return false, errNotStarted
}

func (p *queueProcessor) isClosed(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *queueProcessor) wakeUp() {
	select {
	case p.wait <- struct{}{}:
	default:
	}
}
