package filewatermark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_LoadMissing(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	state, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, state.Version)
	assert.Empty(t, state.DAGs)
	assert.True(t, state.LastTick.IsZero())
}

func TestStore_SaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	state := &exec.SchedulerState{
		Version:  1,
		LastTick: now,
		DAGs: map[string]exec.DAGWatermark{
			"hourly-etl": {LastScheduledTime: now.Add(-time.Hour)},
			"daily-report": {LastScheduledTime: now.Add(-24 * time.Hour)},
		},
	}

	err := store.Save(ctx, state)
	require.NoError(t, err)

	loaded, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.Version)
	assert.Equal(t, now.UTC(), loaded.LastTick.UTC())
	assert.Len(t, loaded.DAGs, 2)
	assert.Equal(t, now.Add(-time.Hour).UTC(), loaded.DAGs["hourly-etl"].LastScheduledTime.UTC())
	assert.Equal(t, now.Add(-24*time.Hour).UTC(), loaded.DAGs["daily-report"].LastScheduledTime.UTC())
}

func TestStore_LoadCorrupt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)

	// Write corrupt data
	err := os.MkdirAll(dir, 0o750)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, stateFileName), []byte("not json"), 0o600)
	require.NoError(t, err)

	state, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, state.Version)
	assert.Empty(t, state.DAGs)
}

func TestStore_SaveCreatesDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nested", "scheduler")
	store := New(dir)

	state := &exec.SchedulerState{
		Version: 1,
		DAGs:    make(map[string]exec.DAGWatermark),
	}

	err := store.Save(context.Background(), state)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filepath.Join(dir, stateFileName))
	require.NoError(t, err)
}

func TestStore_SaveAtomicity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)
	ctx := context.Background()

	// Save initial state
	state1 := &exec.SchedulerState{
		Version:  1,
		LastTick: time.Now().Truncate(time.Second),
		DAGs: map[string]exec.DAGWatermark{
			"dag1": {LastScheduledTime: time.Now().Truncate(time.Second)},
		},
	}
	require.NoError(t, store.Save(ctx, state1))

	// Save updated state
	state2 := &exec.SchedulerState{
		Version:  1,
		LastTick: time.Now().Add(time.Minute).Truncate(time.Second),
		DAGs: map[string]exec.DAGWatermark{
			"dag1": {LastScheduledTime: time.Now().Add(time.Minute).Truncate(time.Second)},
			"dag2": {LastScheduledTime: time.Now().Add(time.Minute).Truncate(time.Second)},
		},
	}
	require.NoError(t, store.Save(ctx, state2))

	// Verify updated state
	loaded, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Len(t, loaded.DAGs, 2)

	// Verify no temp file left behind
	_, err = os.Stat(filepath.Join(dir, stateFileName+".tmp"))
	assert.True(t, os.IsNotExist(err))
}
