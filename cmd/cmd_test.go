package main

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/test"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// cmdTest is a helper struct to test commands.
type cmdTest struct {
	name        string   // Name of the test.
	args        []string // Arguments to pass to the command.
	expectedOut []string // Expected output to be present in the standard output / error.
}

// testHelper is a helper struct to test commands.
type testHelper struct {
	test.Helper
}

func (th testHelper) RunCommand(t *testing.T, cmd *cobra.Command, testCase cmdTest) {
	t.Helper()

	cmdRoot := &cobra.Command{Use: "root"}
	cmdRoot.AddCommand(cmd)

	// Set arguments.
	cmdRoot.SetArgs(testCase.args)

	// Run the command
	err := cmdRoot.ExecuteContext(th.Context)
	require.NoError(t, err)

	output := th.LoggingOutput.String()

	// Check if the expected output is present in the standard output.
	for _, expectedOutput := range testCase.expectedOut {
		require.Contains(t, output, expectedOutput)
	}
}

func testSetup(t *testing.T) testHelper {
	t.Helper()

	return testHelper{Helper: test.Setup(t, test.WithCaptureLoggingOutput())}
}

const (
	waitForStatusTimeout = time.Millisecond * 3000 // timeout for waiting for status becoming expected.
	statusCheckInterval  = time.Millisecond * 50   // tick for checking status.
)
