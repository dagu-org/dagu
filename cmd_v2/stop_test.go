package cmd_v2

import (
	"testing"
	"time"

	"github.com/yohamta/dagu/internal/scheduler"
)

func TestStopCommand(t *testing.T) {
	dagFile := testDAGFile("stop.yaml")

	// Start the DAG.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, startCommand(), cmdTest{args: []string{"start", dagFile}})
		close(done)
	}()

	time.Sleep(time.Millisecond * 50)

	// Wait for the DAG running.
	testLastStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Stop the DAG.
	testRunCommand(t, stopCommand(), cmdTest{
		args:        []string{"stop", dagFile},
		expectedOut: []string{"Stopping..."}})

	// Check the last execution is cancelled.
	testLastStatusEventual(t, dagFile, scheduler.SchedulerStatus_Cancel)
	<-done
}
