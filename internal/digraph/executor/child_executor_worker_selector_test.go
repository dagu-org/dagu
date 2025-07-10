package executor_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChildDAGExecutorWorkerSelector(t *testing.T) {
	t.Run("ShouldUseDistributedExecution", func(t *testing.T) {
		// Create a mock executor to test the logic
		exec := &executor.ChildDAGExecutor{
			DAG: &digraph.DAG{Name: "test-dag"},
		}

		// Test without worker selector
		assert.False(t, exec.ShouldUseDistributedExecution())

		// Set worker selector
		exec.SetWorkerSelector(map[string]string{
			"gpu":    "true",
			"memory": "64G",
		})

		// Test with worker selector
		assert.True(t, exec.ShouldUseDistributedExecution())
	})

	t.Run("BuildCoordinatorTask", func(t *testing.T) {
		// Setup test environment
		dagPath := test.TestdataPath(t, "digraph/loader_test.yaml")
		dag, err := digraph.Load(context.Background(), dagPath)
		require.NoError(t, err)

		// Create execution context
		ctx := executor.WithEnv(context.Background(), executor.Env{
			Env: digraph.Env{
				DAGRunID:   "test-run-123",
				RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-456"),
				DAG:        dag,
			},
		})

		// Create child executor
		exec := &executor.ChildDAGExecutor{
			DAG: &digraph.DAG{
				Name:     "child-dag",
				Location: "/path/to/child.yaml",
			},
		}

		// Set worker selector
		workerSelector := map[string]string{
			"gpu":    "true",
			"memory": "64G",
		}
		exec.SetWorkerSelector(workerSelector)

		// Build coordinator task
		runParams := executor.RunParams{
			RunID:  "child-run-789",
			Params: "KEY=value",
		}

		task, err := exec.BuildCoordinatorTask(ctx, runParams)
		require.NoError(t, err)

		// Verify task fields
		assert.Equal(t, "OPERATION_START", task.Operation.String())
		assert.Equal(t, "root-dag", task.RootDagRunName)
		assert.Equal(t, "root-run-456", task.RootDagRunId)
		assert.Equal(t, dag.Name, task.ParentDagRunName)
		assert.Equal(t, "test-run-123", task.ParentDagRunId)
		assert.Equal(t, "child-run-789", task.DagRunId)
		assert.Equal(t, "/path/to/child.yaml", task.Target)
		assert.Equal(t, "KEY=value", task.Params)
		assert.Equal(t, workerSelector, task.WorkerSelector)
	})
}
