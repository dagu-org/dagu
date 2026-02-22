package fileagentsoul

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CreateAndGetByID(t *testing.T) {
	dir := t.TempDir()
	store, err := New(context.Background(), dir)
	require.NoError(t, err)

	soul := &agent.Soul{
		ID:          "test-soul",
		Name:        "Test Soul",
		Description: "A test soul",
		Content:     "# Identity\n\nYou are a test soul.",
	}

	err = store.Create(context.Background(), soul)
	require.NoError(t, err)

	got, err := store.GetByID(context.Background(), "test-soul")
	require.NoError(t, err)
	assert.Equal(t, soul.ID, got.ID)
	assert.Equal(t, soul.Name, got.Name)
	assert.Equal(t, soul.Description, got.Description)
	assert.Equal(t, soul.Content, got.Content)
}

func TestStore_CreateDuplicate(t *testing.T) {
	dir := t.TempDir()
	store, err := New(context.Background(), dir)
	require.NoError(t, err)

	soul := &agent.Soul{
		ID:      "test-soul",
		Name:    "Test Soul",
		Content: "content",
	}
	err = store.Create(context.Background(), soul)
	require.NoError(t, err)

	err = store.Create(context.Background(), soul)
	assert.ErrorIs(t, err, agent.ErrSoulAlreadyExists)
}

func TestStore_List(t *testing.T) {
	dir := t.TempDir()
	store, err := New(context.Background(), dir)
	require.NoError(t, err)

	for _, s := range []*agent.Soul{
		{ID: "b-soul", Name: "Beta", Content: "b"},
		{ID: "a-soul", Name: "Alpha", Content: "a"},
	} {
		require.NoError(t, store.Create(context.Background(), s))
	}

	souls, err := store.List(context.Background())
	require.NoError(t, err)
	require.Len(t, souls, 2)
	assert.Equal(t, "Alpha", souls[0].Name)
	assert.Equal(t, "Beta", souls[1].Name)
}

func TestStore_Update(t *testing.T) {
	dir := t.TempDir()
	store, err := New(context.Background(), dir)
	require.NoError(t, err)

	soul := &agent.Soul{
		ID:      "test-soul",
		Name:    "Original",
		Content: "original content",
	}
	require.NoError(t, store.Create(context.Background(), soul))

	soul.Name = "Updated"
	soul.Content = "updated content"
	require.NoError(t, store.Update(context.Background(), soul))

	got, err := store.GetByID(context.Background(), "test-soul")
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
	assert.Equal(t, "updated content", got.Content)
}

func TestStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := New(context.Background(), dir)
	require.NoError(t, err)

	soul := &agent.Soul{
		ID:      "test-soul",
		Name:    "Test",
		Content: "content",
	}
	require.NoError(t, store.Create(context.Background(), soul))

	err = store.Delete(context.Background(), "test-soul")
	require.NoError(t, err)

	_, err = store.GetByID(context.Background(), "test-soul")
	assert.ErrorIs(t, err, agent.ErrSoulNotFound)
}

func TestStore_Search(t *testing.T) {
	dir := t.TempDir()
	store, err := New(context.Background(), dir)
	require.NoError(t, err)

	for _, s := range []*agent.Soul{
		{ID: "ops-soul", Name: "Ops Assistant", Content: "ops"},
		{ID: "dev-soul", Name: "Dev Assistant", Content: "dev"},
	} {
		require.NoError(t, store.Create(context.Background(), s))
	}

	result, err := store.Search(context.Background(), agent.SearchSoulsOptions{
		Query: "ops",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "ops-soul", result.Items[0].ID)
}

func TestStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := New(context.Background(), dir)
	require.NoError(t, err)

	_, err = store.GetByID(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, agent.ErrSoulNotFound)
}

func TestStore_InvalidID(t *testing.T) {
	dir := t.TempDir()
	store, err := New(context.Background(), dir)
	require.NoError(t, err)

	soul := &agent.Soul{
		ID:      "INVALID",
		Name:    "Test",
		Content: "content",
	}
	err = store.Create(context.Background(), soul)
	assert.ErrorIs(t, err, agent.ErrInvalidSoulID)
}

func TestSeedExampleSouls(t *testing.T) {
	dir := t.TempDir()
	soulsDir := filepath.Join(dir, "souls")

	created := SeedExampleSouls(context.Background(), soulsDir)
	assert.True(t, created)

	// Verify marker file exists
	_, err := os.Stat(filepath.Join(soulsDir, examplesMarkerFile))
	assert.NoError(t, err)

	// Verify default soul exists
	_, err = os.Stat(filepath.Join(soulsDir, "default.md"))
	assert.NoError(t, err)

	// Second call should not re-seed
	created = SeedExampleSouls(context.Background(), soulsDir)
	assert.False(t, created)
}

func TestParseSoulFile(t *testing.T) {
	data := []byte(`---
name: Test Soul
description: A test
---
# Identity

You are a test soul.
`)

	soul, err := parseSoulFile(data, "test")
	require.NoError(t, err)
	assert.Equal(t, "test", soul.ID)
	assert.Equal(t, "Test Soul", soul.Name)
	assert.Equal(t, "A test", soul.Description)
	assert.Equal(t, "# Identity\n\nYou are a test soul.", soul.Content)
}
