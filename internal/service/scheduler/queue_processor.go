package scheduler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

var errProcessorClosed = errors.New("processor closed")

// inFlightTTL is the time after which an in-flight item can be re-dispatched.
// This handles cases where a process crashes before establishing heartbeat.
const inFlightTTL = 10 * time.Minute

// QueueProcessor is responsible for processing queued DAG runs.
type QueueProcessor struct {
	queueStore  exec.QueueStore
	dagRunStore exec.DAGRunStore
	procStore   exec.ProcStore
	dagExecutor *DAGExecutor
	queues      sync.Map // map[string]*queue
	inFlight    sync.Map // map[string]time.Time - tracks dispatched items to prevent duplicate dispatch
	wakeUpCh    chan struct{}
	quit        chan struct{}
	wg          sync.WaitGroup
	stopOnce    sync.Once
	prevTime    time.Time
	lock        sync.Mutex
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

func (q *queue) isGlobalQueue() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.isGlobal
}

// NewQueueProcessor creates a new QueueProcessor.
func NewQueueProcessor(
	queueStore exec.QueueStore,
	dagRunStore exec.DAGRunStore,
	procStore exec.ProcStore,
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
	inFlightKey := queueName + "/" + runID

	logger.Debug(ctx, "Processing queue item", tag.Name(runRef.Name))

	// Check if the DAG run is already running (heartbeat detected)
	running, err := p.procStore.IsRunAlive(ctx, queueName, runRef)
	if err != nil {
		logger.Error(ctx, "Failed to check if run is alive", tag.Error(err))
		return false
	}
	if running {
		logger.Warn(ctx, "DAG run is already running, discarding")
		p.inFlight.Delete(inFlightKey) // Clean up if it was in-flight
		return true
	}

	// Fetch the DAG run attempt
	attempt, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		logger.Error(ctx, "Failed to find run", tag.Error(err))
		if errors.Is(err, exec.ErrDAGRunIDNotFound) {
			logger.Error(ctx, "DAG run not found, discarding")
			p.inFlight.Delete(inFlightKey)
			return true
		}
		return false
	}

	if attempt.Hidden() {
		logger.Info(ctx, "DAG run is hidden, discarding")
		p.inFlight.Delete(inFlightKey)
		return true
	}

	st, err := attempt.ReadStatus(ctx)
	if err != nil {
		if errors.Is(err, exec.ErrCorruptedStatusFile) {
			logger.Error(ctx, "Status file is corrupted, marking as invalid", tag.Error(err))
			p.inFlight.Delete(inFlightKey)
			return true
		}
		logger.Error(ctx, "Failed to read status", tag.Error(err))
		return false
	}

	if st.Status != core.Queued {
		logger.Info(ctx, "Status is not queued, skipping", tag.Status(st.Status.String()))
		p.inFlight.Delete(inFlightKey)
		return true
	}

	// Check if already dispatched (in-flight) - prevents duplicate ExecuteDAG calls
	if dispatchTime, alreadyDispatched := p.inFlight.Load(inFlightKey); alreadyDispatched {
		elapsed := time.Since(dispatchTime.(time.Time))
		if elapsed < inFlightTTL {
			// Recently dispatched, just check if startup confirmed
			logger.Debug(ctx, "Already in-flight, checking startup", tag.Duration(elapsed))
			started, err := p.checkStartup(ctx, queueName, runRef)
			if err != nil && !errors.Is(err, errProcessorClosed) {
				logger.Warn(ctx, "Failed to check startup", tag.Error(err))
			}
			if started {
				p.inFlight.Delete(inFlightKey)
			}
			return started
		}
		// TTL expired - process may have crashed, allow re-dispatch
		logger.Warn(ctx, "In-flight TTL expired, allowing re-dispatch", tag.Duration(elapsed))
		p.inFlight.Delete(inFlightKey)
	}

	// Read DAG for execution
	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG", tag.Error(err), tag.DAG(runRef.Name))
		return false
	}

	// Mark as in-flight BEFORE dispatching
	p.inFlight.Store(inFlightKey, time.Now())

	// Dispatch the DAG
	if err := p.dagExecutor.ExecuteDAG(ctx, dag, coordinatorv1.Operation_OPERATION_RETRY, runID, st); err != nil {
		logger.Error(ctx, "Failed to execute DAG", tag.Error(err))
		p.inFlight.Delete(inFlightKey) // Failed to dispatch, allow retry
		return false
	}

	// Single immediate check for startup - no blocking retry
	started, err := p.checkStartup(ctx, queueName, runRef)
	if err != nil {
		if errors.Is(err, errProcessorClosed) || errors.Is(err, context.Canceled) {
			return false // Shutdown - keep in queue, keep in-flight
		}
		logger.Warn(ctx, "Failed to check startup", tag.Error(err))
		return false // Error - keep in queue for next iteration
	}

	if started {
		p.inFlight.Delete(inFlightKey)
	}
	return started
}

func (p *QueueProcessor) wakeUp() {
	select {
	case p.wakeUpCh <- struct{}{}:
	default:
	}
}

// checkStartup checks if the DAG has started (heartbeat or status change).
// Returns true if startup confirmed (should dequeue), false if not yet started.
// This is a single immediate check - no retry loop.
func (p *QueueProcessor) checkStartup(ctx context.Context, queueName string, runRef exec.DAGRunRef) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-p.quit:
		return false, errProcessorClosed
	default:
	}

	// Check if the process is alive (has heartbeat)
	isAlive, err := p.procStore.IsRunAlive(ctx, queueName, runRef)
	if err != nil {
		logger.Warn(ctx, "Failed to check run liveness", tag.Error(err))
		// Continue to status check
	} else if isAlive {
		logger.Info(ctx, "DAG run has started (heartbeat detected)")
		return true, nil
	}

	// Check status file
	att, err := p.dagRunStore.FindAttempt(ctx, runRef)
	if err != nil {
		return false, err
	}

	status, err := att.ReadStatus(ctx)
	if err != nil {
		return false, err
	}

	// If status changed from Queued, the DAG has been processed
	if status.Status != core.Queued {
		logger.Info(ctx, "DAG execution started or finished",
			tag.Status(status.Status.String()),
		)
		return true, nil
	}

	// Not started yet - keep in queue for next iteration
	return false, nil
}
