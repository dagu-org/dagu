package main

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

func TestStopCommand(t *testing.T) {
	t.Run("StopDAG", func(t *testing.T) {
		th := testSetup(t)

		dagFile := th.DAG(t, "cmd/stop.yaml")

		done := make(chan struct{})
		go func() {
			// Start the DAG to stop.
			args := []string{"start", dagFile.Location}
			th.RunCommand(t, startCmd(), cmdTest{args: args})
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)

		// Wait for the DAG running.
		dagFile.AssertLatestStatus(t, scheduler.StatusRunning)

		// Stop the DAG.
		th.RunCommand(t, stopCmd(), cmdTest{
			args:        []string{"stop", dagFile.Location},
			expectedOut: []string{"DAG stopped"}})

		// Log the status of the DAG.
		go func() {
			for {
				select {
				case <-time.After(time.Millisecond * 100):
					status, err := th.Client.GetLatestStatus(th.Context, dagFile.DAG)
					require.NoError(t, err)
					t.Logf("status: %s, started: %s, finished: %s", status.Status, status.StartedAt, status.FinishedAt)
				case <-done:
					return
				}
			}
		}()

		// Check the DAG is stopped.
		dagFile.AssertLatestStatus(t, scheduler.StatusCancel)
		<-done
	})
}
