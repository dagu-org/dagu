// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

func TestStatusCommand(t *testing.T) {
	t.Run("StatusDAG", func(t *testing.T) {
		th := testSetup(t)

		dagFile := th.DAGFile("long.yaml")

		done := make(chan struct{})
		go func() {
			// Start a DAG to check the status.
			args := []string{"start", dagFile}
			th.RunCommand(t, startCmd(), cmdTest{args: args})
			close(done)
		}()

		hs := th.DataStore().HistoryStore()
		require.Eventually(t, func() bool {
			status := hs.ReadStatusRecent(th.Context, dagFile, 1)
			if len(status) < 1 {
				return false
			}
			println(status[0].Status.Status.String())
			return scheduler.StatusRunning == status[0].Status.Status
		}, waitForStatusTimeout, tick)

		// Check the current status.
		th.RunCommand(t, statusCmd(), cmdTest{
			args:        []string{"status", dagFile},
			expectedOut: []string{"status=running"},
		})

		// Stop the DAG.
		args := []string{"stop", dagFile}
		th.RunCommand(t, stopCmd(), cmdTest{args: args})
		<-done
	})
}
