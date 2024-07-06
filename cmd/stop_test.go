package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/dag/scheduler"
)

func TestStopCommand(t *testing.T) {
	t.Run("Stop a DAG", func(t *testing.T) {
		tmpDir, _, dataStore, _ := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dagFile := testDAGFile("stop.yaml")

		// Start the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, startCmd(), cmdTest{args: []string{"start", dagFile}})
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)

		// Wait for the DAG running.
		testLastStatusEventual(t, dataStore.NewHistoryStore(), dagFile, scheduler.StatusRunning)

		// Stop the DAG.
		testRunCommand(t, stopCmd(), cmdTest{
			args:        []string{"stop", dagFile},
			expectedOut: []string{"Stopping..."}})

		// Check the last execution is cancelled.
		testLastStatusEventual(t, dataStore.NewHistoryStore(), dagFile, scheduler.StatusCancel)
		<-done
	})
}
