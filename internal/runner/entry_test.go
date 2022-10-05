package runner

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/suspend"
)

func TestReadEntries(t *testing.T) {
	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC).Add(-time.Second)

	r := newEntryReader(&admin.Config{
		DAGs: path.Join(testdataDir, "invalid_directory"),
	})
	entries, err := r.Read(now)
	require.NoError(t, err)
	require.Len(t, entries, 0)

	r = newEntryReader(&admin.Config{
		DAGs: testdataDir,
	})

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
	sc := suspend.NewSuspendChecker(
		storage.NewStorage(settings.MustGet(
			settings.SETTING__SUSPEND_FLAGS_DIR,
		)))
	err = sc.ToggleSuspend(j.DAG, true)
	require.NoError(t, err)

	// check if the job is suspended
	lives, err := r.Read(now)
	require.NoError(t, err)
	require.Equal(t, len(entries)-1, len(lives))
}
