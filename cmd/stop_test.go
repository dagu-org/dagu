// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
)

func TestStopCommand(t *testing.T) {
	t.Run("StopDAG", func(t *testing.T) {
		setup := test.SetupTest(t)

		dagFile := testDAGFile("long2.yaml")

		// Start the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, startCmd(), cmdTest{args: []string{"start", dagFile}})
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)

		// Wait for the DAG running.
		testLastStatusEventual(
			t,
			setup.DataStore().HistoryStore(),
			dagFile,
			scheduler.StatusRunning,
		)

		// Stop the DAG.
		testRunCommand(t, stopCmd(), cmdTest{
			args:        []string{"stop", dagFile},
			expectedOut: []string{"Stopping..."}})

		// Check the last execution is cancelled.
		testLastStatusEventual(
			t, setup.DataStore().HistoryStore(), dagFile, scheduler.StatusCancel,
		)
		<-done
	})
}
