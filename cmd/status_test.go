package cmd

import (
	"github.com/dagu-dev/dagu/internal/scheduler"
	"os"
	"testing"
)

func TestStatusCommand(t *testing.T) {
	tmpDir, _, df := setupTest(t)
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
}
