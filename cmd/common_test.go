// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/test"

	"github.com/dagu-org/dagu/internal/client"
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

func (th testHelper) DAGFile(name string) string {
	return filepath.Join(filepath.Join(fileutil.MustGetwd(), "testdata"), name)
}

func (th testHelper) RunCommand(t *testing.T, cmd *cobra.Command, testCase cmdTest) {
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

// testStatusEventual tests the status of a DAG to be the expected status.
func testStatusEventual(t *testing.T, e client.Client, dagFile string, expected scheduler.Status) {
	t.Helper()

	cfg, err := config.Load()
	require.NoError(t, err)

	dag, err := digraph.Load(context.Background(), cfg.BaseConfig, dagFile, "")
	require.NoError(t, err)

	ctx := context.Background()
	require.Eventually(t, func() bool {
		status, err := e.GetCurrentStatus(ctx, dag)
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
		status := hs.ReadStatusRecent(context.Background(), dg, 1)
		if len(status) < 1 {
			return false
		}
		return expected == status[0].Status.Status
	}, waitForStatusTimeout, tick)
}
