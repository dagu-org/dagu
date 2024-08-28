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
	"testing"

	"github.com/daguflow/dagu/internal/dag/scheduler"
	"github.com/daguflow/dagu/internal/test"
)

func TestStatusCommand(t *testing.T) {
	t.Run("StatusDAG", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		dagFile := testDAGFile("long.yaml")

		// Start the DAG.
		done := make(chan struct{})
		go func() {
			testRunCommand(t, startCmd(), cmdTest{args: []string{"start", dagFile}})
			close(done)
		}()

		testLastStatusEventual(
			t,
			setup.DataStore().HistoryStore(),
			dagFile,
			scheduler.StatusRunning,
		)

		// Check the current status.
		testRunCommand(t, statusCmd(), cmdTest{
			args:        []string{"status", dagFile},
			expectedOut: []string{"Status=running"},
		})

		// Stop the DAG.
		testRunCommand(t, stopCmd(), cmdTest{args: []string{"stop", dagFile}})
		<-done
	})
}
