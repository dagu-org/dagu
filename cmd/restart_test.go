package cmd

import (
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"github.com/yohamta/dagu/internal/scheduler"
	"testing"
	"time"
)

func TestRestartCommand(t *testing.T) {
	dagFile := testDAGFile("restart.yaml")

	// Start the DAG.
	go func() {
		testRunCommand(t, startCmd(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})
	}()

	time.Sleep(time.Millisecond * 100)

	// Wait for the DAG running.
	testStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Restart the DAG.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, restartCmd(), cmdTest{args: []string{"restart", dagFile}})
		close(done)
	}()

	time.Sleep(time.Millisecond * 100)

	// Wait for the DAG running again.
	testStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Stop the restarted DAG.
	testRunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dagFile}})

	time.Sleep(time.Millisecond * 100)

	// Wait for the DAG is stopped.
	testStatusEventual(t, dagFile, scheduler.SchedulerStatus_None)

	// Check parameter was the same as the first execution
	d, err := loadDAG(dagFile, "")
	require.NoError(t, err)
	ctrl := controller.New(d, jsondb.New())
	sts := ctrl.GetRecentStatuses(2)
	require.Len(t, sts, 2)
	require.Equal(t, sts[0].Status.Params, sts[1].Status.Params)

	<-done
}
