package cmd_test

import (
	"errors"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRecordEarlyFailure(t *testing.T) {
	t.Run("RecordsFailureForNewDAGRun", func(t *testing.T) {
		th := test.SetupCommand(t)

		dag := th.DAG(t, `
steps:
  - name: step1
    command: echo hello
`)

		dagRunID := "test-run-id-001"
		testErr := errors.New("process acquisition failed")

		// Create Context with required stores
		ctx := &cmd.Context{
			Context:     th.Context,
			Config:      th.Config,
			DAGRunStore: th.DAGRunStore,
		}

		// Record the early failure
		err := ctx.RecordEarlyFailure(dag.DAG, dagRunID, testErr)
		require.NoError(t, err)

		// Verify the failure was recorded
		ref := execution.NewDAGRunRef(dag.Name, dagRunID)
		attempt, err := th.DAGRunStore.FindAttempt(th.Context, ref)
		require.NoError(t, err)
		require.NotNil(t, attempt)

		status, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)
		require.Equal(t, core.Failed, status.Status)
		require.Contains(t, status.Error, "process acquisition failed")
	})

	t.Run("RecordsFailureForExistingAttempt", func(t *testing.T) {
		th := test.SetupCommand(t)

		dag := th.DAG(t, `
steps:
  - name: step1
    command: echo hello
`)

		// First, run the DAG to create an attempt
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args: []string{"start", dag.Location},
		})

		// Get the existing run ID
		latestStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		dagRunID := latestStatus.DAGRunID

		// Now record an early failure for the same run ID
		testErr := errors.New("retry failed due to lock contention")

		ctx := &cmd.Context{
			Context:     th.Context,
			Config:      th.Config,
			DAGRunStore: th.DAGRunStore,
		}

		err = ctx.RecordEarlyFailure(dag.DAG, dagRunID, testErr)
		require.NoError(t, err)

		// Verify the failure was recorded (status should be updated)
		ref := execution.NewDAGRunRef(dag.Name, dagRunID)
		attempt, err := th.DAGRunStore.FindAttempt(th.Context, ref)
		require.NoError(t, err)
		require.NotNil(t, attempt)

		status, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)
		require.Equal(t, core.Failed, status.Status)
		require.Contains(t, status.Error, "retry failed due to lock contention")
	})

	t.Run("ReturnsErrorForNilDAG", func(t *testing.T) {
		th := test.SetupCommand(t)

		ctx := &cmd.Context{
			Context:     th.Context,
			Config:      th.Config,
			DAGRunStore: th.DAGRunStore,
		}

		err := ctx.RecordEarlyFailure(nil, "some-run-id", errors.New("test error"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "DAG and dag-run ID are required")
	})

	t.Run("ReturnsErrorForEmptyDAGRunID", func(t *testing.T) {
		th := test.SetupCommand(t)

		dag := th.DAG(t, `
steps:
  - name: step1
    command: echo hello
`)

		ctx := &cmd.Context{
			Context:     th.Context,
			Config:      th.Config,
			DAGRunStore: th.DAGRunStore,
		}

		err := ctx.RecordEarlyFailure(dag.DAG, "", errors.New("test error"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "DAG and dag-run ID are required")
	})
}
