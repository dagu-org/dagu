package cmd

import (
	"os"
	"testing"

	"github.com/dagu-dev/dagu/internal/dag/scheduler"
)

func TestStatusCommand(t *testing.T) {
	t.Run("Status command should run", func(t *testing.T) {
		tmpDir, _, df, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dagFile := testDAGFile("status.yaml")

		// Start the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, startCmd(), cmdTest{args: []string{"start", dagFile}})
			close(done)
		}()

		testLastStatusEventual(t, df.NewHistoryStore(), dagFile, scheduler.StatusRunning)

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
