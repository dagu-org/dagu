// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filewatermark

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/service/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_LoadMissing(t *testing.T) {
	t.Parallel()

	store := New(t.TempDir())
	state, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, scheduler.SchedulerStateVersion, state.Version)
	assert.Empty(t, state.DAGs)
	assert.True(t, state.LastTick.IsZero())
}

func TestStore_SaveAndLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	state := &scheduler.SchedulerState{
		Version:  scheduler.SchedulerStateVersion,
		LastTick: now,
		DAGs: map[string]scheduler.DAGWatermark{
			"hourly-etl":   {LastScheduledTime: now.Add(-time.Hour)},
			"daily-report": {LastScheduledTime: now.Add(-24 * time.Hour)},
		},
	}

	err := store.Save(ctx, state)
	require.NoError(t, err)

	loaded, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, scheduler.SchedulerStateVersion, loaded.Version)
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
	assert.Equal(t, scheduler.SchedulerStateVersion, state.Version)
	assert.Empty(t, state.DAGs)
}

func TestStore_SaveCreatesDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nested", "scheduler")
	store := New(dir)

	state := &scheduler.SchedulerState{
		Version: scheduler.SchedulerStateVersion,
		DAGs:    make(map[string]scheduler.DAGWatermark),
	}

	err := store.Save(context.Background(), state)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filepath.Join(dir, stateFileName))
	require.NoError(t, err)
}

func TestStore_LoadVersionMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)

	// Write state with a future version
	data := []byte(`{"version":99,"lastTick":"2020-01-01T00:00:00Z","dags":{}}`)
	err := os.MkdirAll(dir, 0o750)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, stateFileName), data, 0o600)
	require.NoError(t, err)

	state, err := store.Load(context.Background())
	require.NoError(t, err)
	// Should return fresh state due to version mismatch
	assert.Equal(t, scheduler.SchedulerStateVersion, state.Version)
	assert.Empty(t, state.DAGs)
	assert.True(t, state.LastTick.IsZero())
}

func TestStore_SaveAtomicity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	// Save initial state
	state1 := &scheduler.SchedulerState{
		Version:  scheduler.SchedulerStateVersion,
		LastTick: now,
		DAGs: map[string]scheduler.DAGWatermark{
			"dag1": {LastScheduledTime: now},
		},
	}
	require.NoError(t, store.Save(ctx, state1))

	// Save updated state
	later := now.Add(time.Minute)
	state2 := &scheduler.SchedulerState{
		Version:  scheduler.SchedulerStateVersion,
		LastTick: later,
		DAGs: map[string]scheduler.DAGWatermark{
			"dag1": {LastScheduledTime: later},
			"dag2": {LastScheduledTime: later},
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

func TestStore_LoadMigratesVersionOneState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)

	data := []byte(`{"version":1,"lastTick":"2026-02-07T12:00:00Z","dags":{"legacy":{"lastScheduledTime":"2026-02-07T11:00:00Z"}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, stateFileName), data, 0o600))

	state, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, scheduler.SchedulerStateVersion, state.Version)
	assert.Equal(t, time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC), state.DAGs["legacy"].LastScheduledTime)
}

func TestStore_LoadMigratesVersionTwoState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)

	data := []byte(`{"version":2,"lastTick":"2026-02-07T12:00:00Z","dags":{"legacy":{"lastScheduledTime":"2026-02-07T11:00:00Z"}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, stateFileName), data, 0o600))

	state, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, scheduler.SchedulerStateVersion, state.Version)
	assert.Equal(t, time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC), state.DAGs["legacy"].LastScheduledTime)
}

func TestStore_LoadMigratesVersionZeroState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := New(dir)

	data := []byte(`{"lastTick":"2026-02-07T12:00:00Z","dags":{"legacy":{"lastScheduledTime":"2026-02-07T11:00:00Z"}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, stateFileName), data, 0o600))

	state, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, scheduler.SchedulerStateVersion, state.Version)
	assert.Equal(t, time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC), state.DAGs["legacy"].LastScheduledTime)
}
