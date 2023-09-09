package cmd

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRetryCommand(t *testing.T) {
	dagFile := testDAGFile("retry.yaml")

	// Run a DAG.
	testRunCommand(t, startCmd(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})

	// Find the request ID.
	e := engine.NewFactory().Create()
	s, err := e.ReadStatus(dagFile, false)
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
