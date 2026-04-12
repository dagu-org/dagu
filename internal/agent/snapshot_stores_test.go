// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSnapshotStores_HydratesReadOnlyStores(t *testing.T) {
	t.Parallel()

	stores := NewSnapshotStores(&Snapshot{
		Config: &Config{
			Enabled:        true,
			DefaultModelID: "model-default",
		},
		Models: []*ModelConfig{
			testModelConfig("model-default"),
		},
		Memory: &MemorySnapshot{
			Global: "global memory",
			PerDAG: map[string]string{
				"b-dag": "B",
				"a-dag": "A",
			},
		},
	})

	require.NotNil(t, stores.ConfigStore)
	require.NotNil(t, stores.ModelStore)
	require.NotNil(t, stores.MemoryStore)

	cfg, err := stores.ConfigStore.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "model-default", cfg.DefaultModelID)
	assert.True(t, stores.ConfigStore.IsEnabled(context.Background()))
	assert.ErrorIs(t, stores.ConfigStore.Save(context.Background(), cfg), ErrSnapshotStoreReadOnly)

	model, err := stores.ModelStore.GetByID(context.Background(), "model-default")
	require.NoError(t, err)
	assert.Equal(t, "model-default", model.ID)
	_, err = stores.ModelStore.GetByID(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrModelNotFound)
	assert.ErrorIs(t, stores.ModelStore.Create(context.Background(), testModelConfig("other")), ErrSnapshotStoreReadOnly)
	assert.ErrorIs(t, stores.ModelStore.Delete(context.Background(), "model-default"), ErrSnapshotStoreReadOnly)

	global, err := stores.MemoryStore.LoadGlobalMemory(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "global memory", global)
	dagMemory, err := stores.MemoryStore.LoadDAGMemory(context.Background(), "a-dag")
	require.NoError(t, err)
	assert.Equal(t, "A", dagMemory)
	names, err := stores.MemoryStore.ListDAGMemories(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"a-dag", "b-dag"}, names)
	assert.Equal(t, "", stores.MemoryStore.MemoryDir())
	assert.ErrorIs(t, stores.MemoryStore.SaveGlobalMemory(context.Background(), "new"), ErrSnapshotStoreReadOnly)
	assert.ErrorIs(t, stores.MemoryStore.DeleteDAGMemory(context.Background(), "a-dag"), ErrSnapshotStoreReadOnly)

	readOnlyStore, ok := stores.MemoryStore.(SnapshotReadOnlyMemoryStore)
	require.True(t, ok)
	assert.True(t, readOnlyStore.MemoryReadOnly())
}

func TestSnapshotSoulStore_SearchAndReadOnly(t *testing.T) {
	t.Parallel()

	store := NewSnapshotSoulStore([]*Soul{
		{ID: "advisor", Name: "Advisor", Description: "Strategic guidance"},
		{ID: "builder", Name: "Builder", Description: "Build systems carefully"},
	})
	require.NotNil(t, store)

	result, err := store.Search(context.Background(), SearchSoulsOptions{
		Query:     "build",
		Paginator: exec.NewPaginator(1, 10),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "builder", result.Items[0].ID)

	soul, err := store.GetByID(context.Background(), "advisor")
	require.NoError(t, err)
	assert.Equal(t, "Advisor", soul.Name)
	_, err = store.GetByID(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrSoulNotFound)
	assert.ErrorIs(t, store.Delete(context.Background(), "advisor"), ErrSnapshotStoreReadOnly)
}
