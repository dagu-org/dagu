package main

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

func TestStatusCommand(t *testing.T) {
	t.Run("StatusDAG", func(t *testing.T) {
		th := testSetup(t)

		dagFile := th.DAG(t, "cmd/status.yaml")

		done := make(chan struct{})
		go func() {
			// Start a DAG to check the status.
			args := []string{"start", dagFile.Location}
			th.RunCommand(t, startCmd(), cmdTest{args: args})
			close(done)
		}()

		require.Eventually(t, func() bool {
			status := th.HistoryStore.ReadStatusRecent(th.Context, dagFile.Location, 1)
			if len(status) < 1 {
				return false
			}
			println(status[0].Status.Status.String())
			return scheduler.StatusRunning == status[0].Status.Status
		}, waitForStatusTimeout, statusCheckInterval)

		// Check the current status.
		th.RunCommand(t, statusCmd(), cmdTest{
			args:        []string{"status", dagFile.Location},
			expectedOut: []string{"status=running"},
		})

		// Stop the DAG.
		args := []string{"stop", dagFile.Location}
		th.RunCommand(t, stopCmd(), cmdTest{args: args})
		<-done
	})
}
