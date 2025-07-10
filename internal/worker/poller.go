package worker

import (
	"context"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/backoff"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/google/uuid"
)

// pollState tracks the connection state for a poller
type pollState struct {
	mu               sync.Mutex
	isConnected      bool
	consecutiveFails int
	lastError        error
}

// Poller handles polling for tasks from the coordinator
type Poller struct {
	workerID        string
	coordinatorAddr string
	client          coordinatorv1.CoordinatorServiceClient
	taskExecutor    TaskExecutor
	index           int
	state           *pollState
	labels          map[string]string
}

// NewPoller creates a new poller instance
func NewPoller(workerID string, coordinatorAddr string, client coordinatorv1.CoordinatorServiceClient, taskExecutor TaskExecutor, index int, labels map[string]string) *Poller {
	return &Poller{
		workerID:        workerID,
		coordinatorAddr: coordinatorAddr,
		client:          client,
		taskExecutor:    taskExecutor,
		index:           index,
		state: &pollState{
			isConnected: true, // Assume connected initially
		},
		labels: labels,
	}
}

// Run starts the polling loop
func (p *Poller) Run(ctx context.Context) {
	// Set up retry policy for poll failures only
	basePolicy := backoff.NewExponentialBackoffPolicy(time.Second)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = time.Minute
	basePolicy.MaxRetries = 0 // Retry indefinitely

	policy := backoff.WithJitter(basePolicy, backoff.FullJitter)

	for {
		select {
		case <-ctx.Done():
			logger.Debug(ctx, "Poller stopping due to context cancellation",
				"worker_id", p.workerID,
				"poller_index", p.index)
			return
		default:
			// Poll for a task
			task, err := p.pollForTask(ctx, policy)
			if err != nil {
				// Context canceled, exit gracefully
				if ctx.Err() != nil {
					return
				}
				// Poll failed, but will be retried by pollForTask
				continue
			}

			// If we got a task, execute it
			if task != nil {
				logger.Info(ctx, "Task received, starting execution",
					"worker_id", p.workerID,
					"poller_index", p.index,
					"dag_run_id", task.DagRunId)

				// Execute the task using the TaskExecutor
				err := p.taskExecutor.Execute(ctx, task)
				if err != nil {
					if ctx.Err() != nil {
						// Context cancelled, exit gracefully
						return
					}
					logger.Error(ctx, "Task execution failed",
						"worker_id", p.workerID,
						"poller_index", p.index,
						"dag_run_id", task.DagRunId,
						"error", err)
				} else {
					logger.Info(ctx, "Task execution completed successfully",
						"worker_id", p.workerID,
						"poller_index", p.index,
						"dag_run_id", task.DagRunId)
				}
			}
			// Continue polling for the next task
		}
	}
}

// pollForTask polls the coordinator for a task with retry on failure
func (p *Poller) pollForTask(ctx context.Context, policy backoff.RetryPolicy) (*coordinatorv1.Task, error) {
	pollerID := uuid.New().String()

	var task *coordinatorv1.Task
	err := backoff.Retry(ctx, func(ctx context.Context) error {
		req := &coordinatorv1.PollRequest{
			WorkerId: p.workerID,
			PollerId: pollerID,
			Labels:   p.labels,
		}

		// Perform the poll (this is a long-polling call)
		resp, err := p.client.Poll(ctx, req)
		if err != nil {
			// Update state and log based on state transition
			p.state.mu.Lock()
			wasConnected := p.state.isConnected
			p.state.isConnected = false
			p.state.consecutiveFails++
			p.state.lastError = err
			failCount := p.state.consecutiveFails
			p.state.mu.Unlock()

			if wasConnected {
				// First failure after being connected - log as ERROR
				logger.Error(ctx, "Poll failed - lost connection to coordinator",
					"error", err,
					"worker_id", p.workerID,
					"poller_id", pollerID,
					"poller_index", p.index,
					"coordinator", p.coordinatorAddr)
			} else {
				// Subsequent failures - log as DEBUG
				logger.Debug(ctx, "Poll still failing",
					"error", err,
					"worker_id", p.workerID,
					"poller_id", pollerID,
					"poller_index", p.index,
					"consecutive_failures", failCount)
			}
			return err // Will be retried with backoff
		}

		// Success - update state
		p.state.mu.Lock()
		wasDisconnected := !p.state.isConnected
		previousFailCount := p.state.consecutiveFails
		p.state.isConnected = true
		p.state.consecutiveFails = 0
		p.state.lastError = nil
		p.state.mu.Unlock()

		if wasDisconnected && previousFailCount > 0 {
			// Recovered from disconnection - log as INFO
			logger.Info(ctx, "Poll succeeded - reconnected to coordinator",
				"worker_id", p.workerID,
				"poller_id", pollerID,
				"poller_index", p.index,
				"coordinator", p.coordinatorAddr,
				"previous_consecutive_failures", previousFailCount)
		}

		// Handle the received task
		if resp.Task != nil {
			logger.Info(ctx, "Task received",
				"worker_id", p.workerID,
				"poller_id", pollerID,
				"poller_index", p.index,
				"root_dag_run_name", resp.Task.RootDagRunName,
				"root_dag_run_id", resp.Task.RootDagRunId,
				"parent_dag_run_name", resp.Task.ParentDagRunName,
				"parent_dag_run_id", resp.Task.ParentDagRunId,
				"dag_run_id", resp.Task.DagRunId)
			task = resp.Task
		}

		return nil
	}, policy, nil)

	return task, err
}

// GetState returns the current connection state (for monitoring/testing)
func (p *Poller) GetState() (isConnected bool, consecutiveFails int, lastError error) {
	p.state.mu.Lock()
	defer p.state.mu.Unlock()
	return p.state.isConnected, p.state.consecutiveFails, p.state.lastError
}
