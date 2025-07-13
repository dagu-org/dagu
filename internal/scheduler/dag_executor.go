package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"

	coordinatorclient "github.com/dagu-org/dagu/internal/coordinator/client"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// DAGExecutor handles both local and distributed DAG execution.
// It encapsulates the logic for deciding between local and distributed execution
// and dispatching DAGs accordingly.
type DAGExecutor struct {
	coordinatorClientFactory *coordinatorclient.Factory
	coordinatorClient        coordinatorclient.Client
	coordinatorClientMu      sync.Mutex
	dagRunManager            dagrun.Manager
}

// NewDAGExecutor creates a new DAGExecutor instance.
func NewDAGExecutor(
	coordinatorClientFactory *coordinatorclient.Factory,
	dagRunManager dagrun.Manager,
) *DAGExecutor {
	return &DAGExecutor{
		coordinatorClientFactory: coordinatorClientFactory,
		dagRunManager:            dagRunManager,
	}
}

// ExecuteDAG handles the execution of a DAG, choosing between local and distributed execution.
// For distributed execution, it creates a task and dispatches it to the coordinator.
// For local execution, it uses the appropriate DAG run manager method based on the operation.
func (e *DAGExecutor) ExecuteDAG(
	ctx context.Context,
	dag *digraph.DAG,
	operation coordinatorv1.Operation,
	runID string,
) error {
	if e.shouldUseDistributedExecution(dag) {
		// Distributed execution
		task := dag.CreateTask(
			operation,
			runID,
			digraph.WithWorkerSelector(dag.WorkerSelector),
		)
		return e.dispatchToCoordinator(ctx, task)
	}

	// Local execution
	switch operation {
	case coordinatorv1.Operation_OPERATION_START:
		return e.dagRunManager.StartDAGRunAsync(ctx, dag, dagrun.StartOptions{
			Quiet:    true,
			DAGRunID: runID,
		})
	case coordinatorv1.Operation_OPERATION_RETRY:
		return e.dagRunManager.RetryDAGRun(ctx, dag, runID)
	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return errors.New("operation not specified")
	default:
		return fmt.Errorf("unsupported operation: %v", operation)
	}
}

// shouldUseDistributedExecution checks if distributed execution should be used.
// Returns true only if coordinator is configured AND the DAG has workerSelector labels.
func (e *DAGExecutor) shouldUseDistributedExecution(dag *digraph.DAG) bool {
	return e.coordinatorClientFactory != nil && dag != nil && len(dag.WorkerSelector) > 0
}

// getCoordinatorClient returns the coordinator client, creating it lazily if needed.
// Returns nil if no coordinator is configured.
func (e *DAGExecutor) getCoordinatorClient(ctx context.Context) (coordinatorclient.Client, error) {
	// If no factory configured, distributed execution is disabled
	if e.coordinatorClientFactory == nil {
		return nil, nil
	}

	// Check if client already exists
	e.coordinatorClientMu.Lock()
	defer e.coordinatorClientMu.Unlock()

	if e.coordinatorClient != nil {
		return e.coordinatorClient, nil
	}

	// Create client
	client, err := e.coordinatorClientFactory.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create coordinator client: %w", err)
	}

	e.coordinatorClient = client
	logger.Info(ctx, "Coordinator client initialized for distributed execution")
	return client, nil
}

// dispatchToCoordinator dispatches a task to the coordinator for distributed execution.
func (e *DAGExecutor) dispatchToCoordinator(ctx context.Context, task *coordinatorv1.Task) error {
	client, err := e.getCoordinatorClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get coordinator client: %w", err)
	}

	if client == nil {
		// Should not happen if shouldUseDistributedExecution is checked
		return errors.New("coordinator client not available")
	}

	err = client.Dispatch(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	logger.Info(ctx, "Task dispatched to coordinator",
		"target", task.Target,
		"runID", task.DagRunId,
		"operation", task.Operation.String())

	return nil
}
