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

package scheduler

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/logger"
	dsclient "github.com/dagu-org/dagu/internal/persistence/client"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/dagu-org/dagu/internal/util"

	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/config"
)

func TestReadEntries(t *testing.T) {
	t.Run("ReadEntries", func(t *testing.T) {
		tmpDir, cli := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC).Add(-time.Second)
		entryReader := newEntryReader(newEntryReaderArgs{
			DagsDir:    filepath.Join(testdataDir, "invalid_directory"),
			JobCreator: &mockJobFactory{},
			Logger:     test.NewLogger(),
			Client:     cli,
		})

		entries, err := entryReader.Read(now)
		require.NoError(t, err)
		require.Len(t, entries, 0)

		entryReader = newEntryReader(newEntryReaderArgs{
			DagsDir:    testdataDir,
			JobCreator: &mockJobFactory{},
			Logger:     test.NewLogger(),
			Client:     cli,
		})

		done := make(chan any)
		defer close(done)
		entryReader.Start(done)

		entries, err = entryReader.Read(now)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(entries), 1)

		next := entries[0].Next
		require.Equal(t, now.Add(time.Second), next)

		// suspend
		var j job
		for _, e := range entries {
			jj := e.Job
			if jj.GetDAG().Name == "scheduled_job" {
				j = jj
				break
			}
		}

		err = cli.ToggleSuspend(j.GetDAG().Name, true)
		require.NoError(t, err)

		// check if the job is suspended
		lives, err := entryReader.Read(now)
		require.NoError(t, err)
		require.Equal(t, len(entries)-1, len(lives))
	})
}

var testdataDir = filepath.Join(util.MustGetwd(), "testdata")

func setupTest(t *testing.T) (string, client.Client) {
	t.Helper()

	tmpDir := util.MustTempDir("dagu_test")

	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	cfg := &config.Config{
		DataDir:         filepath.Join(tmpDir, ".dagu", "data"),
		DAGs:            testdataDir,
		SuspendFlagsDir: tmpDir,
		WorkDir:         tmpDir,
	}

	dataStore := dsclient.NewDataStores(
		cfg.DAGs,
		cfg.DataDir,
		cfg.QueueDir,
		cfg.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: cfg.LatestStatusToday,
		},
	)

	return tmpDir, client.New(dataStore, "", cfg.WorkDir, logger.Default)
}
