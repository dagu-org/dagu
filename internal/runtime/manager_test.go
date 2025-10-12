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
	core1 "github.com/dagu-org/dagu/internal/core"
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
					dagRunID, core1.Running, 0, time.Now(),
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

		dag.AssertCurrentStatus(t, core1.Running)

		_ = socketServer.Shutdown(ctx)

		dag.AssertCurrentStatus(t, core1.None)
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

		dagRunStatus := testNewStatus(dag.DAG, dagRunID, core1.Success, core1.NodeSuccess)

		err = att.Write(ctx, dagRunStatus)
		require.NoError(t, err)
		_ = att.Close(ctx)

		// Get the status and check if it is the same as the one we wrote.
		ref := core.NewDAGRunRef(dag.Name, dagRunID)
		statusToCheck, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)
		require.Equal(t, core1.NodeSuccess, statusToCheck.Nodes[0].Status)

		// Update the status.
		newStatus := core1.NodeError
		dagRunStatus.Nodes[0].Status = newStatus

		root := core.NewDAGRunRef(dag.Name, dagRunID)
		err = cli.UpdateStatus(ctx, root, dagRunStatus)
		require.NoError(t, err)

		statusByDAGRunID, err := cli.GetSavedStatus(ctx, ref)
		require.NoError(t, err)

		require.Equal(t, 1, len(dagRunStatus.Nodes))
		require.Equal(t, newStatus, statusByDAGRunID.Nodes[0].Status)
	})
	t.Run("UpdateChildDAGRunStatus", func(t *testing.T) {
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

		spec := th.SubCmdBuilder.Start(dag.DAG, runtime.StartOptions{})
		err := runtime.Start(th.Context, spec)
		require.NoError(t, err)

		dag.AssertLatestStatus(t, core1.Success)

		// Get the child dag-run status.
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		dagRunID := dagRunStatus.DAGRunID
		childDAGRun := dagRunStatus.Nodes[0].Children[0]

		root := core.NewDAGRunRef(dag.Name, dagRunID)
		childDAGRunStatus, err := th.DAGRunMgr.FindChildDAGRunStatus(th.Context, root, childDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, core1.Success.String(), childDAGRunStatus.Status.String())

		// Update the the child dag-run status.
		childDAGRunStatus.Nodes[0].Status = core1.NodeError
		err = th.DAGRunMgr.UpdateStatus(th.Context, root, *childDAGRunStatus)
		require.NoError(t, err)

		// Check if the child dag-run status is updated.
		childDAGRunStatus, err = th.DAGRunMgr.FindChildDAGRunStatus(th.Context, root, childDAGRun.DAGRunID)
		require.NoError(t, err)
		require.Equal(t, core1.NodeError.String(), childDAGRunStatus.Nodes[0].Status.String())
	})
	t.Run("InvalidUpdateStatusWithInvalidDAGRunID", func(t *testing.T) {
		dag := th.DAG(t, `steps:
  - name: "1"
    command: sleep 1
`)
		ctx := th.Context
		cli := th.DAGRunMgr

		// update with invalid dag-run ID.
		status := testNewStatus(dag.DAG, "unknown-req-id", core1.Error, core1.NodeError)

		// Check if the update fails.
		root := core.NewDAGRunRef(dag.Name, "unknown-req-id")
		err := cli.UpdateStatus(ctx, root, status)
		require.Error(t, err)
	})
}

func testNewStatus(dag *core.DAG, dagRunID string, dagStatus core1.Status, nodeStatus core1.NodeStatus) execution.DAGRunStatus {
	nodes := []runtime.NodeData{{State: runtime.NodeState{Status: nodeStatus}}}
	tm := time.Now()
	startedAt := &tm
	return transform.NewStatusBuilder(dag).Create(dagRunID, dagStatus, 0, *startedAt, transform.WithNodes(nodes))
}
