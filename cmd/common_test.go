// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/persistence"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

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
	return filepath.Join(
		filepath.Join(fileutil.MustGetwd(), "testdata"),
		name,
	)
}

const (
	waitForStatusTimeout = time.Millisecond * 5000
	tick                 = time.Millisecond * 50
)

// testStatusEventual tests the status of a DAG to be the expected status.
func testStatusEventual(t *testing.T, e client.Client, dagFile string, expected scheduler.Status) {
	t.Helper()

	cfg, err := config.Load()
	require.NoError(t, err)

	workflow, err := digraph.Load(cfg.BaseConfig, dagFile, "")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		status, err := e.GetCurrentStatus(workflow)
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
