package digraph_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAG(t *testing.T) {
	th := test.Setup(t)
	t.Run("String", func(t *testing.T) {
		dag := th.DAG(t, `
name: default
steps:
  - name: "1"
    command: "true"
`)
		ret := dag.String()
		require.Contains(t, ret, "Name: default")
	})
}

func TestUnixSocket(t *testing.T) {
	t.Run("Location", func(t *testing.T) {
		dag := &digraph.DAG{Location: "testdata/testDag.yml"}
		require.Regexp(t, `^/tmp/@dagu_testdata_testDag_yml_[0-9a-f]+\.sock$`, dag.SockAddr(""))
	})
	t.Run("MaxUnixSocketLength", func(t *testing.T) {
		dag := &digraph.DAG{
			Location: "testdata/testDagVeryLongNameThatExceedsUnixSocketLengthMaximum-testDagVeryLongNameThatExceedsUnixSocketLengthMaximum.yml",
		}
		// 50 is the maximum length of a unix socket address
		require.LessOrEqual(t, 50, len(dag.SockAddr("")))
		require.Equal(
			t,
			"/tmp/@dagu_testdata_testDagVeryLongNameThat_21bace.sock",
			dag.SockAddr(""),
		)
	})
}

func TestMarshalJSON(t *testing.T) {
	th := test.Setup(t)
	t.Run("MarshalJSON", func(t *testing.T) {
		dag := th.DAG(t, `
name: default
steps:
  - name: "1"
    command: "true"
`)
		_, err := json.Marshal(dag.DAG)
		require.NoError(t, err)
	})
}

func TestScheduleJSON(t *testing.T) {
	t.Run("MarshalUnmarshalJSON", func(t *testing.T) {
		// Create a Schedule with a valid cron expression
		original := digraph.Schedule{
			Expression: "0 0 * * *", // Run at midnight every day
		}

		// Parse the expression to populate the Parsed field
		parsed, err := cron.ParseStandard(original.Expression)
		require.NoError(t, err)
		original.Parsed = parsed

		// Marshal to JSON
		data, err := json.Marshal(original)
		require.NoError(t, err)

		// Verify JSON format (camelCase field names)
		jsonStr := string(data)
		require.Contains(t, jsonStr, `"expression":"0 0 * * *"`)
		require.NotContains(t, jsonStr, `"Expression"`)
		require.NotContains(t, jsonStr, `"Parsed"`)

		// Unmarshal back to a Schedule struct
		var unmarshaled digraph.Schedule
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		// Verify the unmarshaled struct has the correct values
		require.Equal(t, original.Expression, unmarshaled.Expression)

		// Verify the Parsed field was populated correctly
		require.NotNil(t, unmarshaled.Parsed)

		// Test that the next scheduled time is the same for both objects
		// This verifies that the Parsed field was correctly populated during unmarshaling
		now := time.Now()
		expectedNext := original.Parsed.Next(now)
		actualNext := unmarshaled.Parsed.Next(now)
		require.Equal(t, expectedNext, actualNext)
	})

	t.Run("UnmarshalInvalidCron", func(t *testing.T) {
		// Test unmarshaling with an invalid cron expression
		invalidJSON := `{"expression":"invalid cron"}`

		var schedule digraph.Schedule
		err := json.Unmarshal([]byte(invalidJSON), &schedule)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid cron expression")
	})
}

func TestDAG_CreateTask(t *testing.T) {
	t.Run("BasicTaskCreation", func(t *testing.T) {
		dag := &digraph.DAG{
			Name:     "test-dag",
			YamlData: []byte("name: test-dag\nsteps:\n  - name: step1\n    command: echo hello"),
		}

		runID := "run-123"
		params := "param1=value1"
		selector := map[string]string{
			"gpu":    "true",
			"region": "us-east-1",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			runID,
			digraph.WithTaskParams(params),
			digraph.WithWorkerSelector(selector),
		)

		assert.NotNil(t, task)
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, runID, task.RootDagRunId)
		assert.Equal(t, coordinatorv1.Operation_OPERATION_START, task.Operation)
		assert.Equal(t, runID, task.DagRunId)
		assert.Equal(t, "test-dag", task.Target)
		assert.Equal(t, params, task.Params)
		assert.Equal(t, selector, task.WorkerSelector)
		assert.Equal(t, string(dag.YamlData), task.Definition)
		// Parent fields should be empty when no options provided
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("WithRootDagRunOption", func(t *testing.T) {
		dag := &digraph.DAG{
			Name: "child-dag",
		}

		rootRef := digraph.DAGRunRef{
			Name: "root-dag",
			ID:   "root-run-123",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_RETRY,
			"child-run-456",
			digraph.WithRootDagRun(rootRef),
		)

		assert.Equal(t, "root-dag", task.RootDagRunName)
		assert.Equal(t, "root-run-123", task.RootDagRunId)
		assert.Equal(t, "child-run-456", task.DagRunId)
		assert.Equal(t, "child-dag", task.Target)
	})

	t.Run("WithParentDagRunOption", func(t *testing.T) {
		dag := &digraph.DAG{
			Name: "child-dag",
		}

		parentRef := digraph.DAGRunRef{
			Name: "parent-dag",
			ID:   "parent-run-789",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"child-run-456",
			digraph.WithParentDagRun(parentRef),
		)

		assert.Equal(t, "parent-dag", task.ParentDagRunName)
		assert.Equal(t, "parent-run-789", task.ParentDagRunId)
		assert.Equal(t, "child-dag", task.RootDagRunName)
		assert.Equal(t, "child-run-456", task.RootDagRunId)
	})

	t.Run("WithMultipleOptions", func(t *testing.T) {
		dag := &digraph.DAG{
			Name:     "grandchild-dag",
			YamlData: []byte("name: grandchild-dag"),
		}

		rootRef := digraph.DAGRunRef{
			Name: "root-dag",
			ID:   "root-run-123",
		}
		parentRef := digraph.DAGRunRef{
			Name: "parent-dag",
			ID:   "parent-run-456",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"grandchild-run-789",
			digraph.WithTaskParams("nested=true"),
			digraph.WithWorkerSelector(map[string]string{"env": "prod"}),
			digraph.WithRootDagRun(rootRef),
			digraph.WithParentDagRun(parentRef),
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
		dag := &digraph.DAG{
			Name: "test-dag",
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
		)

		assert.Nil(t, task.WorkerSelector)
	})

	t.Run("OptionsWithEmptyRefs", func(t *testing.T) {
		dag := &digraph.DAG{
			Name: "test-dag",
		}

		// Test that empty refs don't modify the task
		emptyRootRef := digraph.DAGRunRef{}
		emptyParentRef := digraph.DAGRunRef{Name: "", ID: ""}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
			digraph.WithRootDagRun(emptyRootRef),
			digraph.WithParentDagRun(emptyParentRef),
		)

		// Should use DAG name and runID as root values
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, "run-123", task.RootDagRunId)
		// Parent fields should remain empty
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("PartiallyEmptyRefs", func(t *testing.T) {
		dag := &digraph.DAG{
			Name: "test-dag",
		}

		// Test refs with only one field set
		partialRootRef := digraph.DAGRunRef{Name: "root-dag", ID: ""}
		partialParentRef := digraph.DAGRunRef{Name: "", ID: "parent-id"}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
			digraph.WithRootDagRun(partialRootRef),
			digraph.WithParentDagRun(partialParentRef),
		)

		// Partial refs should not modify the task
		assert.Equal(t, "test-dag", task.RootDagRunName)
		assert.Equal(t, "run-123", task.RootDagRunId)
		assert.Empty(t, task.ParentDagRunName)
		assert.Empty(t, task.ParentDagRunId)
	})

	t.Run("CustomTaskOption", func(t *testing.T) {
		dag := &digraph.DAG{
			Name: "test-dag",
		}

		// Create a custom task option
		withStep := func(step string) digraph.TaskOption {
			return func(task *coordinatorv1.Task) {
				task.Step = step
			}
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_RETRY,
			"run-123",
			withStep("step-2"),
		)

		assert.Equal(t, "step-2", task.Step)
		assert.Equal(t, coordinatorv1.Operation_OPERATION_RETRY, task.Operation)
	})

	t.Run("NilYamlData", func(t *testing.T) {
		dag := &digraph.DAG{
			Name:     "test-dag",
			YamlData: nil,
		}

		task := dag.CreateTask(
			coordinatorv1.Operation_OPERATION_START,
			"run-123",
		)

		assert.Empty(t, task.Definition)
	})

	t.Run("AllOperationTypes", func(t *testing.T) {
		dag := &digraph.DAG{
			Name: "test-dag",
		}

		operations := []coordinatorv1.Operation{
			coordinatorv1.Operation_OPERATION_UNSPECIFIED,
			coordinatorv1.Operation_OPERATION_START,
			coordinatorv1.Operation_OPERATION_RETRY,
		}

		for _, op := range operations {
			task := dag.CreateTask(op, "run-123")
			assert.Equal(t, op, task.Operation)
		}
	})
}

func TestTaskOption_Functions(t *testing.T) {
	t.Run("WithRootDagRun", func(t *testing.T) {
		task := &coordinatorv1.Task{}
		ref := digraph.DAGRunRef{Name: "root", ID: "123"}

		digraph.WithRootDagRun(ref)(task)

		assert.Equal(t, "root", task.RootDagRunName)
		assert.Equal(t, "123", task.RootDagRunId)
	})

	t.Run("WithParentDagRun", func(t *testing.T) {
		task := &coordinatorv1.Task{}
		ref := digraph.DAGRunRef{Name: "parent", ID: "456"}

		digraph.WithParentDagRun(ref)(task)

		assert.Equal(t, "parent", task.ParentDagRunName)
		assert.Equal(t, "456", task.ParentDagRunId)
	})

	t.Run("WithTaskParams", func(t *testing.T) {
		task := &coordinatorv1.Task{}

		digraph.WithTaskParams("key1=value1 key2=value2")(task)

		assert.Equal(t, "key1=value1 key2=value2", task.Params)
	})

	t.Run("WithWorkerSelector", func(t *testing.T) {
		task := &coordinatorv1.Task{}
		selector := map[string]string{
			"gpu":    "true",
			"region": "us-west-2",
		}

		digraph.WithWorkerSelector(selector)(task)

		assert.Equal(t, selector, task.WorkerSelector)
	})

	t.Run("WithStep", func(t *testing.T) {
		task := &coordinatorv1.Task{}

		digraph.WithStep("step-name")(task)

		assert.Equal(t, "step-name", task.Step)
	})
}
