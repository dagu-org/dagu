package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/runtime"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// TaskHandler defines the interface for executing tasks
type TaskHandler interface {
	Handle(ctx context.Context, task *coordinatorv1.Task) error
}

var _ TaskHandler = (*taskHandler)(nil)

// NewTaskHandler creates a new TaskHandler
func NewTaskHandler(cfg *config.Config) TaskHandler {
	return &taskHandler{
		subCmdBuilder: runtime.NewSubCmdBuilder(cfg),
	}
}

type taskHandler struct{ subCmdBuilder *runtime.SubCmdBuilder }

// Handle runs the task using the dagrun.Manager.
func (e *taskHandler) Handle(ctx context.Context, task *coordinatorv1.Task) error {
	logger.Info(ctx, "Executing task",
		slog.String("operation", task.Operation.String()),
		tag.Target(task.Target),
		tag.RunID(task.DagRunId),
		slog.String("root-dag-run-id", task.RootDagRunId),
		slog.String("parent-dag-run-id", task.ParentDagRunId),
		slog.String("worker-id", task.WorkerId))

	logger.Info(ctx, "Creating temporary DAG file from definition",
		tag.DAG(task.Target),
		tag.Size(len(task.Definition)))

	tempFile, err := fileutil.CreateTempDAGFile("worker-dags", task.Target, []byte(task.Definition))
	if err != nil {
		return fmt.Errorf("failed to create temp DAG file: %w", err)
	}
	defer func() {
		if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
			logger.Errorf(ctx, "Failed to remove temp DAG file: %v", err)
		}
	}()

	task.Target = tempFile

	logger.Info(ctx, "Created temporary DAG file",
		tag.File(tempFile))

	spec, err := e.buildCommandSpec(task)
	if err != nil {
		return err
	}

	if err := runtime.Run(ctx, spec); err != nil {
		logger.Error(ctx, "Distributed task execution failed",
			slog.String("operation", task.Operation.String()),
			tag.Target(task.Target),
			tag.RunID(task.DagRunId),
			tag.Error(err))
		return err
	}

	logger.Info(ctx, "Distributed task execution finished",
		slog.String("operation", task.Operation.String()),
		tag.Target(task.Target),
		tag.RunID(task.DagRunId))

	return nil
}

func (e *taskHandler) buildCommandSpec(task *coordinatorv1.Task) (runtime.CmdSpec, error) {
	switch task.Operation {
	case coordinatorv1.Operation_OPERATION_START:
		return e.subCmdBuilder.TaskStart(task), nil

	case coordinatorv1.Operation_OPERATION_RETRY:
		return e.subCmdBuilder.TaskRetry(task), nil

	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return runtime.CmdSpec{}, fmt.Errorf("operation not specified")

	default:
		return runtime.CmdSpec{}, fmt.Errorf("unknown operation: %v", task.Operation)
	}
}
