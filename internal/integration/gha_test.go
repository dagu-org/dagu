package integration_test

import (
	"os/exec"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
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
    executor:
      type: github_action
      config:
        runner: node:25-bookworm
    params:
      repository: dagu-org/dagu
      sparse-checkout: README.md
`)

		// Initialize git repo in the temp dir to satisfy act requirements
		cmd := exec.Command("git", "init", dag.WorkingDir)
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to init git repo: %v", err)
		}

		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
	})
}
