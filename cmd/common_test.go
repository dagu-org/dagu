package cmd

import (
	"bytes"
	"io"
	"log"
	"os"
	"path"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/client"

	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (string, engine.Engine, persistence.DataStoreFactory) {
	t.Helper()

	tmpDir := utils.MustTempDir("dagu_test")
	changeHomeDir(tmpDir)

	ds := client.NewDataStoreFactory(&config.Config{
		DataDir: path.Join(tmpDir, ".dagu", "data"),
	})

	e := engine.NewFactory(ds, nil).Create()

	return tmpDir, e, ds
}

func changeHomeDir(dir string) {
	homeDir = dir
	_ = os.Setenv("HOME", dir)
	_ = config.LoadConfig(dir)
}

type cmdTest struct {
	args        []string
	expectedOut []string
}

func testRunCommand(t *testing.T, cmd *cobra.Command, test cmdTest) {
	t.Helper()

	root := &cobra.Command{Use: "root"}
	root.AddCommand(cmd)

	// Set arguments.
	root.SetArgs(test.args)

	// Run the command.
	out := withSpool(t, func() {
		err := root.Execute()
		require.NoError(t, err)
	})

	// Check outputs.
	for _, s := range test.expectedOut {
		require.Contains(t, out, s)
	}
}

func withSpool(t *testing.T, f func()) string {
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

	f()

	os.Stdout = origStdout
	_ = w.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	return buf.String()
}

func testDAGFile(name string) string {
	d := path.Join(utils.MustGetwd(), "testdata")
	return path.Join(d, name)
}

func testStatusEventual(t *testing.T, e engine.Engine, dagFile string, expected scheduler.SchedulerStatus) {
	t.Helper()

	d, err := loadDAG(dagFile, "")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		status, err := e.GetCurrentStatus(d)
		require.NoError(t, err)
		return expected == status.Status
	}, time.Millisecond*5000, time.Millisecond*50)
}

func testLastStatusEventual(t *testing.T, hs persistence.HistoryStore, dag string, expected scheduler.SchedulerStatus) {
	t.Helper()
	require.Eventually(t, func() bool {
		// TODO: do not use history store directly.
		status := hs.ReadStatusRecent(dag, 1)
		if len(status) < 1 {
			return false
		}
		return expected == status[0].Status.Status
	}, time.Millisecond*5000, time.Millisecond*50)
}
