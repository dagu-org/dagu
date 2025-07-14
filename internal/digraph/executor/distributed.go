package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/dagu-org/dagu/internal/coordinator/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
)

// DistributedExecutor provides common functionality for distributed DAG execution
type DistributedExecutor struct {
	coordinatorClientFactory *client.Factory
}

// NewDistributedExecutor creates a new distributed executor
func NewDistributedExecutor(ctx context.Context) *DistributedExecutor {
	env := GetEnv(ctx)
	return &DistributedExecutor{
		coordinatorClientFactory: env.CoordinatorClientFactory,
	}
}

// DispatchToCoordinator builds and dispatches a task to the coordinator
func (d *DistributedExecutor) DispatchToCoordinator(ctx context.Context, child *ChildDAGExecutor, runParams RunParams) error {
	// Build the coordinator task
	task, err := child.BuildCoordinatorTask(ctx, runParams)
	if err != nil {
		return fmt.Errorf("failed to build coordinator task: %w", err)
	}

	// Create coordinator client
	coordinatorClient, err := d.getCoordinatorClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create coordinator client: %w", err)
	}
	defer func() {
		if err := coordinatorClient.Close(); err != nil {
			logger.Error(ctx, "Failed to close coordinator client", "err", err)
		}
	}()

	// Dispatch the task
	logger.Info(ctx, "Dispatching task to coordinator",
		"dag_run_id", task.DagRunId,
		"target", task.Target,
		"worker_selector", task.WorkerSelector,
	)

	if err := coordinatorClient.Dispatch(ctx, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	return nil
}

// WaitForCompletion polls for the completion of a distributed task
func (d *DistributedExecutor) WaitForCompletion(ctx context.Context, dagRunID string, stdout io.Writer) error {
	env := GetEnv(ctx)

	// Poll for completion
	pollInterval := 1 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("distributed execution cancelled: %w", ctx.Err())
		case <-ticker.C:
			// Check if the child DAG run has completed
			isCompleted, err := env.DB.IsChildDAGRunCompleted(ctx, dagRunID, env.RootDAGRun)
			if err != nil {
				logger.Error(ctx, "Failed to check child DAG run completion",
					"dag_run_id", dagRunID,
					"err", err,
				)
				continue // Retry on error
			}

			if !isCompleted {
				logger.Debug(ctx, "Child DAG run not completed yet",
					"dag_run_id", dagRunID,
				)
				continue // Not completed, keep polling
			}

			// Check the final status of the child DAG run
			result, err := env.DB.GetChildDAGRunStatus(ctx, dagRunID, env.RootDAGRun)
			if err != nil {
				// Not found yet, continue polling
				logger.Debug(ctx, "Child DAG run status not available yet",
					"dag_run_id", dagRunID,
					"err", err,
				)
				continue
			}

			// If we got a result, the child DAG has completed
			logger.Info(ctx, "Distributed execution completed",
				"dag_run_id", dagRunID,
				"name", result.Name,
				"is_success", result.Success,
			)

			// Write the results to stdout if available
			if stdout != nil {
				jsonData, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal outputs: %w", err)
				}
				jsonData = append(jsonData, '\n')

				if _, err := stdout.Write(jsonData); err != nil {
					return fmt.Errorf("failed to write outputs: %w", err)
				}
			}

			// Check if the execution was successful
			if !result.Success {
				return fmt.Errorf("child DAG execution failed")
			}

			return nil
		}
	}
}

// WaitForCompletionWithResult is similar to WaitForCompletion but returns the result
func (d *DistributedExecutor) WaitForCompletionWithResult(ctx context.Context, dagRunID string) (*ChildResult, error) {
	env := GetEnv(ctx)

	// Poll for completion
	pollInterval := 1 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("distributed execution cancelled: %w", ctx.Err())
		case <-ticker.C:
			// Check if the child DAG run has completed
			isCompleted, err := env.DB.IsChildDAGRunCompleted(ctx, dagRunID, env.RootDAGRun)
			if err != nil {
				logger.Error(ctx, "Failed to check child DAG run completion",
					"dag_run_id", dagRunID,
					"err", err,
				)
				continue // Retry on error
			}

			if !isCompleted {
				logger.Debug(ctx, "Child DAG run not completed yet",
					"dag_run_id", dagRunID,
				)
				continue // Not completed, keep polling
			}

			// Check the final status of the child DAG run
			result, err := env.DB.GetChildDAGRunStatus(ctx, dagRunID, env.RootDAGRun)
			if err != nil {
				// Not found yet, continue polling
				logger.Debug(ctx, "Child DAG run status not available yet",
					"dag_run_id", dagRunID,
					"err", err,
				)
				continue
			}

			// If we got a result, the child DAG has completed
			logger.Info(ctx, "Distributed execution completed",
				"dag_run_id", dagRunID,
				"name", result.Name,
				"is_success", result.Success,
			)

			// Convert to ChildResult
			childResult := &ChildResult{
				RunID:    dagRunID,
				Params:   result.Params,
				Status:   getStatusString(result.Success),
				Output:   convertOutputsToMap(result.Outputs),
				Error:    getErrorString(result),
				ExitCode: getExitCode(result.Success),
			}

			return childResult, nil
		}
	}
}

// getCoordinatorClient gets a coordinator client using the factory from environment
func (d *DistributedExecutor) getCoordinatorClient(ctx context.Context) (client.Client, error) {
	// Factory should be initialized when Env is created
	if d.coordinatorClientFactory == nil {
		return nil, fmt.Errorf("coordinator client factory not initialized in environment")
	}

	// Build client from factory
	return d.coordinatorClientFactory.Build(ctx)
}

// Helper functions to convert result status
func getStatusString(success bool) string {
	if success {
		return "succeeded"
	}
	return "failed"
}

func getErrorString(result *digraph.Status) string {
	if !result.Success {
		return "child DAG execution failed"
	}
	return ""
}

func getExitCode(success bool) int {
	if success {
		return 0
	}
	return 1
}

// convertOutputsToMap converts string map to map[string]any
func convertOutputsToMap(outputs map[string]string) map[string]any {
	if outputs == nil {
		return nil
	}
	result := make(map[string]any)
	for k, v := range outputs {
		result[k] = v
	}
	return result
}

