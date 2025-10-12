package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
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

	// Start the DAG to restart.
	done1 := make(chan struct{})
	go func() {
		args := []string{"start", `--params="foo"`, dag.Location}
		th.RunCommand(t, cmd.Start(), test.CmdTest{Args: args})
		close(done1)
	}()

	// Wait for the DAG to be running.
	dag.AssertCurrentStatus(t, core.Running)

	// Restart the DAG.
	done2 := make(chan struct{})
	go func() {
		args := []string{"restart", dag.Location}
		th.RunCommand(t, cmd.Restart(), test.CmdTest{Args: args})
		close(done2)
	}()

	// Wait for both executions to complete.
	<-done1
	<-done2

	// Check parameter was the same as the first execution
	loaded, err := spec.Load(th.Context, dag.Location, spec.WithBaseConfig(th.Config.Paths.BaseConfig))
	require.NoError(t, err)

	// Check parameter was the same as the first execution
	recentHistory := th.DAGRunMgr.ListRecentStatus(th.Context, loaded.Name, 2)

	require.Len(t, recentHistory, 2)
	require.Equal(t, recentHistory[0].Params, recentHistory[1].Params)
}
