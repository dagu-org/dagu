package cli_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cli"
	"github.com/dagu-org/dagu/internal/digraph/builder"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRestartCommand(t *testing.T) {
	th := test.SetupCommand(t)

	dag := th.DAG(t, `params: "p1"
steps:
  - name: "1"
    script: "echo $1"
  - name: "2"
    script: "sleep 1"
`)

	go func() {
		// Start the DAG to restart.
		args := []string{"start", `--params="foo"`, dag.Location}
		th.RunCommand(t, cli.CmdStart(), test.CmdTest{Args: args})
	}()

	// Wait for the DAG to be running.
	dag.AssertCurrentStatus(t, status.Running)

	// Restart the DAG.
	done := make(chan struct{})

	go func() {
		defer close(done)
		args := []string{"restart", dag.Location}
		th.RunCommand(t, cli.CmdRestart(), test.CmdTest{Args: args})
	}()

	// Wait for the dag-run running again.
	dag.AssertCurrentStatus(t, status.Running)

	// Stop the restarted DAG.
	th.RunCommand(t, cli.CmdStop(), test.CmdTest{Args: []string{"stop", dag.Location}})

	// Wait for the DAG is stopped.
	dag.AssertCurrentStatus(t, status.None)

	// Check parameter was the same as the first execution
	loaded, err := builder.Load(th.Context, dag.Location, builder.WithBaseConfig(th.Config.Paths.BaseConfig))
	require.NoError(t, err)

	time.Sleep(time.Millisecond * 1000) // Wait for the history to be updated.

	// Check parameter was the same as the first execution
	recentHistory := th.DAGRunMgr.ListRecentStatus(th.Context, loaded.Name, 2)

	require.Len(t, recentHistory, 2)
	require.Equal(t, recentHistory[0].Params, recentHistory[1].Params)

	<-done
}
