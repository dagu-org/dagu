package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestValidateCommand(t *testing.T) {
	th := test.SetupCommand(t)

	t.Run("ValidSpec", func(t *testing.T) {
		dag := th.DAG(t, `
steps:
  - echo ok
`)

		th.RunCommand(t, cmd.Validate(), test.CmdTest{
			Args:        []string{"validate", dag.Location},
			ExpectedOut: []string{"DAG spec is valid"},
		})
	})

	t.Run("InvalidDependency", func(t *testing.T) {
		// This DAG has a step depending on a non-existent step
		dagFile := th.CreateDAGFile(t, "invalid.yaml", `
steps:
  - echo A
  - name: "b"
    command: echo B
    depends: ["missing_step"]
`)

		err := th.RunCommandWithError(t, cmd.Validate(), test.CmdTest{
			Args: []string{"validate", dagFile},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Validation failed")
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		// This DAG has invalid YAML syntax
		dagFile := th.CreateDAGFile(t, "invalid_yaml.yaml", `
steps:
  - name: "test"
    command: echo test
  invalid yaml here: [[[
`)

		err := th.RunCommandWithError(t, cmd.Validate(), test.CmdTest{
			Args: []string{"validate", dagFile},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Validation failed")
	})

	t.Run("MissingFile", func(t *testing.T) {
		err := th.RunCommandWithError(t, cmd.Validate(), test.CmdTest{
			Args: []string{"validate", "/nonexistent/file.yaml"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "Validation failed")
	})
}
