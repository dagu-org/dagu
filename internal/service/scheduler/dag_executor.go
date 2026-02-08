package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
//
// It uses a persistence-first approach for distributed execution:
//   - HandleJob: Entry point for new scheduled jobs. For distributed START operations,
//     enqueues to persist the run before dispatch. For local execution, delegates to ExecuteDAG.
//   - ExecuteDAG: Executes or dispatches an already-persisted job. Used by the queue handler
//     for processing queued items. Never enqueues.
type DAGExecutor struct {
	coordinatorCli  exec.Dispatcher
	subCmdBuilder   *runtime.SubCmdBuilder
	defaultExecMode config.ExecutionMode
}

// NewDAGExecutor creates a new DAGExecutor instance.
func NewDAGExecutor(
	coordinatorCli exec.Dispatcher,
	subCmdBuilder *runtime.SubCmdBuilder,
	defaultExecMode config.ExecutionMode,
) *DAGExecutor {
	return &DAGExecutor{
		coordinatorCli:  coordinatorCli,
		subCmdBuilder:   subCmdBuilder,
		defaultExecMode: defaultExecMode,
	}
}

// HandleJob is the entry point for new scheduled jobs.
// For distributed START operations, it enqueues for persistence before dispatch.
// For all other cases, it delegates to ExecuteDAG.
func (e *DAGExecutor) HandleJob(
	ctx context.Context,
	dag *core.DAG,
	operation coordinatorv1.Operation,
	runID string,
	triggerType core.TriggerType,
	scheduledTime time.Time,
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
			DAGRunID:      runID,
			TriggerType:   triggerType.String(),
			ScheduledTime: formatScheduledTime(scheduledTime),
		})
		if err := runtime.Run(ctx, spec); err != nil {
			return fmt.Errorf("failed to enqueue DAG run: %w", err)
		}
		return nil
	}

	// For all other cases (local execution or non-START operations), use ExecuteDAG
	return e.ExecuteDAG(ctx, dag, operation, runID, nil, triggerType, scheduledTime)
}

// ExecuteDAG executes or dispatches an already-persisted DAG.
// For distributed execution, it creates a task and dispatches to the coordinator.
// For local execution, it runs the DAG directly.
func (e *DAGExecutor) ExecuteDAG(
	ctx context.Context,
	dag *core.DAG,
	operation coordinatorv1.Operation,
	runID string,
	previousStatus *exec.DAGRunStatus,
	triggerType core.TriggerType,
	scheduledTime time.Time,
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
			DAGRunID:      runID,
			Quiet:         true,
			TriggerType:   triggerType.String(),
			ScheduledTime: formatScheduledTime(scheduledTime),
		})
		return runtime.Start(ctx, spec)

	case coordinatorv1.Operation_OPERATION_RETRY:
		spec := e.subCmdBuilder.Retry(dag, runID, "")
		return runtime.Run(ctx, spec)

	default:
		return fmt.Errorf("unsupported operation: %v", operation)
	}
}

// formatScheduledTime returns the RFC3339 representation of a scheduled time,
// or an empty string if the time is zero.
func formatScheduledTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// shouldUseDistributedExecution checks if distributed execution should be used.
// Delegates to core.ShouldDispatchToCoordinator for consistent dispatch logic
// across all execution paths (API, CLI, scheduler, sub-DAG).
func (e *DAGExecutor) shouldUseDistributedExecution(dag *core.DAG) bool {
	return core.ShouldDispatchToCoordinator(dag, e.coordinatorCli != nil, e.defaultExecMode)
}

// dispatchToCoordinator dispatches a task to the coordinator for distributed execution.
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
