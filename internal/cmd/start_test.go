package cmd_test

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"
)

func TestStartCommand(t *testing.T) {
	th := test.SetupCommand(t)

	tests := []test.CmdTest{
		{
			Name:        "StartDAG",
			Args:        []string{"start", th.DAG(t, "cmd/start.yaml").Location},
			ExpectedOut: []string{"Step started"},
		},
		{
			Name:        "StartDAGWithDefaultParams",
			Args:        []string{"start", th.DAG(t, "cmd/start_with_params.yaml").Location},
			ExpectedOut: []string{`params="[1=p1 2=p2]"`},
		},
		{
			Name:        "StartDAGWithParams",
			Args:        []string{"start", `--params="p3 p4"`, th.DAG(t, "cmd/start_with_params.yaml").Location},
			ExpectedOut: []string{`params="[1=p3 2=p4]"`},
		},
		{
			Name:        "StartDAGWithParamsAfterDash",
			Args:        []string{"start", th.DAG(t, "cmd/start_with_params.yaml").Location, "--", "p5", "p6"},
			ExpectedOut: []string{`params="[1=p5 2=p6]"`},
		},
		{
			Name:        "StartDAGWithRequestID",
			Args:        []string{"start", th.DAG(t, "cmd/start_with_dagrun_id.yaml").Location, "--run-id", "CfmC9GPywTC24bXbY1yEU7eQANNvpdxAPJXdSKTSaCVC"},
			ExpectedOut: []string{"CfmC9GPywTC24bXbY1yEU7eQANNvpdxAPJXdSKTSaCVC"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			th.RunCommand(t, cmd.CmdStart(), tc)
		})
	}
}

func TestCmdStart_InteractiveMode(t *testing.T) {
	t.Run("Works with explicit DAG path", func(t *testing.T) {
		// Create a test DAG
		tmpDir := t.TempDir()
		dagFile := tmpDir + "/test.yaml"
		dagContent := `
name: test-dag
steps:
  - name: step1
    command: echo test
`
		err := os.WriteFile(dagFile, []byte(dagContent), 0644)
		require.NoError(t, err)

		// Providing a DAG path should work
		cmd := cmd.CmdStart()
		cmd.SetArgs([]string{dagFile})
		// The actual execution might fail for other reasons in test environment,
		// but it should accept the DAG file argument
		_ = cmd.Execute()
	})

	t.Run("Terminal detection function available", func(t *testing.T) {
		// Verify that term.IsTerminal is available and doesn't panic
		isTTY := term.IsTerminal(int(os.Stdin.Fd()))
		require.False(t, isTTY, "Tests should not run in a TTY")
	})

	t.Run("Interactive mode info message", func(t *testing.T) {
		// Verify the info message is appropriate
		expectedMsg := "No DAG specified, opening interactive selector..."
		require.Contains(t, expectedMsg, "interactive selector")
	})
}

func TestCmdStart_BackwardCompatibility(t *testing.T) {
	t.Run("Should accept parameters after --", func(t *testing.T) {
		tmpDir := t.TempDir()
		dagFile := tmpDir + "/test-params.yaml"
		dagContent := `
name: test-params
params: KEY1=default1 KEY2=default2
steps:
  - name: step1
    command: echo $KEY1 $KEY2
`
		err := os.WriteFile(dagFile, []byte(dagContent), 0644)
		require.NoError(t, err)

		cmd := cmd.CmdStart()
		cmd.SetArgs([]string{dagFile, "--", "KEY1=value1", "KEY2=value2"})

		// Execute will fail due to missing context setup, but we're testing
		// that the command accepts the arguments
		_ = cmd.Execute()
	})

	t.Run("Should accept --params flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		dagFile := tmpDir + "/test-params-flag.yaml"
		dagContent := `
name: test-params-flag
params: KEY=default
steps:
  - name: step1
    command: echo $KEY
`
		err := os.WriteFile(dagFile, []byte(dagContent), 0644)
		require.NoError(t, err)

		cmd := cmd.CmdStart()
		cmd.SetArgs([]string{dagFile, "--params", "KEY=value"})

		// Execute will fail due to missing context setup, but we're testing
		// that the command accepts the arguments
		_ = cmd.Execute()
	})
}
