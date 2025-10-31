package runtime_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/common/sock"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/test"
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
				status := transform.NewStatusBuilder(dag.DAG).Create(
					dagRunID, core.Running, 0, time.Now(),
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

		dag.AssertCurrentStatus(t, core.Running)

		_ = socketServer.Shutdown(ctx)

		dag.AssertCurrentStatus(t, core.NotStarted)
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
		att, err := th.DAGRunStore.CreateAttempt(ctx, dag.DAG, now, dagRunID, execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		err = att.Open(ctx)
		require.NoError(t, err)

		dagRunStatus := testNewStatus(dag.DAG, dagRunID, core.Succeeded, core.NodeSucceeded)

		err = att.Write(ctx, dagRunStatus)
		require.NoError(t, err)
		_ = att.Close(ctx)

		// Get the status and check if it is the same as the one we wrote.
		ref := execution.NewDAGRunRef(dag.Name, dagRunID)
		statusToCheck, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded, statusToCheck.Nodes[0].Status)

		// Update the status.
		newStatus := core.NodeFailed
		dagRunStatus.Nodes[0].Status = newStatus

		root := execution.NewDAGRunRef(dag.Name, dagRunID)
		err = cli.UpdateStatus(ctx, root, dagRunStatus)
		require.NoError(t, err)

		statusByDAGRunID, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)

		require.Equal(t, 1, len(dagRunStatus.Nodes))
		require.Equal(t, newStatus, statusByDAGRunID.Nodes[0].Status)
	})
	t.Run("UpdateSubDAGRunStatus", func(t *testing.T) {
		dag := th.DAG(t, `
steps:
  - name: "1"
    call: tree_child
---
name: tree_child
steps:
  - name: "1"
    command: "true"
---
`)

		spec := th.SubCmdBuilder.Start(dag.DAG, runtime.StartOptions{})
		err := runtime.Start(th.Context, spec)
		require.NoError(t, err)

		dag.AssertLatestStatus(t, core.Succeeded)

		// Get the sub dag-run status.
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		dagRunID := dagRunStatus.DAGRunID
		subDAGRun := dagRunStatus.Nodes[0].SubRuns[0]

		root := execution.NewDAGRunRef(dag.Name, dagRunID)
		subDAGRunStatus, err := th.DAGRunMgr.FindSubDAGRunStatus(th.Context, root, subDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded.String(), subDAGRunStatus.Status.String())

		// Update the the sub dag-run status.
		subDAGRunStatus.Nodes[0].Status = core.NodeFailed
		err = th.DAGRunMgr.UpdateStatus(th.Context, root, *subDAGRunStatus)
		require.NoError(t, err)

		// Check if the sub dag-run status is updated.
		subDAGRunStatus, err = th.DAGRunMgr.FindSubDAGRunStatus(th.Context, root, subDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, core.NodeFailed.String(), subDAGRunStatus.Nodes[0].Status.String())
	})
	t.Run("InvalidUpdateStatusWithInvalidDAGRunID", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: sleep 1
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// update with invalid dag-run ID.
		status := testNewStatus(dag.DAG, "unknown-req-id", core.Failed, core.NodeFailed)

		// Check if the update fails.
		root := execution.NewDAGRunRef(dag.Name, "unknown-req-id")
		err := cli.UpdateStatus(ctx, root, status)
		require.Error(t, err)
	})
}

func testNewStatus(dag *core.DAG, dagRunID string, dagStatus core.Status, nodeStatus core.NodeStatus) execution.DAGRunStatus {
	nodes := []runtime.NodeData{{State: runtime.NodeState{Status: nodeStatus}}}
	tm := time.Now()
	startedAt := &tm
	return transform.NewStatusBuilder(dag).Create(dagRunID, dagStatus, 0, *startedAt, transform.WithNodes(nodes))
}
