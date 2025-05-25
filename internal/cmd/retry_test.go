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
	t.Run("RetryDAG", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/retry.yaml")

		// Run a DAG.
		args := []string{"start", `--params="foo"`, dagFile.Location}
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})

		// Find the workflow ID.
		cli := th.DAGStore
		ctx := context.Background()

		dag, err := cli.GetMetadata(ctx, dagFile.Location)
		require.NoError(t, err)

		status, err := th.HistoryMgr.GetLatestStatus(ctx, dag)
		require.NoError(t, err)
		require.Equal(t, status.Status, scheduler.StatusSuccess)
		require.NotNil(t, status.Status)

		// Retry with the workflow ID.
		args = []string{"retry", fmt.Sprintf("--workflow-id=%s", status.RunID), dagFile.Location}
		th.RunCommand(t, cmd.CmdRetry(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{`[1=foo]`},
		})
	})
}
