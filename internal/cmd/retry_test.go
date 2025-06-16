package cmd_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRetryCommand(t *testing.T) {
	t.Run("RetryDAGWithFilePath", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/retry.yaml")

		// Run a DAG.
		args := []string{"start", `--params="foo"`, dagFile.Location}
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})

		// Find the dag-run ID.
		cli := th.DAGStore
		ctx := context.Background()

		dag, err := cli.GetMetadata(ctx, dagFile.Location)
		require.NoError(t, err)

		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag)
		require.NoError(t, err)
		require.Equal(t, status.Status, scheduler.StatusSuccess)
		require.NotNil(t, status.Status)

		// Retry with the dag-run ID using file path.
		args = []string{"retry", fmt.Sprintf("--run-id=%s", status.DAGRunID), dagFile.Location}
		th.RunCommand(t, cmd.CmdRetry(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{`[1=foo]`},
		})
	})

	t.Run("RetryDAGWithName", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/retry.yaml")

		// Run a DAG.
		args := []string{"start", `--params="bar"`, dagFile.Location}
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})

		// Find the dag-run ID.
		cli := th.DAGStore
		ctx := context.Background()

		dag, err := cli.GetMetadata(ctx, dagFile.Location)
		require.NoError(t, err)

		status, err := th.DAGRunMgr.GetLatestStatus(ctx, dag)
		require.NoError(t, err)
		require.Equal(t, status.Status, scheduler.StatusSuccess)
		require.NotNil(t, status.Status)

		// Retry with the dag-run ID using DAG name.
		args = []string{"retry", fmt.Sprintf("--run-id=%s", status.DAGRunID), dag.Name}
		th.RunCommand(t, cmd.CmdRetry(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{`[1=bar]`},
		})
	})
}
