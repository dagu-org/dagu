// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

const (
	waitForStatusUpdate = time.Millisecond * 100
)

func TestRestartCommand(t *testing.T) {
	t.Run("RestartDAG", func(t *testing.T) {
		th := testSetup(t)
		dagFile := th.DAGFile("restart.yaml")

		go func() {
			// Start a DAG to restart.
			args := []string{"start", `--params="foo"`, dagFile.Path}
			th.RunCommand(t, startCmd(), cmdTest{args: args})
		}()

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG running.
		dagFile.AssertCurrentStatus(t, scheduler.StatusRunning)

		// Restart the DAG.
		done := make(chan struct{})
		go func() {
			args := []string{"restart", dagFile.Path}
			th.RunCommand(t, restartCmd(), cmdTest{args: args})
			close(done)
		}()

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG running again.
		dagFile.AssertCurrentStatus(t, scheduler.StatusRunning)

		// Stop the restarted DAG.
		th.RunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dagFile.Path}})

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG is stopped.
		dagFile.AssertCurrentStatus(t, scheduler.StatusNone)

		// Check parameter was the same as the first execution
		dag, err := digraph.Load(th.Context, th.Config.Paths.BaseConfig, dagFile.Path, "")
		require.NoError(t, err)

		dataStore := newDataStores(th.Config)
		client := newClient(th.Config, dataStore)
		recentHistory := client.GetRecentHistory(context.Background(), dag, 2)

		require.Len(t, recentHistory, 2)
		require.Equal(t, recentHistory[0].Status.Params, recentHistory[1].Status.Params)

		<-done
	})
}
