package cmd_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
)

func TestStopCommand(t *testing.T) {
	t.Run("StopDAGRun", func(t *testing.T) {
		th := test.SetupCommand(t)

		dag := th.DAG(t, `steps:
  - name: "1"
    script: "sleep 10"
`)

		done := make(chan struct{})
		go func() {
			// Start the DAG to stop.
			args := []string{"start", dag.Location}
			th.RunCommand(t, cmd.Start(), test.CmdTest{Args: args})
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)

		// Wait for the dag-run running.
		dag.AssertLatestStatus(t, status.Running)

		// Stop the dag-run.
		th.RunCommand(t, cmd.Stop(), test.CmdTest{
			Args:        []string{"stop", dag.Location},
			ExpectedOut: []string{"stopped"}})

		// Check the dag-run is stopped.
		dag.AssertLatestStatus(t, status.Cancel)
		<-done
	})
	t.Run("StopDAGRunWithRunID", func(t *testing.T) {
		th := test.SetupCommand(t)

		dag := th.DAG(t, `steps:
  - name: "1"
    script: "sleep 10"
`)

		done := make(chan struct{})
		dagRunID := uuid.Must(uuid.NewV7()).String()
		go func() {
			// Start the dag-run to stop.
			args := []string{"start", "--run-id=" + dagRunID, dag.Location}
			th.RunCommand(t, cmd.Start(), test.CmdTest{Args: args})
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)

		// Wait for the dag-run running
		dag.AssertLatestStatus(t, status.Running)

		// Stop the dag-run with a specific run ID.
		th.RunCommand(t, cmd.Stop(), test.CmdTest{
			Args:        []string{"stop", dag.Location, "--run-id=" + dagRunID},
			ExpectedOut: []string{"stopped"}})

		// Check the dag-run is stopped.
		dag.AssertLatestStatus(t, status.Cancel)
		<-done
	})
}
