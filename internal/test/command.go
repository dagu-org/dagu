package test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// CmdTest is a helper struct to test commands.
type CmdTest struct {
	Name        string   // Name of the test.
	Args        []string // Arguments to pass to the command.
	ExpectedOut []string // Expected output to be present in the standard output / error.
}

// Command is a helper struct to test commands.
type Command struct {
	Helper
}

func (th Command) RunCommand(t *testing.T, cmd *cobra.Command, testCase CmdTest) {
	t.Helper()

	cmdRoot := &cobra.Command{Use: "root"}
	cmdRoot.AddCommand(cmd)

	// Set arguments.
	cmdRoot.SetArgs(testCase.Args)

	// Run the command
	err := cmdRoot.ExecuteContext(th.Context)
	require.NoError(t, err)

	output := th.LoggingOutput.String()

	// Check if the expected output is present in the standard output.
	for _, expectedOutput := range testCase.ExpectedOut {
		require.Contains(t, output, expectedOutput)
	}
}

func SetupCommand(t *testing.T, opts ...HelperOption) Command {
	t.Helper()

	opts = append(opts, WithCaptureLoggingOutput())
	return Command{Helper: Setup(t, opts...)}
}
