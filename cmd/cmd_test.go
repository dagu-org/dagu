package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/test"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// cmdTest is a helper struct to test commands.
// It contains the arguments to the command and the expected output.
type cmdTest struct {
	name        string
	args        []string
	expectedOut []string
}

type testHelper struct {
	test.Helper
}

func (th testHelper) DAGFile(name string) testDAG {
	return testDAG{
		testHelper: th,

		Path: filepath.Join(filepath.Join(fileutil.MustGetwd(), "testdata"), name),
	}
}

type testDAG struct {
	testHelper
	Path string
}

func (td *testDAG) AssertCurrentStatus(t *testing.T, expected scheduler.Status) {
	t.Helper()

	dag, err := digraph.Load(td.Context, td.Path, digraph.WithBaseConfig(td.Config.Paths.BaseConfig))
	require.NoError(t, err)

	cli := td.Client
	require.Eventually(t, func() bool {
		status, err := cli.GetCurrentStatus(td.Context, dag)
		require.NoError(t, err)
		return expected == status.Status
	}, waitForStatusTimeout, tick)
}

func (th *testDAG) AssertLastStatus(t *testing.T, expected scheduler.Status) {
	t.Helper()

	require.Eventually(t, func() bool {
		status := th.HistoryStore.ReadStatusRecent(th.Context, th.Path, 1)
		if len(status) < 1 {
			return false
		}
		return expected == status[0].Status.Status
	}, waitForStatusTimeout, tick)
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
	waitForStatusTimeout = time.Millisecond * 3000
	tick                 = time.Millisecond * 50
)
