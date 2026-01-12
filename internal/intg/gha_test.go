package intg_test

import (
	"os/exec"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestGitHubActionsExecutor(t *testing.T) {
	t.Skip("skip")
	t.Run("BasicExecution", func(t *testing.T) {
		tmpDir := t.TempDir()

		th := test.Setup(t)
		dag := th.DAG(t, `
workingDir: `+tmpDir+`
steps:
  - name: test-action
    command: actions/checkout@v4
    type: github_action
    config:
      runner: node:25-bookworm
    params:
      repository: dagu-org/dagu
      sparse-checkout: README.md
`)

		// Verify git is available
		_, err := exec.LookPath("git")
		require.NoError(t, err, "git is required for this test but not found in PATH")

		// Initialize git repo in the temp dir to satisfy act requirements
		cmd := exec.Command("git", "init", dag.WorkingDir)
		require.NoError(t, cmd.Run(), "failed to init git repo")

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
	})
}
