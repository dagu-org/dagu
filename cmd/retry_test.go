package main_test

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
		th := test.SetupCommandTest(t)

		dagFile := th.DAG(t, "cmd/retry.yaml")

		// Run a DAG.
		args := []string{"start", `--params="foo"`, dagFile.Location}
		th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})

		// Find the request ID.
		cli := th.Client
		ctx := context.Background()
		status, err := cli.GetStatus(ctx, dagFile.Location)
		require.NoError(t, err)
		require.Equal(t, status.Status.Status, scheduler.StatusSuccess)
		require.NotNil(t, status.Status)

		requestID := status.Status.RequestID

		// Retry with the request ID.
		args = []string{"retry", fmt.Sprintf("--request-id=%s", requestID), dagFile.Location}
		th.RunCommand(t, cmd.CmdRetry(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{`[1=foo]`},
		})
	})
}
