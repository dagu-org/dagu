package cmd_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
)

func TestStopCommand(t *testing.T) {
	t.Run("StopDAG", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/stop.yaml")

		done := make(chan struct{})
		go func() {
			// Start the DAG to stop.
			args := []string{"start", dagFile.Location}
			th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)

		// Wait for the DAG running.
		dagFile.AssertLatestStatus(t, scheduler.StatusRunning)

		// Stop the DAG.
		th.RunCommand(t, cmd.CmdStop(), test.CmdTest{
			Args:        []string{"stop", dagFile.Location},
			ExpectedOut: []string{"DAG stopped"}})

		// Check the DAG is stopped.
		dagFile.AssertLatestStatus(t, scheduler.StatusCancel)
		<-done
	})
}
