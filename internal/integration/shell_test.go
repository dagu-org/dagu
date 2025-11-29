package integration_test

import (
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/test"
)

func TestShellExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix shell tests on Windows")
	}
	t.Parallel()

	th := test.Setup(t)

	t.Run("DAGLevelShellWithArgs", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: "/bin/bash -e"
steps:
  - name: test
    script: |
      echo "hello"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "hello",
		})
	})

	t.Run("StepLevelShellOverride", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: /bin/sh
steps:
  - name: test
    shell: "/bin/bash -e"
    script: |
      echo "from bash"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "from bash",
		})
	})

	t.Run("ErrexitBehavior", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: "/bin/bash -e"
steps:
  - name: test
    script: |
      false
      echo "should not reach"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunError(t)
	})

	t.Run("ShellCmdArgsWithPipe", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: /bin/bash
steps:
  - name: test
    command: echo hello | tr 'h' 'H'
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "Hello",
		})
	})
}
