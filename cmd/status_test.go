package cmd

import (
	"github.com/yohamta/dagu/internal/scheduler"
	"testing"
	"time"
)

func TestStatusCommand(t *testing.T) {
	dagFile := testDAGFile("status.yaml")

	// Start the DAG.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, startCmd(), cmdTest{args: []string{"start", dagFile}})
		close(done)
	}()

	time.Sleep(time.Millisecond * 50)

	// Wait for the DAG running.
	testLastStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Check the current status.
	testRunCommand(t, createStatusCommand(), cmdTest{
		args:        []string{"status", dagFile},
		expectedOut: []string{"Status=running"},
	})

	// Stop the DAG.
	testRunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dagFile}})
	<-done
}
