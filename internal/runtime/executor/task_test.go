package executor_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
)

func TestDAG_CreateTask(t *testing.T) {
	t.Parallel()

	t.Run("BasicTaskCreation", func(t *testing.T) {
		t.Parallel()

		dagName := "test-dag"
		yamlDefinition := `
name: test-dag
steps:
  - name: step1
	command: echo hello
`
		runID := "run-123"
		params := "param1=value1"
		selector := map[string]string{
			"gpu":    "true",
			"region": "us-east-1",
		}

		task := executor.CreateTask(
			dagName,
			yamlDefinition,
			coordinatorv1.Operation_OPERATION_START,
			runID,
			executor.WithTaskParams(params),
			executor.WithWorkerSelector(selector),
		)

		assert.NotNil(t, task)
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, runID, task.RootDagRunId)
		assert.Equal(t, coordinatorv1.Operation_OPERATION_START, task.Operation)
		assert.Equal(t, runID, task.DagRunId)
		assert.Equal(t, "test-dag", task.Target)
		assert.Equal(t, params, task.Params)
		assert.Equal(t, selector, task.WorkerSelector)
		assert.Equal(t, yamlDefinition, task.Definition)
		// Parent fields should be empty when no options provided
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("WithRootDagRunOption", func(t *testing.T) {
		t.Parallel()

		dag := &core.DAG{
			Name: "child-dag",
		}

		rootRef := execution.DAGRunRef{
			Name: "root-dag",
			ID:   "root-run-123",
		}

		task := executor.CreateTask(
			dag.Name,
			string(dag.YamlData),
			coordinatorv1.Operation_OPERATION_RETRY,
			"child-run-456",
			executor.WithRootDagRun(rootRef),
		)

		assert.Equal(t, "root-dag", task.RootDagRunName)
		assert.Equal(t, "root-run-123", task.RootDagRunId)
		assert.Equal(t, "child-run-456", task.DagRunId)
		assert.Equal(t, "child-dag", task.Target)
	})

	t.Run("WithParentDagRunOption", func(t *testing.T) {
		t.Parallel()

		parentRef := execution.DAGRunRef{
			Name: "parent-dag",
			ID:   "parent-run-789",
		}

		task := executor.CreateTask(
			"child-dag",
			`name: child-dag`,
			coordinatorv1.Operation_OPERATION_START,
			"child-run-456",
			executor.WithParentDagRun(parentRef),
		)

		assert.Equal(t, "parent-dag", task.ParentDagRunName)
		assert.Equal(t, "parent-run-789", task.ParentDagRunId)
		assert.Equal(t, "child-dag", task.RootDagRunName)
		assert.Equal(t, "child-run-456", task.RootDagRunId)
	})

	t.Run("WithMultipleOptions", func(t *testing.T) {
		t.Parallel()

		rootRef := execution.DAGRunRef{
			Name: "root-dag",
			ID:   "root-run-123",
		}
		parentRef := execution.DAGRunRef{
			Name: "parent-dag",
			ID:   "parent-run-456",
		}

		task := executor.CreateTask(
			"grandchild-dag",
			`name: grandchild-dag`,
			coordinatorv1.Operation_OPERATION_START,
			"grandchild-run-789",
			executor.WithTaskParams("nested=true"),
			executor.WithWorkerSelector(map[string]string{"env": "prod"}),
			executor.WithRootDagRun(rootRef),
			executor.WithParentDagRun(parentRef),
		)

		assert.Equal(t, "root-dag", task.RootDagRunName)
		assert.Equal(t, "root-run-123", task.RootDagRunId)
		assert.Equal(t, "parent-dag", task.ParentDagRunName)
		assert.Equal(t, "parent-run-456", task.ParentDagRunId)
		assert.Equal(t, "grandchild-run-789", task.DagRunId)
		assert.Equal(t, "grandchild-dag", task.Target)
		assert.Equal(t, "nested=true", task.Params)
		assert.Equal(t, map[string]string{"env": "prod"}, task.WorkerSelector)
	})

	t.Run("EmptyWorkerSelector", func(t *testing.T) {
		t.Parallel()

		task := executor.CreateTask(
			"test-dag",
			`name: test-dag`,
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
		)

		assert.Nil(t, task.WorkerSelector)
	})

	t.Run("OptionsWithEmptyRefs", func(t *testing.T) {
		t.Parallel()

		// Test that empty refs don't modify the task
		emptyRootRef := execution.DAGRunRef{}
		emptyParentRef := execution.DAGRunRef{Name: "", ID: ""}

		task := executor.CreateTask(
			"test-dag",
			`name: test-dag`,
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
			executor.WithRootDagRun(emptyRootRef),
			executor.WithParentDagRun(emptyParentRef),
		)

		// Should use DAG name and runID as root values
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, "run-123", task.RootDagRunId)
		// Parent fields should remain empty
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("PartiallyEmptyRefs", func(t *testing.T) {
		t.Parallel()

		// Test refs with only one field set
		partialRootRef := execution.DAGRunRef{Name: "root-dag", ID: ""}
		partialParentRef := execution.DAGRunRef{Name: "", ID: "parent-id"}

		task := executor.CreateTask(
			"test-dag",
			`name: test-dag`,
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
			executor.WithRootDagRun(partialRootRef),
			executor.WithParentDagRun(partialParentRef),
		)

		// Partial refs should not modify the task
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, "run-123", task.RootDagRunId)
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("CustomTaskOption", func(t *testing.T) {
		t.Parallel()

		// Create a custom task option
		withStep := func(step string) executor.TaskOption {
			return func(task *coordinatorv1.Task) {
				task.Step = step
			}
		}

		task := executor.CreateTask(
			"test-dag",
			`name: test-dag`,
			coordinatorv1.Operation_OPERATION_RETRY,
			"run-123",
			withStep("step-2"),
		)

		assert.Equal(t, "step-2", task.Step)
		assert.Equal(t, coordinatorv1.Operation_OPERATION_RETRY, task.Operation)
	})

	t.Run("AllOperationTypes", func(t *testing.T) {
		t.Parallel()

		operations := []coordinatorv1.Operation{
			coordinatorv1.Operation_OPERATION_UNSPECIFIED,
			coordinatorv1.Operation_OPERATION_START,
			coordinatorv1.Operation_OPERATION_RETRY,
		}

		for _, op := range operations {
			task := executor.CreateTask(
				"test-dag",
				`name: test-dag`,
				op,
				"run-123",
			)
			assert.Equal(t, op, task.Operation)
		}
	})
}

func TestTaskOption_Functions(t *testing.T) {
	t.Parallel()

	t.Run("WithRootDagRun", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}
		ref := execution.DAGRunRef{Name: "root", ID: "123"}

		executor.WithRootDagRun(ref)(task)

		assert.Equal(t, "root", task.RootDagRunName)
		assert.Equal(t, "123", task.RootDagRunId)
	})

	t.Run("WithParentDagRun", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}
		ref := execution.DAGRunRef{Name: "parent", ID: "456"}

		executor.WithParentDagRun(ref)(task)

		assert.Equal(t, "parent", task.ParentDagRunName)
		assert.Equal(t, "456", task.ParentDagRunId)
	})

	t.Run("WithTaskParams", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}

		executor.WithTaskParams("key1=value1 key2=value2")(task)

		assert.Equal(t, "key1=value1 key2=value2", task.Params)
	})

	t.Run("WithWorkerSelector", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}
		selector := map[string]string{
			"gpu":    "true",
			"region": "us-west-2",
		}

		executor.WithWorkerSelector(selector)(task)

		assert.Equal(t, selector, task.WorkerSelector)
	})

	t.Run("WithStep", func(t *testing.T) {
		t.Parallel()

		task := &coordinatorv1.Task{}

		executor.WithStep("step-name")(task)

		assert.Equal(t, "step-name", task.Step)
	})
}
