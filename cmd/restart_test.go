package main

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

func TestRestartCommand(t *testing.T) {
	t.Run("RestartDAG", func(t *testing.T) {
		th := testSetup(t)

		dag := th.DAG(t, "cmd/restart.yaml")

		go func() {
			// Start the DAG to restart.
			args := []string{"start", `--params="foo"`, dag.Location}
			th.RunCommand(t, startCmd(), cmdTest{args: args})
		}()

		// Wait for the DAG to be running.
		dag.AssertCurrentStatus(t, scheduler.StatusRunning)

		// Restart the DAG.
		done := make(chan struct{})
		go func() {
			args := []string{"restart", dag.Location}
			th.RunCommand(t, restartCmd(), cmdTest{args: args})
			close(done)
		}()

		// Wait for the DAG running again.
		dag.AssertCurrentStatus(t, scheduler.StatusRunning)

		// Stop the restarted DAG.
		th.RunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dag.Location}})

		// Wait for the DAG is stopped.
		dag.AssertCurrentStatus(t, scheduler.StatusNone)

		// Check parameter was the same as the first execution
		env := newENV(th.Config)
		client, err := env.client()
		require.NoError(t, err)

		recentHistory := client.GetRecentHistory(context.Background(), dag.DAG, 2)

		require.Len(t, recentHistory, 2)
		require.Equal(t, recentHistory[0].Status.Params, recentHistory[1].Status.Params)

		<-done
	})
}
