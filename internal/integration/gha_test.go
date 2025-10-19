package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
)

func TestGitHubActionsExecutor(t *testing.T) {
	t.Parallel()

	t.Run("BasicExecution", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: test-action
    executor:
      type: github-action
      config:
        action: actions/hello-world-javascript-action@main
        time-to-greet: "Morning"
    output: ACTION_OUTPUT
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)

		// Verify that container output was captured to stdout
		// The hello-world action should log "Hello Morning" to console
		dag.AssertOutputs(t, map[string]any{
			"ACTION_OUTPUT": []test.Contains{
				"Hello Morning",
			},
		})
	})
}
