package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/stretchr/testify/require"
)

const (
	waitForStatusUpdate = time.Millisecond * 100
)

func TestRestartCommand(t *testing.T) {
	t.Run("Restart a DAG", func(t *testing.T) {
		tmpDir, eng, _, cfg := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dagFile := testDAGFile("restart.yaml")

		// Start the DAG.
		go func() {
			testRunCommand(
				t,
				startCmd(),
				cmdTest{args: []string{"start", `--params="foo"`, dagFile}},
			)
		}()

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG running.
		testStatusEventual(t, eng, dagFile, scheduler.StatusRunning)

		// Restart the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, restartCmd(), cmdTest{args: []string{"restart", dagFile}})
			close(done)
		}()

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG running again.
		testStatusEventual(t, eng, dagFile, scheduler.StatusRunning)

		// Stop the restarted DAG.
		testRunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dagFile}})

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG is stopped.
		testStatusEventual(t, eng, dagFile, scheduler.StatusNone)

		// Check parameter was the same as the first execution
		dg, err := loadDAG(cfg, dagFile, "")
		require.NoError(t, err)

		recentHistory := newEngine(cfg).GetRecentHistory(dg, 2)

		require.Len(t, recentHistory, 2)
		require.Equal(t, recentHistory[0].Status.Params, recentHistory[1].Status.Params)

		<-done
	})
}
