package main

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
)

func TestStopCommand(t *testing.T) {
	t.Run("StopDAG", func(t *testing.T) {
		th := testSetup(t)

		dagFile := th.DAGFile("long2.yaml")

		done := make(chan struct{})
		go func() {
			// Start the DAG to stop.
			args := []string{"start", dagFile.Path}
			th.RunCommand(t, startCmd(), cmdTest{args: args})
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)

		// Wait for the DAG running.
		dagFile.AssertLastStatus(t, scheduler.StatusRunning)

		// Stop the DAG.
		th.RunCommand(t, stopCmd(), cmdTest{
			args:        []string{"stop", dagFile.Path},
			expectedOut: []string{"DAG stopped"}})

		// Check the DAG is stopped.
		dagFile.AssertLastStatus(t, scheduler.StatusCancel)
		<-done
	})
}
