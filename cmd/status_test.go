package cmd

import (
	"github.com/dagu-dev/dagu/internal/scheduler"
	"os"
	"testing"
	"time"
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

	time.Sleep(time.Millisecond * 50)

	// TODO: do not use history store directly.
	testLastStatusEventual(t, df.NewHistoryStore(), dagFile, scheduler.SchedulerStatus_Running)

	// Check the current status.
	testRunCommand(t, createStatusCommand(), cmdTest{
		args:        []string{"status", dagFile},
		expectedOut: []string{"Status=running"},
	})

	// Stop the DAG.
	testRunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dagFile}})
	<-done
}
