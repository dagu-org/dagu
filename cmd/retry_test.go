package cmd

import (
	"fmt"
	"testing"

	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"

	"github.com/stretchr/testify/require"
)

func TestRetryCommand(t *testing.T) {
	dagFile := testDAGFile("retry.yaml")

	// Run a DAG.
	testRunCommand(t, startCommand(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})

	// Find the request ID.
	dsts, err := controller.NewDAGStatusReader().ReadStatus(dagFile, false)
	require.NoError(t, err)
	require.Equal(t, dsts.Status.Status, scheduler.SchedulerStatus_Success)
	require.NotNil(t, dsts.Status)

	reqID := dsts.Status.RequestId

	// Retry with the request ID.
	testRunCommand(t, retryCommand(), cmdTest{
		args:        []string{"retry", fmt.Sprintf("--req=%s", reqID), dagFile},
		expectedOut: []string{"param is foo"},
	})
}
