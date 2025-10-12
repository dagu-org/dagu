package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/logger"
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
	logger.Info(ctx, "Executing task",
		"operation", task.Operation.String(),
		"target", task.Target,
		"dag_run_id", task.DagRunId,
		"root_dag_run_id", task.RootDagRunId,
		"parent_dag_run_id", task.ParentDagRunId)

	var tempFile string

	// If definition is provided, create a temporary DAG file
	if task.Definition != "" {
		logger.Info(ctx, "Creating temporary DAG file from definition",
			"dagName", task.Target,
			"definitionSize", len(task.Definition))

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

		logger.Info(ctx, "Created temporary DAG file",
			"tempFile", tempFile,
			"originalTarget", originalTarget)
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

	return runtime.Run(ctx, spec) // Synchronous execution
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
