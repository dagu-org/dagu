package cmd

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestRetryCommand(t *testing.T) {
	tmpDir, e, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	dagFile := testDAGFile("retry.yaml")

	// Run a DAG.
	testRunCommand(t, startCmd(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})

	// Find the request ID.
	s, err := e.GetStatus(dagFile)
	require.NoError(t, err)
	require.Equal(t, s.Status.Status, scheduler.SchedulerStatus_Success)
	require.NotNil(t, s.Status)

	reqID := s.Status.RequestId

	// Retry with the request ID.
	testRunCommand(t, retryCmd(), cmdTest{
		args:        []string{"retry", fmt.Sprintf("--req=%s", reqID), dagFile},
		expectedOut: []string{"param is foo"},
	})
}
