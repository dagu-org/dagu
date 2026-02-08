package scheduler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGStateStore_LoadSave(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	require.NoError(t, os.MkdirAll(dagsDir, 0o750))

	store := NewDAGStateStore(tmpDir, dagsDir)

	dag := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
	}

	// Save and reload
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	require.NoError(t, store.Save(dag, dagState{LastTick: now}))

	state, err := store.Load(dag)
	require.NoError(t, err)
	assert.True(t, now.Equal(state.LastTick))

	// Overwrite with new value
	later := now.Add(time.Hour)
	require.NoError(t, store.Save(dag, dagState{LastTick: later}))

	state, err = store.Load(dag)
	require.NoError(t, err)
	assert.True(t, later.Equal(state.LastTick))
}

func TestDAGStateStore_LoadMissing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewDAGStateStore(tmpDir, tmpDir)

	dag := &core.DAG{
		Name:     "nonexistent",
		Location: filepath.Join(tmpDir, "nonexistent.yaml"),
	}

	state, err := store.Load(dag)
	require.NoError(t, err)
	assert.True(t, state.LastTick.IsZero())
}

func TestDAGStateStore_CorruptFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewDAGStateStore(tmpDir, tmpDir)

	dag := &core.DAG{
		Name:     "corrupt",
		Location: filepath.Join(tmpDir, "corrupt.yaml"),
	}

	// Write corrupt data
	dir := filepath.Join(tmpDir, "scheduler", "dag-state")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{invalid"), 0o644))

	state, err := store.Load(dag)
	require.NoError(t, err)
	assert.True(t, state.LastTick.IsZero())
}

func TestDAGStateStore_SaveAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	require.NoError(t, os.MkdirAll(dagsDir, 0o750))

	store := NewDAGStateStore(tmpDir, dagsDir)

	dag1 := &core.DAG{Name: "dag1", Location: filepath.Join(dagsDir, "dag1.yaml")}
	dag2 := &core.DAG{Name: "dag2", Location: filepath.Join(dagsDir, "dag2.yaml")}

	dags := map[string]*core.DAG{
		"dag1.yaml": dag1,
		"dag2.yaml": dag2,
	}

	tick := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	require.NoError(t, store.SaveAll(dags, tick))

	// Verify both DAGs have the same tick
	state1, err := store.Load(dag1)
	require.NoError(t, err)
	assert.True(t, tick.Equal(state1.LastTick))

	state2, err := store.Load(dag2)
	require.NoError(t, err)
	assert.True(t, tick.Equal(state2.LastTick))
}

func TestDAGStateStore_LoadAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	require.NoError(t, os.MkdirAll(dagsDir, 0o750))

	store := NewDAGStateStore(tmpDir, dagsDir)

	dag1 := &core.DAG{Name: "dag1", Location: filepath.Join(dagsDir, "dag1.yaml")}
	dag2 := &core.DAG{Name: "dag2", Location: filepath.Join(dagsDir, "dag2.yaml")}

	dags := map[string]*core.DAG{
		"dag1.yaml": dag1,
		"dag2.yaml": dag2,
	}

	tick1 := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	tick2 := time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)

	require.NoError(t, store.Save(dag1, dagState{LastTick: tick1}))
	require.NoError(t, store.Save(dag2, dagState{LastTick: tick2}))

	states, err := store.LoadAll(dags)
	require.NoError(t, err)
	assert.Len(t, states, 2)
	assert.True(t, tick1.Equal(states[dag1].LastTick))
	assert.True(t, tick2.Equal(states[dag2].LastTick))
}

func TestDAGStateStore_Migrate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	require.NoError(t, os.MkdirAll(dagsDir, 0o750))

	store := NewDAGStateStore(tmpDir, dagsDir)

	// Create old global watermark file
	oldPath := filepath.Join(tmpDir, "scheduler", "state.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(oldPath), 0o750))

	globalTick := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	oldData, err := json.Marshal(struct {
		LastTick time.Time `json:"lastTick"`
	}{LastTick: globalTick})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(oldPath, oldData, 0o600))

	dag1 := &core.DAG{Name: "dag1", Location: filepath.Join(dagsDir, "dag1.yaml")}
	dag2 := &core.DAG{Name: "dag2", Location: filepath.Join(dagsDir, "dag2.yaml")}

	dags := map[string]*core.DAG{
		"dag1.yaml": dag1,
		"dag2.yaml": dag2,
	}

	require.NoError(t, store.Migrate(oldPath, dags))

	// All DAGs should have the global tick
	state1, err := store.Load(dag1)
	require.NoError(t, err)
	assert.True(t, globalTick.Equal(state1.LastTick))

	state2, err := store.Load(dag2)
	require.NoError(t, err)
	assert.True(t, globalTick.Equal(state2.LastTick))

	// Old file should be removed
	_, err = os.Stat(oldPath)
	assert.True(t, os.IsNotExist(err))
}

func TestDAGStateStore_MigrateNoOldFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := NewDAGStateStore(tmpDir, tmpDir)

	oldPath := filepath.Join(tmpDir, "scheduler", "state.json")

	dags := map[string]*core.DAG{
		"test.yaml": {Name: "test", Location: filepath.Join(tmpDir, "test.yaml")},
	}

	// Should be a no-op
	require.NoError(t, store.Migrate(oldPath, dags))
}

func TestDAGStateStore_NamespacedDAG(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	require.NoError(t, os.MkdirAll(filepath.Join(dagsDir, "team-a"), 0o750))

	store := NewDAGStateStore(tmpDir, dagsDir)

	// Flat DAG
	flatDAG := &core.DAG{Name: "etl", Location: filepath.Join(dagsDir, "etl.yaml")}
	// Namespaced DAG
	nsDAG := &core.DAG{Name: "etl", Location: filepath.Join(dagsDir, "team-a", "etl.yaml")}

	tick := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	// Both should be saveable without collision
	require.NoError(t, store.Save(flatDAG, dagState{LastTick: tick}))
	require.NoError(t, store.Save(nsDAG, dagState{LastTick: tick.Add(time.Hour)}))

	state1, err := store.Load(flatDAG)
	require.NoError(t, err)
	assert.True(t, tick.Equal(state1.LastTick))

	state2, err := store.Load(nsDAG)
	require.NoError(t, err)
	assert.True(t, tick.Add(time.Hour).Equal(state2.LastTick))
}
