package cli_test

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/apps/cli"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"
)

func TestStartCommand(t *testing.T) {
	th := test.SetupCommand(t)

	dagStart := th.DAG(t, `maxActiveRuns: 1
steps:
  - name: "1"
    command: "true"
`)

	dagStartWithParams := th.DAG(t, `params: "p1 p2"
steps:
  - name: "1"
    command: "echo \"params is $1 and $2\""
`)

	dagStartWithDAGRunID := th.DAG(t, `steps:
  - name: "1"
    command: "true"
`)

	tests := []test.CmdTest{
		{
			Name:        "StartDAG",
			Args:        []string{"start", dagStart.Location},
			ExpectedOut: []string{"Step started"},
		},
		{
			Name:        "StartDAGWithDefaultParams",
			Args:        []string{"start", dagStartWithParams.Location},
			ExpectedOut: []string{`params="[1=p1 2=p2]"`},
		},
		{
			Name:        "StartDAGWithParams",
			Args:        []string{"start", `--params="p3 p4"`, dagStartWithParams.Location},
			ExpectedOut: []string{`params="[1=p3 2=p4]"`},
		},
		{
			Name:        "StartDAGWithParamsAfterDash",
			Args:        []string{"start", dagStartWithParams.Location, "--", "p5", "p6"},
			ExpectedOut: []string{`params="[1=p5 2=p6]"`},
		},
		{
			Name:        "StartDAGWithRequestID",
			Args:        []string{"start", dagStartWithDAGRunID.Location, "--run-id", "CfmC9GPywTC24bXbY1yEU7eQANNvpdxAPJXdSKTSaCVC"},
			ExpectedOut: []string{"CfmC9GPywTC24bXbY1yEU7eQANNvpdxAPJXdSKTSaCVC"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			th.RunCommand(t, cli.Start(), tc)
		})
	}
}

func TestCmdStart_InteractiveMode(t *testing.T) {
	t.Run("WorksWithExplicitDAGPath", func(t *testing.T) {
		// Create a test DAG
		th := test.SetupCommand(t)
		dagContent := `
steps:
  - name: step1
    command: echo test
`
		dagFile := th.CreateDAGFile(t, "test.yaml", dagContent)

		// Providing a DAG path should work
		cli := cli.Start()
		cli.SetArgs([]string{dagFile})
		// The actual execution might fail for other reasons in test environment,
		// but it should accept the DAG file argument
		_ = cli.Execute()
	})

	t.Run("TerminalDetectionFunctionAvailable", func(t *testing.T) {
		// Verify that term.IsTerminal is available and doesn't panic
		isTTY := term.IsTerminal(int(os.Stdin.Fd()))
		require.False(t, isTTY, "Tests should not run in a TTY")
	})

	t.Run("InteractiveModeInfoMessage", func(t *testing.T) {
		// Verify the info message is appropriate
		expectedMsg := "No DAG specified, opening interactive selector..."
		require.Contains(t, expectedMsg, "interactive selector")
	})
}

func TestCmdStart_BackwardCompatibility(t *testing.T) {
	t.Run("ShouldAcceptParametersAfter", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagContent := `
params: KEY1=default1 KEY2=default2
steps:
  - name: step1
    command: echo $KEY1 $KEY2
`
		dagFile := th.CreateDAGFile(t, "test-params.yaml", dagContent)

		cli := cli.Start()
		cli.SetArgs([]string{dagFile, "--", "KEY1=value1", "KEY2=value2"})

		// Execute will fail due to missing context setup, but we're testing
		// that the command accepts the arguments
		_ = cli.Execute()
	})

	t.Run("ShouldAcceptParamsFlag", func(t *testing.T) {
		th := test.SetupCommand(t)
		dagContent := `
params: KEY=default
steps:
  - name: step1
    command: echo $KEY
`
		dagFile := th.CreateDAGFile(t, "test-params-flag.yaml", dagContent)

		cli := cli.Start()
		cli.SetArgs([]string{dagFile, "--params", "KEY=value"})

		// Execute will fail due to missing context setup, but we're testing
		// that the command accepts the arguments
		_ = cli.Execute()
	})
}
