// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
)

func TestStatusCommand(t *testing.T) {
	t.Run("StatusDAG", func(t *testing.T) {
		th := test.Setup(t)

		dagFile := testDAGFile("long.yaml")

		// Start the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, th.Context, startCmd(), cmdTest{args: []string{"start", dagFile}})
			close(done)
		}()

		testLastStatusEventual(
			t,
			th.DataStore().HistoryStore(),
			dagFile,
			scheduler.StatusRunning,
		)

		// Check the current status.
		testRunCommand(t, th.Context, statusCmd(), cmdTest{
			args:        []string{"status", dagFile},
			expectedOut: []string{"Status=running"},
		})

		// Stop the DAG.
		testRunCommand(t, th.Context, stopCmd(), cmdTest{args: []string{"stop", dagFile}})
		<-done
	})
}
