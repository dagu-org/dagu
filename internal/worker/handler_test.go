package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/dagu-org/dagu/internal/worker"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTaskHandler(t *testing.T) {
	th := test.Setup(t)

	t.Run("HandleTaskRetry", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: echo step1
  - name: "2"
    command: echo step2
`)
		ctx := th.Context

		// First, start a DAG run
		spec := th.SubCmdBuilder.Start(dag.DAG, dagrun.StartOptions{})
		err := dagrun.Start(th.Context, spec)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.Success)

		// Get the st to get the dag-run ID
		st, err := th.DAGRunMgr.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		dagRunID := st.DAGRunID

		// Create a retry task
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_RETRY,
			DagRunId:  dagRunID,
			Target:    dag.Name,
		}

		// Create a context with timeout for the task execution
		taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Execute the task
		handler := worker.NewTaskHandler(th.Config)
		err = handler.Handle(taskCtx, task)
		require.NoError(t, err)

		// Verify the DAG ran again successfully
		dag.AssertLatestStatus(t, status.Success)
	})

	t.Run("HandleTaskRetryWithStep", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: echo step1
  - name: "2"
    command: echo step2
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// First, start a DAG run
		spec := th.SubCmdBuilder.Start(dag.DAG, dagrun.StartOptions{})
		err := dagrun.Start(th.Context, spec)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.Success)

		// Get the st to get the dag-run ID
		st, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		dagRunID := st.DAGRunID

		// Create a retry task with specific step
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_RETRY,
			DagRunId:  dagRunID,
			Target:    dag.Name,
			Step:      "1",
		}

		// Create a context with timeout for the task execution
		taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Execute the task
		handler := worker.NewTaskHandler(th.Config)
		err = handler.Handle(taskCtx, task)
		require.NoError(t, err)

		// Verify the DAG ran again successfully
		dag.AssertLatestStatus(t, status.Success)
	})

	t.Run("HandleTaskStart", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "process"
    command: echo processing $1
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// Generate a new dag-run ID
		dagRunID := uuid.Must(uuid.NewV7()).String()

		// Create a start task
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_START,
			DagRunId:  dagRunID,
			Target:    dag.Location,
			Params:    "param1=value1",
		}

		// Create a context with timeout for the task execution
		taskCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Execute the task
		handler := worker.NewTaskHandler(th.Config)
		err := handler.Handle(taskCtx, task)
		require.NoError(t, err)

		// Verify the DAG ran successfully
		dag.AssertLatestStatus(t, status.Success)

		// Verify the params were passed
		status, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, dagRunID, status.DAGRunID)
		require.Equal(t, "param1=value1", status.Params)
	})

	t.Run("HandleTaskInvalidOperation", func(t *testing.T) {
		ctx := th.Context

		// Create a task with invalid operation
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_UNSPECIFIED,
			DagRunId:  "test-id",
			Target:    "test-dag",
		}

		// Execute the task
		handler := worker.NewTaskHandler(th.Config)
		err := handler.Handle(ctx, task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "operation not specified")
	})
}
