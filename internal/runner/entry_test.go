package runner

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yohamta/dagu/internal/admin"
)

func TestReadEntries(t *testing.T) {
	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC).Add(-time.Second)

	r := &entryReader{Admin: &admin.Config{
		DAGs: path.Join(testsDir, "runner/invalid"),
	}}
	_, err := r.Read(now)
	require.Error(t, err)

	r = &entryReader{Admin: &admin.Config{
		DAGs: path.Join(testsDir, "runner"),
	}}
	entries, err := r.Read(now)
	require.NoError(t, err)

	require.Len(t, entries, 1)

	j := entries[0].Job.(*job)
	require.Equal(t, "scheduled_job", j.DAG.Name)

	next := entries[0].Next
	require.Equal(t, now.Add(time.Second), next)
}
