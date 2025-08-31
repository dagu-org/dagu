package scheduler

import (
	"context"
	"errors"
	"fmt"

	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// DAGExecutor handles both local and distributed DAG execution.
// It encapsulates the logic for deciding between local and distributed execution
// and dispatching DAGs accordingly.
//
// Architecture Overview:
//
// The DAGExecutor implements a persistence-first approach for distributed execution to ensure
// reliability and eventual execution even when the coordinator or workers are temporarily unavailable.
//
// Execution Flow:
//
// 1. Scheduled Jobs (from DAGRunJob.Start):
//   - Operation: OPERATION_START
//   - Flow: DAGRunJob.Start() → HandleJob() → EnqueueDAGRun() (for distributed)
//   - This creates a persisted record with status=QUEUED before any dispatch attempt
//   - Ensures the job is tracked and can be retried if coordinator/workers are down
//
// 2. Queue Processing (from Scheduler queue handler):
//   - Operation: OPERATION_RETRY (meaning "retry the dispatch", not "retry failed execution")
//   - Flow: Queue Handler → ExecuteDAG() → Dispatch to Coordinator
//   - The item has already been persisted (was enqueued in step 1)
//   - Directly dispatches to coordinator without enqueueing again
//
// This two-phase approach guarantees:
// - No lost jobs: All scheduled runs are persisted before dispatch
// - Automatic retry: If dispatch fails, the queue handler will retry
// - Idempotency: Queue items are never enqueued twice
// - Resilience: System continues to work even if coordinator is temporarily down
//
// Method Responsibilities:
// - HandleJob(): Entry point for new scheduled jobs (handles persistence)
// - ExecuteDAG(): Executes/dispatches already-persisted jobs (no persistence)
type DAGExecutor struct {
	coordinatorCli digraph.Dispatcher
	dagRunManager  dagrun.Manager
}

// NewDAGExecutor creates a new DAGExecutor instance.
func NewDAGExecutor(
	coordinatorCli digraph.Dispatcher,
	dagRunManager dagrun.Manager,
) *DAGExecutor {
	return &DAGExecutor{
		coordinatorCli: coordinatorCli,
		dagRunManager:  dagRunManager,
	}
}

// HandleJob is the entry point for new scheduled jobs (from DAGRunJob.Start).
// For distributed execution, it enqueues the DAG run to ensure persistence before dispatch.
// For local execution, it delegates to ExecuteDAG.
//
// This method implements the persistence-first approach:
// 1. Distributed: Enqueue → Queue Handler picks up → ExecuteDAG dispatches
// 2. Local: Direct execution via ExecuteDAG
//
// The enqueueing step ensures that:
// - The job is persisted with status=QUEUED before any execution attempt
// - The job can be retried if the coordinator or workers are unavailable
// - No jobs are lost due to temporary system failures
func (e *DAGExecutor) HandleJob(
	ctx context.Context,
	dag *digraph.DAG,
	operation coordinatorv1.Operation,
	runID string,
) error {
	// For distributed execution with START operation, enqueue for persistence
	if e.shouldUseDistributedExecution(dag) && operation == coordinatorv1.Operation_OPERATION_START {
		logger.Info(ctx, "Enqueueing DAG for distributed execution",
			"dag", dag.Name,
			"runId", runID,
			"workerSelector", dag.WorkerSelector)

		if err := e.dagRunManager.EnqueueDAGRun(ctx, dag, dagrun.EnqueueOptions{
			DAGRunID: runID,
		}); err != nil {
			return fmt.Errorf("failed to enqueue DAG run: %w", err)
		}
		return nil
	}

	// For all other cases (local execution or non-START operations), use ExecuteDAG
	return e.ExecuteDAG(ctx, dag, operation, runID)
}

// ExecuteDAG executes or dispatches an already-persisted DAG.
// This method is used by the queue handler for processing queued items.
// It NEVER enqueues - that's the responsibility of HandleJob.
//
// For distributed execution: Creates a task and dispatches to coordinator
// For local execution: Runs the DAG using the appropriate manager method
//
// Note: When called from the queue handler, operation is always OPERATION_RETRY,
// which means "retry the dispatch", not "retry a failed execution".
func (e *DAGExecutor) ExecuteDAG(
	ctx context.Context,
	dag *digraph.DAG,
	operation coordinatorv1.Operation,
	runID string,
) error {
	if e.shouldUseDistributedExecution(dag) {
		// Distributed execution: dispatch to coordinator
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
		return e.dagRunManager.RetryDAGRun(ctx, dag, runID, true)
	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return errors.New("operation not specified")
	default:
		return fmt.Errorf("unsupported operation: %v", operation)
	}
}

// shouldUseDistributedExecution checks if distributed execution should be used.
// Returns true only if:
// 1. A coordinator client factory is configured (coordinator is available)
// 2. The DAG has workerSelector labels (DAG explicitly requests distributed execution)
//
// This ensures backward compatibility - DAGs without workerSelector continue
// to run locally even when a coordinator is configured.
func (e *DAGExecutor) shouldUseDistributedExecution(dag *digraph.DAG) bool {
	return e.coordinatorCli != nil && dag != nil && len(dag.WorkerSelector) > 0
}

// dispatchToCoordinator dispatches a task to the coordinator for distributed execution.
// This is called after the job has been persisted (for START operations via HandleJob)
// or when retrying dispatch (for RETRY operations from queue handler).
//
// The coordinator will:
// 1. Select an appropriate worker based on the task's workerSelector
// 2. Forward the task to the selected worker
// 3. Track the execution status
func (e *DAGExecutor) dispatchToCoordinator(ctx context.Context, task *coordinatorv1.Task) error {
	if err := e.coordinatorCli.Dispatch(ctx, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	logger.Info(ctx, "Task dispatched to coordinator",
		"target", task.Target,
		"runID", task.DagRunId,
		"operation", task.Operation.String())

	return nil
}

// Close closes any resources held by the DAGExecutor, including the coordinator client
func (e *DAGExecutor) Close(ctx context.Context) {
	if e.coordinatorCli != nil {
		if err := e.coordinatorCli.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup coordinator client", "err", err)
		}
		e.coordinatorCli = nil
	}
}
