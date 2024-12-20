// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

const (
	waitForStatusUpdate = time.Millisecond * 100
)

func TestRestartCommand(t *testing.T) {
	t.Run("RestartDAG", func(t *testing.T) {
		th := test.Setup(t)

		dagFile := testDAGFile("restart.yaml")

		// Start the DAG.
		go func() {
			testRunCommand(
				t,
				th.Context,
				startCmd(),
				cmdTest{args: []string{"start", `--params="foo"`, dagFile}},
			)
		}()

		time.Sleep(waitForStatusUpdate)
		cli := th.Client()

		// Wait for the DAG running.
		testStatusEventual(t, cli, dagFile, scheduler.StatusRunning)

		// Restart the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, th.Context, restartCmd(), cmdTest{args: []string{"restart", dagFile}})
			close(done)
		}()

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG running again.
		testStatusEventual(t, cli, dagFile, scheduler.StatusRunning)

		// Stop the restarted DAG.
		testRunCommand(t, th.Context, stopCmd(), cmdTest{args: []string{"stop", dagFile}})

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG is stopped.
		testStatusEventual(t, cli, dagFile, scheduler.StatusNone)

		// Check parameter was the same as the first execution
		dag, err := digraph.Load(th.Context, th.Config.BaseConfig, dagFile, "")
		require.NoError(t, err)

		dataStore := newDataStores(th.Config)
		client := newClient(th.Config, dataStore)
		recentHistory := client.GetRecentHistory(context.Background(), dag, 2)

		require.Len(t, recentHistory, 2)
		require.Equal(t, recentHistory[0].Status.Params, recentHistory[1].Status.Params)

		<-done
	})
}
