package scheduler

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatermarkStore_LoadSave(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewWatermarkStore(tmpDir)

	// Load from non-existent file returns zero time
	ts, err := store.Load()
	require.NoError(t, err)
	assert.True(t, ts.IsZero())

	// Save and reload
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	require.NoError(t, store.Save(now))

	ts, err = store.Load()
	require.NoError(t, err)
	assert.True(t, now.Equal(ts))

	// Overwrite with new value
	later := now.Add(time.Hour)
	require.NoError(t, store.Save(later))

	ts, err = store.Load()
	require.NoError(t, err)
	assert.True(t, later.Equal(ts))
}

func TestWatermarkStore_CorruptFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewWatermarkStore(tmpDir)

	// Write corrupt data
	dir := filepath.Join(tmpDir, "scheduler")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "state.json"), []byte("{invalid"), 0o644))

	// Should return zero time (not error)
	ts, err := store.Load()
	require.NoError(t, err)
	assert.True(t, ts.IsZero())
}
