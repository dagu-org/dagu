package dagrun_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/sock"
	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

func TestManager(t *testing.T) {
	th := test.Setup(t)

	t.Run("Valid", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: sleep 1
`)
		ctx := th.Context

		dagRunID := uuid.Must(uuid.NewV7()).String()
		socketServer, _ := sock.NewServer(
			dag.SockAddr(dagRunID),
			func(w http.ResponseWriter, _ *http.Request) {
				status := models.NewStatusBuilder(dag.DAG).Create(
					dagRunID, status.Running, 0, time.Now(),
				)
				w.WriteHeader(http.StatusOK)
				jsonData, err := json.Marshal(status)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write(jsonData)
			},
		)

		go func() {
			_ = socketServer.Serve(ctx, nil)
			_ = socketServer.Shutdown(ctx)
		}()

		dag.AssertCurrentStatus(t, status.Running)

		_ = socketServer.Shutdown(ctx)

		dag.AssertCurrentStatus(t, status.None)
	})
	t.Run("UpdateStatus", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		now := time.Now()
		ctx := th.Context
		cli := th.DAGRunMgr

		// Open the Attempt data and write a status before updating it.
		att, err := th.DAGRunStore.CreateAttempt(ctx, dag.DAG, now, dagRunID, models.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		err = att.Open(ctx)
		require.NoError(t, err)

		dagRunStatus := testNewStatus(dag.DAG, dagRunID, status.Success, status.NodeSuccess)

		err = att.Write(ctx, dagRunStatus)
		require.NoError(t, err)
		_ = att.Close(ctx)

		// Get the status and check if it is the same as the one we wrote.
		ref := digraph.NewDAGRunRef(dag.Name, dagRunID)
		statusToCheck, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, status.NodeSuccess, statusToCheck.Nodes[0].Status)

		// Update the status.
		newStatus := status.NodeError
		dagRunStatus.Nodes[0].Status = newStatus

		root := digraph.NewDAGRunRef(dag.Name, dagRunID)
		err = cli.UpdateStatus(ctx, root, dagRunStatus)
		require.NoError(t, err)

		statusByDAGRunID, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)

		require.Equal(t, 1, len(dagRunStatus.Nodes))
		require.Equal(t, newStatus, statusByDAGRunID.Nodes[0].Status)
	})
	t.Run("UpdateChildDAGRunStatus", func(t *testing.T) {
		// Create child DAG first
		dag := th.DAG(t, `
steps:
  - name: "1"
    run: tree_child
---
name: tree_child
steps:
  - name: "1"
    command: "true"
---
`)

		err := th.DAGRunMgr.StartDAGRunAsync(th.Context, dag.DAG, dagrun.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.Success)

		// Get the child dag-run status.
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		dagRunID := dagRunStatus.DAGRunID
		childDAGRun := dagRunStatus.Nodes[0].Children[0]

		root := digraph.NewDAGRunRef(dag.Name, dagRunID)
		childDAGRunStatus, err := th.DAGRunMgr.FindChildDAGRunStatus(th.Context, root, childDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, status.Success.String(), childDAGRunStatus.Status.String())

		// Update the the child dag-run status.
		childDAGRunStatus.Nodes[0].Status = status.NodeError
		err = th.DAGRunMgr.UpdateStatus(th.Context, root, *childDAGRunStatus)
		require.NoError(t, err)

		// Check if the child dag-run status is updated.
		childDAGRunStatus, err = th.DAGRunMgr.FindChildDAGRunStatus(th.Context, root, childDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, status.NodeError.String(), childDAGRunStatus.Nodes[0].Status.String())
	})
	t.Run("InvalidUpdateStatusWithInvalidDAGRunID", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: sleep 1
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// update with invalid dag-run ID.
		status := testNewStatus(dag.DAG, "unknown-req-id", status.Error, status.NodeError)

		// Check if the update fails.
		root := digraph.NewDAGRunRef(dag.Name, "unknown-req-id")
		err := cli.UpdateStatus(ctx, root, status)
		require.Error(t, err)
	})
}

func TestClient_RunDAG(t *testing.T) {
	th := test.Setup(t)

	t.Run("RunDAG", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)

		err := th.DAGRunMgr.StartDAGRunAsync(th.Context, dag.DAG, dagrun.StartOptions{
			Quiet: true,
		})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.Success)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, status.Success.String(), dagRunStatus.Status.String())
	})
	t.Run("Stop", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: sleep 10
`)
		ctx := th.Context

		err := th.DAGRunMgr.StartDAGRunAsync(ctx, dag.DAG, dagrun.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.Running)

		err = th.DAGRunMgr.Stop(ctx, dag.DAG, "")
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.Cancel)
	})
	t.Run("Restart", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: sleep 1
`)
		ctx := th.Context

		err := th.DAGRunMgr.StartDAGRunAsync(th.Context, dag.DAG, dagrun.StartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.Running)

		err = th.DAGRunMgr.RestartDAG(ctx, dag.DAG, dagrun.RestartOptions{})
		require.NoError(t, err)

		dag.AssertLatestStatus(t, status.Success)
	})
	t.Run("Retry", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		err := cli.StartDAGRunAsync(ctx, dag.DAG, dagrun.StartOptions{Params: "x y z"})
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.Success)

		// Retry the DAG with the same params.
		dagRunStatus, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		prevDAGRunID := dagRunStatus.DAGRunID
		prevParams := dagRunStatus.Params

		time.Sleep(1 * time.Second)

		err = cli.RetryDAGRun(ctx, dag.DAG, prevDAGRunID, true)
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.Success)

		dagRunStatus, err = cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		// Check if the params are the same as the previous run.
		require.Equal(t, prevDAGRunID, dagRunStatus.DAGRunID)
		require.Equal(t, prevParams, dagRunStatus.Params)
	})
	t.Run("RetryStep", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		err := cli.StartDAGRunAsync(ctx, dag.DAG, dagrun.StartOptions{})
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.Success)

		dagRunStatus, err := cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)
		dagRunID := dagRunStatus.DAGRunID
		prevParams := dagRunStatus.Params

		time.Sleep(1 * time.Second)

		err = cli.RetryDAGStep(ctx, dag.DAG, dagRunID, "2")
		require.NoError(t, err)

		// Wait for the DAG to finish again
		dag.AssertLatestStatus(t, status.Success)

		dagRunStatus, err = cli.GetLatestStatus(ctx, dag.DAG)
		require.NoError(t, err)

		// Check if the params are the same as the previous run.
		require.Equal(t, dagRunID, dagRunStatus.DAGRunID)
		require.Equal(t, prevParams, dagRunStatus.Params)
	})
}

func testNewStatus(dag *digraph.DAG, dagRunID string, dagStatus status.Status, nodeStatus status.NodeStatus) models.DAGRunStatus {
	nodes := []scheduler.NodeData{{State: scheduler.NodeState{Status: nodeStatus}}}
	tm := time.Now()
	startedAt := &tm
	return models.NewStatusBuilder(dag).Create(dagRunID, dagStatus, 0, *startedAt, models.WithNodes(nodes))
}

func TestHandleTask(t *testing.T) {
	th := test.Setup(t)

	t.Run("HandleTaskRetry", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: echo step1
  - name: "2"
    command: echo step2
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// First, start a DAG run
		err := cli.StartDAGRunAsync(ctx, dag.DAG, dagrun.StartOptions{})
		require.NoError(t, err)

		// Wait for the DAG to finish
		dag.AssertLatestStatus(t, status.Success)

		// Get the st to get the dag-run ID
		st, err := cli.GetLatestStatus(ctx, dag.DAG)
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
		err = cli.HandleTask(taskCtx, task)
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
		err := cli.StartDAGRunAsync(ctx, dag.DAG, dagrun.StartOptions{})
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
		err = cli.HandleTask(taskCtx, task)
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
		err := cli.HandleTask(taskCtx, task)
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
		cli := th.DAGRunMgr

		// Create a task with invalid operation
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_UNSPECIFIED,
			DagRunId:  "test-id",
			Target:    "test-dag",
		}

		// Execute the task
		err := cli.HandleTask(ctx, task)
		require.Error(t, err)
		require.Contains(t, err.Error(), "operation not specified")
	})
}
