package scheduler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/executor"
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
	coordinatorCli       exec.Dispatcher
	subCmdBuilder        *runtime.SubCmdBuilder
	defaultExecutionMode config.ExecutionMode
}

// NewDAGExecutor creates a new DAGExecutor instance.
func NewDAGExecutor(
	coordinatorCli exec.Dispatcher,
	subCmdBuilder *runtime.SubCmdBuilder,
	defaultExecutionMode config.ExecutionMode,
) *DAGExecutor {
	return &DAGExecutor{
		coordinatorCli:       coordinatorCli,
		subCmdBuilder:        subCmdBuilder,
		defaultExecutionMode: defaultExecutionMode,
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
	dag *core.DAG,
	operation coordinatorv1.Operation,
	runID string,
	triggerType core.TriggerType,
) error {
	// For distributed execution with START operation, enqueue for persistence
	if e.shouldUseDistributedExecution(dag) && operation == coordinatorv1.Operation_OPERATION_START {
		ctx = logger.WithValues(ctx,
			tag.DAG(dag.Name),
			tag.RunID(runID),
		)

		logger.Info(ctx, "Enqueueing DAG for distributed execution",
			slog.Any("worker-selector", dag.WorkerSelector),
		)

		spec := e.subCmdBuilder.Enqueue(dag, runtime.EnqueueOptions{
			DAGRunID:    runID,
			TriggerType: triggerType.String(),
		})
		if err := runtime.Run(ctx, spec); err != nil {
			return fmt.Errorf("failed to enqueue DAG run: %w", err)
		}
		return nil
	}

	// For all other cases (local execution or non-START operations), use ExecuteDAG
	return e.ExecuteDAG(ctx, dag, operation, runID, nil, triggerType)
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
	dag *core.DAG,
	operation coordinatorv1.Operation,
	runID string,
	previousStatus *exec.DAGRunStatus,
	triggerType core.TriggerType,
) error {
	if e.shouldUseDistributedExecution(dag) {
		// Distributed execution: dispatch to coordinator
		task := executor.CreateTask(
			dag.Name,
			string(dag.YamlData),
			operation,
			runID,
			executor.WithWorkerSelector(dag.WorkerSelector),
			executor.WithPreviousStatus(previousStatus),
		)
		return e.dispatchToCoordinator(ctx, task)
	}

	// Local execution
	switch operation {
	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return fmt.Errorf("operation not specified")

	case coordinatorv1.Operation_OPERATION_START:
		spec := e.subCmdBuilder.Start(dag, runtime.StartOptions{
			DAGRunID:    runID,
			Quiet:       true,
			TriggerType: triggerType.String(),
		})
		return runtime.Start(ctx, spec)

	case coordinatorv1.Operation_OPERATION_RETRY:
		spec := e.subCmdBuilder.Retry(dag, runID, "")
		return runtime.Run(ctx, spec)

	default:
		return fmt.Errorf("unsupported operation: %v", operation)
	}
}

// shouldUseDistributedExecution checks if distributed execution should be used.
// Delegates to core.ShouldDispatchToCoordinator for consistent dispatch logic
// across all execution paths (API, CLI, scheduler, sub-DAG).
func (e *DAGExecutor) shouldUseDistributedExecution(dag *core.DAG) bool {
	return core.ShouldDispatchToCoordinator(dag, e.coordinatorCli != nil, e.defaultExecutionMode)
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
	ctx = logger.WithValues(ctx,
		tag.Target(task.Target),
		tag.RunID(task.DagRunId),
	)

	if err := e.coordinatorCli.Dispatch(ctx, task); err != nil {
		logger.Error(ctx, "Failed to dispatch task to coordinator",
			tag.Error(err),
			slog.String("operation", task.Operation.String()),
		)
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	logger.Info(ctx, "Task dispatched to coordinator",
		slog.String("operation", task.Operation.String()),
	)

	return nil
}

// Restart restarts a DAG unconditionally.
func (e *DAGExecutor) Restart(ctx context.Context, dag *core.DAG) error {
	spec := e.subCmdBuilder.Restart(dag, runtime.RestartOptions{
		Quiet: true,
	})
	return runtime.Start(ctx, spec)
}

// Close closes any resources held by the DAGExecutor, including the coordinator client
func (e *DAGExecutor) Close(ctx context.Context) {
	if e.coordinatorCli != nil {
		if err := e.coordinatorCli.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup coordinator client", tag.Error(err))
		}
		e.coordinatorCli = nil
	}
}
