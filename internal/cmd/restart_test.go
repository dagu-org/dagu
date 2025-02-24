package cmd_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRestartCommand(t *testing.T) {
	t.Run("RestartDAG", func(t *testing.T) {
		th := test.SetupCommand(t)

		dag := th.DAG(t, "cmd/restart.yaml")

		go func() {
			// Start the DAG to restart.
			args := []string{"start", `--params="foo"`, dag.Location}
			th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})
		}()

		// Wait for the DAG to be running.
		dag.AssertCurrentStatus(t, scheduler.StatusRunning)

		// Restart the DAG.
		done := make(chan struct{})

		go func() {
			defer close(done)
			args := []string{"restart", dag.Location}
			th.RunCommand(t, cmd.CmdRestart(), test.CmdTest{Args: args})
		}()

		// Wait for the DAG running again.
		dag.AssertCurrentStatus(t, scheduler.StatusRunning)

		time.Sleep(time.Millisecond * 300) // Wait a bit (need to investigate why this is needed).

		// Stop the restarted DAG.
		th.RunCommand(t, cmd.CmdStop(), test.CmdTest{Args: []string{"stop", dag.Location}})

		// Wait for the DAG is stopped.
		dag.AssertCurrentStatus(t, scheduler.StatusNone)

		// Check parameter was the same as the first execution
		loaded, err := digraph.Load(th.Context, dag.Location, digraph.WithBaseConfig(th.Config.Paths.BaseConfig))
		require.NoError(t, err)

		// Check parameter was the same as the first execution
		setup := cmd.SetupWithConfig(th.Context, th.Config)
		client, err := setup.Client()
		require.NoError(t, err)

		time.Sleep(time.Millisecond * 300) // Wait for the history to be updated.

		recentHistory := client.GetRecentHistory(th.Context, loaded, 2)

		require.Len(t, recentHistory, 2)
		require.Equal(t, recentHistory[0].Status.Params, recentHistory[1].Status.Params)

		<-done
	})
}
