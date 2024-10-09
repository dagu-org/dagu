// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

const (
	waitForStatusUpdate = time.Millisecond * 100
)

func TestRestartCommand(t *testing.T) {
	t.Run("RestartDAG", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		dagFile := testDAGFile("restart.yaml")
		ctx := context.Background()

		// Start the DAG.
		go func() {
			testRunCommand(
				t,
				startCmd(),
				cmdTest{args: []string{"start", `--params="foo"`, dagFile}},
			)
		}()

		time.Sleep(waitForStatusUpdate)
		cli := setup.Client()

		// Wait for the DAG running.
		testStatusEventual(t, cli, dagFile, scheduler.StatusRunning)

		// Restart the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, restartCmd(), cmdTest{args: []string{"restart", dagFile}})
			close(done)
		}()

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG running again.
		testStatusEventual(t, cli, dagFile, scheduler.StatusRunning)

		// Stop the restarted DAG.
		testRunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dagFile}})

		time.Sleep(waitForStatusUpdate)

		// Wait for the DAG is stopped.
		testStatusEventual(t, cli, dagFile, scheduler.StatusNone)

		// Check parameter was the same as the first execution
		dAG, err := dag.Load(setup.Config.BaseConfig, dagFile, "")
		require.NoError(t, err)

		dataStore := newDataStores(setup.Config, logger.Default)
		recentHistory := newClient(
			setup.Config,
			dataStore,
			logger.Default,
		).ListRecentHistory(ctx, dAG, 2)

		require.Len(t, recentHistory, 2)
		require.Equal(t, recentHistory[0].Status.Params, recentHistory[1].Status.Params)

		<-done
	})
}
