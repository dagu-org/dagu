// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
)

func TestStopCommand(t *testing.T) {
	t.Run("StopDAG", func(t *testing.T) {
		th := testSetup(t)

		dagFile := th.DAGFile("long2.yaml")

		done := make(chan struct{})
		go func() {
			// Start the DAG to stop.
			args := []string{"start", dagFile}
			th.RunCommand(t, startCmd(), cmdTest{args: args})
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)

		// Wait for the DAG running.
		testLastStatusEventual(
			t,
			th.DataStore().HistoryStore(),
			dagFile,
			scheduler.StatusRunning,
		)

		// Stop the DAG.
		th.RunCommand(t, stopCmd(), cmdTest{
			args:        []string{"stop", dagFile},
			expectedOut: []string{"DAG stopped"}})

		// Check the DAG is stopped.
		testLastStatusEventual(
			t, th.DataStore().HistoryStore(), dagFile, scheduler.StatusCancel,
		)
		<-done
	})
}
