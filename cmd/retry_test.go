package cmd

import (
	"fmt"
	"testing"

	"github.com/dagu-dev/dagu/internal/dag/scheduler"
	"github.com/dagu-dev/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRetryCommand(t *testing.T) {
	t.Run("RetryDAG", func(t *testing.T) {
		setup := test.Setup(t)
		defer setup.Cleanup()

		dagFile := testDAGFile("retry.yaml")

		// Run a DAG.
		testRunCommand(t, startCmd(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})

		// Find the request ID.
		eng := setup.Engine()
		status, err := eng.GetStatus(dagFile)
		require.NoError(t, err)
		require.Equal(t, status.Status.Status, scheduler.StatusSuccess)
		require.NotNil(t, status.Status)

		reqID := status.Status.RequestID

		// Retry with the request ID.
		testRunCommand(t, retryCmd(), cmdTest{
			args:        []string{"retry", fmt.Sprintf("--req=%s", reqID), dagFile},
			expectedOut: []string{"param is foo"},
		})
	})
}
