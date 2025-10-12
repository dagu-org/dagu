package worker

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/services/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/google/uuid"
)

// Poller handles polling for tasks from the coordinator
type Poller struct {
	workerID       string
	coordinatorCli coordinator.Client
	handler        TaskHandler
	index          int
	labels         map[string]string
}

// NewPoller creates a new poller instance
func NewPoller(workerID string, coordinatorCli coordinator.Client, handler TaskHandler, index int, labels map[string]string) *Poller {
	return &Poller{
		workerID:       workerID,
		coordinatorCli: coordinatorCli,
		handler:        handler,
		index:          index,
		labels:         labels,
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

				// Execute the task using the TaskHandler
				err := p.handler.Handle(ctx, task)
				if err != nil {
					if ctx.Err() != nil {
						// Context cancelled, exit gracefully
						return
					}
					logger.Error(ctx, "Task execution failed",
						"worker_id", p.workerID,
						"poller_index", p.index,
						"dag_run_id", task.DagRunId,
						"err", err)
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

	// Get current coordinator client state before polling
	beforeMetrics := p.coordinatorCli.Metrics()

	req := &coordinatorv1.PollRequest{
		WorkerId: p.workerID,
		PollerId: pollerID,
		Labels:   p.labels,
	}

	// Use coordinator client's Poll method which handles retries and failover
	task, err := p.coordinatorCli.Poll(ctx, policy, req)
	if err != nil {
		// Get updated metrics after failure
		afterMetrics := p.coordinatorCli.Metrics()

		// Check if this was first failure after being connected
		if beforeMetrics.IsConnected && !afterMetrics.IsConnected {
			// First failure after being connected - log as ERROR
			logger.Error(ctx, "Poll failed - lost connection to coordinator",
				"err", err,
				"worker_id", p.workerID,
				"poller_id", pollerID,
				"poller_index", p.index)
		} else {
			// Subsequent failures - log as DEBUG
			logger.Debug(ctx, "Poll still failing",
				"err", err,
				"worker_id", p.workerID,
				"poller_id", pollerID,
				"poller_index", p.index,
				"consecutive_failures", afterMetrics.ConsecutiveFails)
		}
		return nil, err
	}

	// Success - check if we recovered from disconnection
	afterMetrics := p.coordinatorCli.Metrics()
	if !beforeMetrics.IsConnected && afterMetrics.IsConnected && beforeMetrics.ConsecutiveFails > 0 {
		// Recovered from disconnection - log as INFO
		logger.Info(ctx, "Poll succeeded - reconnected to coordinator",
			"worker_id", p.workerID,
			"poller_id", pollerID,
			"poller_index", p.index,
			"previous_consecutive_failures", beforeMetrics.ConsecutiveFails)
	}

	// Handle the received task
	if task != nil {
		logger.Info(ctx, "Task received",
			"worker_id", p.workerID,
			"poller_id", pollerID,
			"poller_index", p.index,
			"root_dag_run_name", task.RootDagRunName,
			"root_dag_run_id", task.RootDagRunId,
			"parent_dag_run_name", task.ParentDagRunName,
			"parent_dag_run_id", task.ParentDagRunId,
			"dag_run_id", task.DagRunId)
	}

	return task, nil
}

// GetState returns the current connection state (for monitoring/testing)
func (p *Poller) GetState() (isConnected bool, consecutiveFails int, lastError error) {
	metrics := p.coordinatorCli.Metrics()
	return metrics.IsConnected, metrics.ConsecutiveFails, metrics.LastError
}
