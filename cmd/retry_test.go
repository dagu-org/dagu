package cmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/stretchr/testify/require"
)

func TestRetryCommand(t *testing.T) {
	t.Run("Retry a DAG", func(t *testing.T) {
		tmpDir, eng, _, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dagFile := testDAGFile("retry.yaml")

		// Run a DAG.
		testRunCommand(t, startCmd(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})

		// Find the request ID.
		status, err := eng.GetStatus(dagFile)
		require.NoError(t, err)
		require.Equal(t, status.Status.Status, scheduler.StatusSuccess)
		require.NotNil(t, status.Status)

		reqID := status.Status.RequestId

		// Retry with the request ID.
		testRunCommand(t, retryCmd(), cmdTest{
			args:        []string{"retry", fmt.Sprintf("--req=%s", reqID), dagFile},
			expectedOut: []string{"param is foo"},
		})
	})
}
