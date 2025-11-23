package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
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

// Handle runs the task using the dagrun.Manager
func (e *taskHandler) Handle(ctx context.Context, task *coordinatorv1.Task) error {
	logger.Info(ctx, "Executing task", "operation", task.Operation.String(), tag.Target, task.Target, tag.RunID, task.DagRunId, "root-dag-run-id", task.RootDagRunId, "parent-dag-run-id", task.ParentDagRunId)

	var tempFile string

	// If definition is provided, create a temporary DAG file
	if task.Definition != "" {
		logger.Info(ctx, "Creating temporary DAG file from definition", tag.DAG, task.Target, tag.Size, len(task.Definition))

		tf, err := createTempDAGFile(task.Target, []byte(task.Definition))
		if err != nil {
			return fmt.Errorf("failed to create temp DAG file: %w", err)
		}
		tempFile = tf
		defer func() {
			// Clean up the temporary file
			if err := os.Remove(tempFile); err != nil && !os.IsNotExist(err) {
				logger.Errorf(ctx, "Failed to remove temp DAG file: %v", err)
			}
		}()
		// Update the target to use the temp file
		originalTarget := task.Target
		task.Target = tempFile

		logger.Info(ctx, "Created temporary DAG file", tag.File, tempFile, "original-target", originalTarget)
	}

	// Build command spec based on operation
	var spec runtime.CmdSpec

	switch task.Operation {
	case coordinatorv1.Operation_OPERATION_START:
		spec = e.subCmdBuilder.TaskStart(task)
	case coordinatorv1.Operation_OPERATION_RETRY:
		spec = e.subCmdBuilder.TaskRetry(task)
	case coordinatorv1.Operation_OPERATION_UNSPECIFIED:
		return fmt.Errorf("operation not specified")
	default:
		return fmt.Errorf("unknown operation: %v", task.Operation)
	}

	if err := runtime.Run(ctx, spec); err != nil {
		logger.Error(ctx, "Distributed task execution failed", "operation", task.Operation.String(), tag.Target, task.Target, tag.RunID, task.DagRunId, tag.Error, err)
		return err
	}

	logger.Info(ctx, "Distributed task execution finished", "operation", task.Operation.String(), tag.Target, task.Target, tag.RunID, task.DagRunId)

	return nil
}

func createTempDAGFile(dagName string, yamlData []byte) (string, error) {
	// Create a temporary directory if it doesn't exist
	tempDir := filepath.Join(os.TempDir(), "dagu", "worker-dags")
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create a temporary file with a meaningful name
	pattern := fmt.Sprintf("%s-*.yaml", dagName)
	tempFile, err := os.CreateTemp(tempDir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
	}()

	// Write the YAML data
	if _, err := tempFile.Write(yamlData); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write YAML data: %w", err)
	}

	return tempFile.Name(), nil
}
