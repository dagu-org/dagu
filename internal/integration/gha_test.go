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
`)
		agent := dag.Agent()

		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Succeeded)
	})
}
