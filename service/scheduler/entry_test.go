package scheduler

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/storage"
	"github.com/dagu-dev/dagu/internal/suspend"
)

func TestReadEntries(t *testing.T) {
	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC).Add(-time.Second)

	r := NewEntryReader(path.Join(testdataDir, "invalid_directory"), &config.Config{
		DAGs: path.Join(testdataDir, "invalid_directory"),
	})
	entries, err := r.Read(now)
	require.NoError(t, err)
	require.Len(t, entries, 0)

	r = NewEntryReader(testdataDir, &config.Config{DAGs: testdataDir})

	entries, err = r.Read(now)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(entries), 1)

	next := entries[0].Next
	require.Equal(t, now.Add(time.Second), next)

	// suspend
	var j *job
	for _, e := range entries {
		jj := e.Job.(*job)
		if jj.DAG.Name == "scheduled_job" {
			j = jj
			break
		}
	}
	sc := suspend.NewSuspendChecker(storage.NewStorage(config.Get().SuspendFlagsDir))
	err = sc.ToggleSuspend(j.DAG, true)
	require.NoError(t, err)

	// check if the job is suspended
	lives, err := r.Read(now)
	require.NoError(t, err)
	require.Equal(t, len(entries)-1, len(lives))
}
