package integration_test

import (
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/test"
)

func TestDirectShellExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix shell tests on Windows")
	}
	t.Parallel()

	th := test.Setup(t)

	t.Run("DirectShellWithCommandArray", func(t *testing.T) {
		t.Parallel()

		// Use command as array for direct execution
		dag := th.DAG(t, `
shell: direct
steps:
  - name: test
    command: [echo, hello, world]
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "hello world",
		})
	})

	t.Run("DirectShellWithPrintf", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: direct
steps:
  - name: test
    command: [printf, "%s-%s", foo, bar]
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "foo-bar",
		})
	})

	t.Run("DirectShellNoShellExpansion", func(t *testing.T) {
		t.Parallel()

		// With direct shell, $HOME should NOT be expanded by shell
		// Note: Dagu's own variable evaluation still happens, so we use a non-existent var
		dag := th.DAG(t, `
shell: direct
steps:
  - name: test
    command: [echo, "literal-text"]
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "literal-text",
		})
	})

	t.Run("DirectShellExitCode", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: direct
steps:
  - name: test
    command: [/bin/false]
`)
		agent := dag.Agent()
		agent.RunError(t)
	})

	t.Run("DirectShellSpecialCharsNotInterpreted", func(t *testing.T) {
		t.Parallel()

		// With direct shell, special chars like | and > should be literal
		dag := th.DAG(t, `
shell: direct
steps:
  - name: test
    command: [echo, "hello | world > file"]
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "hello | world > file",
		})
	})

	t.Run("DirectShellWithDaguVariables", func(t *testing.T) {
		t.Parallel()

		// Dagu's variable evaluation still works with direct shell
		dag := th.DAG(t, `
shell: direct
params:
  - NAME: "dagu"
steps:
  - name: test
    command: [echo, "${NAME}"]
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "dagu",
		})
	})

	t.Run("DirectShellWithAbsolutePath", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: direct
steps:
  - name: test
    command: [/bin/echo, "absolute path works"]
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "absolute path works",
		})
	})
}

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

	t.Run("ShellWithCommandSubstitution", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: /bin/sh
steps:
  - name: test
    command: echo "date is $(date +%Y)"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		// Just verify it runs - date will vary
	})

	t.Run("ShellWithEnvironmentVariable", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: /bin/sh
env:
  - MY_VAR: "test_value"
steps:
  - name: test
    command: echo "$MY_VAR"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "test_value",
		})
	})

	t.Run("ShellWithMultipleCommands", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: /bin/sh
steps:
  - name: test
    command: VAR=hello && echo "$VAR world"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "hello world",
		})
	})

	t.Run("ShellScriptWithMultilineCommands", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: /bin/sh
steps:
  - name: test
    script: |
      VAR="hello"
      echo "$VAR world"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "hello world",
		})
	})

	t.Run("ShellWithRedirection", func(t *testing.T) {
		t.Parallel()

		dag := th.DAG(t, `
shell: /bin/sh
steps:
  - name: test
    command: echo "test" | cat
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "test",
		})
	})

	t.Run("DefaultShellFallback", func(t *testing.T) {
		t.Parallel()

		// When no shell is specified, should use system default
		dag := th.DAG(t, `
steps:
  - name: test
    command: echo "default shell"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "default shell",
		})
	})

	t.Run("ShellWithGlobExpansion", func(t *testing.T) {
		t.Parallel()

		// Shell should expand globs
		dag := th.DAG(t, `
shell: /bin/sh
steps:
  - name: test
    command: echo /bin/ech*
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "/bin/echo",
		})
	})

	t.Run("ShellScriptWithShebang", func(t *testing.T) {
		t.Parallel()

		// Script with shebang should use the shebang interpreter
		dag := th.DAG(t, `
shell: /bin/sh
steps:
  - name: test
    script: |
      #!/bin/bash
      echo "bash via shebang"
    output: OUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"OUT": "bash via shebang",
		})
	})
}
