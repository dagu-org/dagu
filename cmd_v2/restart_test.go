package cmd_v2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
)

func TestRestartCommand(t *testing.T) {
	dagFile := testDAGFile("restart.yaml")

	// Start the DAG.
	go func() {
		testRunCommand(t, startCommand, cmdTest{args: []string{"start", `--params="foo"`, dagFile}})
	}()

	time.Sleep(time.Millisecond * 50)

	// Wait for the DAG running.
	testStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Restart the DAG.
	go func() {
		testRunCommand(t, restartCommand, cmdTest{
			args:        []string{"restart", dagFile},
			expectedOut: []string{"Restarting"}})
	}()

	time.Sleep(time.Millisecond * 50)

	// Check the last execution is cancelled.
	testLastStatusEventual(t, dagFile, scheduler.SchedulerStatus_Cancel)

	// Wait for the DAG running again.
	testStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Stop the restarted DAG.
	testRunCommand(t, stopCommand, cmdTest{args: []string{"stop", dagFile}})

	// Check parameter was the same as the first execution
	d, err := loadDAG(dagFile, "")
	require.NoError(t, err)
	ctrl := controller.NewDAGController(d)
	sts := ctrl.GetRecentStatuses(2)
	require.Len(t, sts, 2)
	require.Equal(t, sts[0].Status.Params, sts[1].Status.Params)
}
