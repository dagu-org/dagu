package cmd

import (
	"testing"

	"github.com/dagu-dev/dagu/internal/dag/scheduler"
)

func TestStatusCommand(t *testing.T) {
	t.Run("StatusDAG", func(t *testing.T) {
		setup := setupTest(t)
		defer setup.cleanup()

		dagFile := testDAGFile("status.yaml")

		// Start the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, startCmd(), cmdTest{args: []string{"start", dagFile}})
			close(done)
		}()

		testLastStatusEventual(t, setup.dataStore.HistoryStore(), dagFile, scheduler.StatusRunning)

		// Check the current status.
		testRunCommand(t, statusCmd(), cmdTest{
			args:        []string{"status", dagFile},
			expectedOut: []string{"Status=running"},
		})

		// Stop the DAG.
		testRunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dagFile}})
		<-done
	})
}
