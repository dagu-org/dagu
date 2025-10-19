package test

import (
	"os"
	"path/filepath"
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

// RunCommandWithError runs a command and returns the error (if any) without failing the test.
func (th Command) RunCommandWithError(t *testing.T, cmd *cobra.Command, testCase CmdTest) error {
	t.Helper()
	cmdRoot := &cobra.Command{Use: "root"}
	cmdRoot.AddCommand(cmd)

	// Set arguments.
	cmdRoot.SetArgs(testCase.Args)

	// Run the command
	err := cmdRoot.ExecuteContext(th.Context)

	if err == nil {
		output := th.LoggingOutput.String()
		// Check if the expected output is present in the standard output.
		for _, expectedOutput := range testCase.ExpectedOut {
			if len(expectedOutput) > 0 {
				require.Contains(t, output, expectedOutput)
			}
		}
	}

	return err
}

func SetupCommand(t *testing.T, opts ...HelperOption) Command {
	t.Helper()

	opts = append(opts, WithCaptureLoggingOutput())
	return Command{Helper: Setup(t, opts...)}
}

// CreateDAGFile creates a DAG file in the DAGsDir for command tests
func (c Command) CreateDAGFile(t *testing.T, name string, content string) string {
	t.Helper()

	dagFile := filepath.Join(c.Config.Paths.DAGsDir, name)
	// Create the directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(dagFile), 0750)
	require.NoError(t, err)
	// Write the DAG file
	err = os.WriteFile(dagFile, []byte(content), 0600)
	require.NoError(t, err)
	return dagFile
}
