package cmd

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"

	"github.com/dagu-dev/dagu/internal/dag/scheduler"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// setupTest is a helper function to setup the test environment.
// This function does the following:
// 1. It creates a temporary directory and returns the path to it.
// 2. Sets the home directory to the temporary directory.
// 3. Creates a new data store factory and engine.
func setupTest(t *testing.T) (
	string, engine.Engine, persistence.DataStoreFactory, *config.Config,
) {
	t.Helper()

	tmpDir := util.MustTempDir("dagu_test")
	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)

	cfg.DataDir = path.Join(tmpDir, ".dagu", "data")
	dataStore := newDataStoreFactory(cfg)

	return tmpDir, engine.New(&engine.NewEngineArgs{
		DataStore: dataStore,
	}), dataStore, cfg
}

// cmdTest is a helper struct to test commands.
// It contains the arguments to the command and the expected output.
type cmdTest struct {
	args        []string
	expectedOut []string
}

// testRunCommand is a helper function to test a command.
func testRunCommand(t *testing.T, cmd *cobra.Command, test cmdTest) {
	t.Helper()

	root := &cobra.Command{Use: "root"}
	root.AddCommand(cmd)

	// Set arguments.
	root.SetArgs(test.args)

	// Run the command

	// TODO: Fix thet test after update the logging code so that it can be
	err := root.Execute()
	require.NoError(t, err)

	// configured to write to a buffer.
	// _ = withSpool(t, func() {
	// 	err := root.Execute()
	// 	require.NoError(t, err)
	// })
	//
	// Check if the expected output is present in the standard output.
	// for _, s := range test.expectedOut {
	// 	require.Contains(t, out, s)
	// }
}

// withSpool temporarily buffers the standard output and returns it as a string.
/*
func withSpool(t *testing.T, testFunction func()) string {
	t.Helper()

	origStdout := os.Stdout

	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
		_ = w.Close()
	}()

	testFunction()

	os.Stdout = origStdout
	_ = w.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	out := buf.String()

	t.Cleanup(func() {
		t.Log(out)
	})

	return out
}
*/

func testDAGFile(name string) string {
	return path.Join(
		path.Join(util.MustGetwd(), "testdata"),
		name,
	)
}

const (
	waitForStatusTimeout = time.Millisecond * 5000
	tick                 = time.Millisecond * 50
)

// testStatusEventual tests the status of a DAG to be the expected status.
func testStatusEventual(t *testing.T, e engine.Engine, dagFile string, expected scheduler.Status) {
	t.Helper()

	cfg, err := config.Load()
	require.NoError(t, err)

	dg, err := dag.Load(cfg.BaseConfig, dagFile, "")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		status, err := e.GetCurrentStatus(dg)
		require.NoError(t, err)
		return expected == status.Status
	}, waitForStatusTimeout, tick)
}

// testLastStatusEventual tests the last status of a DAG to be the expected status.
func testLastStatusEventual(
	t *testing.T,
	hs persistence.HistoryStore,
	dg string,
	expected scheduler.Status,
) {
	t.Helper()

	require.Eventually(t, func() bool {
		status := hs.ReadStatusRecent(dg, 1)
		if len(status) < 1 {
			return false
		}
		return expected == status[0].Status.Status
	}, waitForStatusTimeout, tick)
}
