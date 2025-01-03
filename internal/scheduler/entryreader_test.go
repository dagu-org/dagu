package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/persistence/jsondb"
	"github.com/dagu-org/dagu/internal/persistence/local"
	"github.com/dagu-org/dagu/internal/persistence/local/storage"

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
		entryReader := newEntryReader(
			filepath.Join(testdataDir, "invalid_directory"),
			&mockJobFactory{},
			cli,
		)

		entries, err := entryReader.Read(context.Background(), now)
		require.NoError(t, err)
		require.Len(t, entries, 0)

		entryReader = newEntryReader(
			testdataDir,
			&mockJobFactory{},
			cli,
		)

		done := make(chan any)
		defer close(done)
		err = entryReader.Start(context.Background(), done)
		require.NoError(t, err)

		entries, err = entryReader.Read(context.Background(), now)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(entries), 1)

		next := entries[0].Next
		require.Equal(t, now.Add(time.Second), next)

		// suspend
		var j job
		for _, e := range entries {
			jj := e.Job
			if jj.GetDAG(context.Background()).Name == "scheduled_job" {
				j = jj
				break
			}
		}

		err = cli.ToggleSuspend(context.Background(), j.GetDAG(context.Background()).Name, true)
		require.NoError(t, err)

		// check if the job is suspended
		lives, err := entryReader.Read(context.Background(), now)
		require.NoError(t, err)
		require.Equal(t, len(entries)-1, len(lives))
	})
}

var testdataDir = filepath.Join(fileutil.MustGetwd(), "testdata")

func setupTest(t *testing.T) (string, client.Client) {
	t.Helper()

	tmpDir := fileutil.MustTempDir("test")

	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	cfg := &config.Config{
		Paths: config.PathsConfig{
			DataDir:         filepath.Join(tmpDir, "."+build.Slug, "data"),
			DAGsDir:         testdataDir,
			SuspendFlagsDir: tmpDir,
		},
		WorkDir: tmpDir,
	}

	dagStore := local.NewDAGStore(cfg.Paths.DAGsDir)
	historyStore := jsondb.New(cfg.Paths.DataDir)
	flagStore := local.NewFlagStore(
		storage.NewStorage(cfg.Paths.SuspendFlagsDir),
	)

	return tmpDir, client.New(dagStore, historyStore, flagStore, "", cfg.WorkDir)
}
