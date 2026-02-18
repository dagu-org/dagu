package fileagentmodel

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

func setupTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	baseDir := t.TempDir()
	store, err := New(baseDir)
	require.NoError(t, err)
	return store, baseDir
}

func newTestModel(id, name string) *agent.ModelConfig {
	return &agent.ModelConfig{
		ID:       id,
		Name:     name,
		Provider: "anthropic",
		Model:    "claude-sonnet-4-5",
	}
}

func createModel(t *testing.T, store *Store, model *agent.ModelConfig) {
	t.Helper()
	err := store.Create(context.Background(), model)
	require.NoError(t, err)
}

// Tests for New

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("valid baseDir creates store", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()
		store, err := New(baseDir)
		require.NoError(t, err)
		assert.NotNil(t, store)

		info, err := os.Stat(baseDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("empty baseDir returns error", func(t *testing.T) {
		t.Parallel()
		store, err := New("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "baseDir cannot be empty")
		assert.Nil(t, store)
	})

	t.Run("creates directory if not exists", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		baseDir := filepath.Join(tmpDir, "models")
		store, err := New(baseDir)
		require.NoError(t, err)
		assert.NotNil(t, store)

		info, err := os.Stat(baseDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("directory creation failure returns error", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		blockingFile := filepath.Join(tmpDir, "blocked")
		require.NoError(t, os.WriteFile(blockingFile, []byte("block"), 0600))

		store, err := New(filepath.Join(blockingFile, "subdir"))
		require.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "failed to create directory")
	})
}

// Tests for Create

func TestStore_Create(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("valid model", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		model := newTestModel("claude-sonnet", "Claude Sonnet")

		err := store.Create(ctx, model)
		require.NoError(t, err)

		// Verify file was created
		filePath := filepath.Join(baseDir, "claude-sonnet.json")
		_, err = os.Stat(filePath)
		require.NoError(t, err)

		// Verify can be retrieved
		got, err := store.GetByID(ctx, "claude-sonnet")
		require.NoError(t, err)
		assert.Equal(t, model.ID, got.ID)
		assert.Equal(t, model.Name, got.Name)
		assert.Equal(t, model.Provider, got.Provider)
		assert.Equal(t, model.Model, got.Model)
	})

	t.Run("path traversal ID rejected", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		tests := []struct {
			name string
			id   string
		}{
			{name: "dot-dot-slash", id: "../../etc/passwd"},
			{name: "dot-dot", id: ".."},
			{name: "single-dot", id: "."},
			{name: "slash-prefix", id: "/etc/passwd"},
			{name: "uppercase", id: "InvalidID"},
			{name: "underscore", id: "invalid_id"},
			{name: "space", id: "invalid id"},
			{name: "empty", id: ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				model := newTestModel(tt.id, "Test Model")
				err := store.Create(ctx, model)
				require.Error(t, err)
				assert.ErrorIs(t, err, agent.ErrInvalidModelID)
			})
		}
	})

	t.Run("duplicate ID returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		model := newTestModel("my-model", "My Model")
		createModel(t, store, model)

		duplicate := newTestModel("my-model", "Different Name")
		err := store.Create(ctx, duplicate)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrModelAlreadyExists)
	})

	t.Run("nil model returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		err := store.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model cannot be nil")
	})

	t.Run("empty name returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		model := newTestModel("valid-id", "")

		err := store.Create(ctx, model)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model name is required")
	})
}

// Tests for GetByID

func TestStore_GetByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		model := newTestModel("test-model", "Test Model")
		model.Provider = "openai"
		model.Model = "gpt-4"
		model.APIKey = "sk-test"
		model.ContextWindow = 128000
		model.MaxOutputTokens = 4096
		model.InputCostPer1M = 30.0
		model.OutputCostPer1M = 60.0
		model.SupportsThinking = true
		model.Description = "A test model"
		createModel(t, store, model)

		got, err := store.GetByID(ctx, "test-model")
		require.NoError(t, err)
		assert.Equal(t, model, got)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		got, err := store.GetByID(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrModelNotFound)
		assert.Nil(t, got)
	})

	t.Run("invalid ID path traversal", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		tests := []struct {
			name string
			id   string
		}{
			{name: "dot-dot-slash", id: "../../etc/passwd"},
			{name: "uppercase", id: "InvalidID"},
			{name: "empty", id: ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, err := store.GetByID(ctx, tt.id)
				require.Error(t, err)
				assert.ErrorIs(t, err, agent.ErrInvalidModelID)
				assert.Nil(t, got)
			})
		}
	})
}

// Tests for List

func TestStore_List(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty store returns empty list", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		models, err := store.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, models)
	})

	t.Run("multiple models sorted by name", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		// Create models in non-alphabetical order
		createModel(t, store, newTestModel("model-c", "Zulu Model"))
		createModel(t, store, newTestModel("model-a", "Alpha Model"))
		createModel(t, store, newTestModel("model-b", "Mike Model"))

		models, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, models, 3)

		assert.Equal(t, "Alpha Model", models[0].Name)
		assert.Equal(t, "Mike Model", models[1].Name)
		assert.Equal(t, "Zulu Model", models[2].Name)
	})

	t.Run("single model", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createModel(t, store, newTestModel("only-model", "Only Model"))

		models, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "only-model", models[0].ID)
		assert.Equal(t, "Only Model", models[0].Name)
	})
}

// Tests for Update

func TestStore_Update(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("valid update", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		model := newTestModel("update-me", "Original Name")
		createModel(t, store, model)

		updated := &agent.ModelConfig{
			ID:       "update-me",
			Name:     "Updated Name",
			Provider: "openai",
			Model:    "gpt-4",
		}
		err := store.Update(ctx, updated)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, "update-me")
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", got.Name)
		assert.Equal(t, "openai", got.Provider)
		assert.Equal(t, "gpt-4", got.Model)
	})

	t.Run("name conflict returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createModel(t, store, newTestModel("model-one", "First Model"))
		createModel(t, store, newTestModel("model-two", "Second Model"))

		// Try to rename model-two to the name of model-one
		updated := &agent.ModelConfig{
			ID:       "model-two",
			Name:     "First Model",
			Provider: "anthropic",
			Model:    "claude-sonnet-4-5",
		}
		err := store.Update(ctx, updated)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrModelNameAlreadyExists)
	})

	t.Run("empty name returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createModel(t, store, newTestModel("has-name", "Has A Name"))

		updated := &agent.ModelConfig{
			ID:       "has-name",
			Name:     "",
			Provider: "anthropic",
			Model:    "claude-sonnet-4-5",
		}
		err := store.Update(ctx, updated)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model name is required")
	})

	t.Run("update to same name is allowed", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createModel(t, store, newTestModel("keep-name", "Keep This Name"))

		updated := &agent.ModelConfig{
			ID:       "keep-name",
			Name:     "Keep This Name",
			Provider: "openai",
			Model:    "gpt-4",
		}
		err := store.Update(ctx, updated)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, "keep-name")
		require.NoError(t, err)
		assert.Equal(t, "Keep This Name", got.Name)
		assert.Equal(t, "openai", got.Provider)
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		model := newTestModel("nonexistent", "Does Not Exist")
		err := store.Update(ctx, model)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrModelNotFound)
	})

	t.Run("nil model returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		err := store.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model cannot be nil")
	})

	t.Run("invalid ID returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		model := &agent.ModelConfig{
			ID:   "../../bad",
			Name: "Bad Model",
		}
		err := store.Update(ctx, model)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrInvalidModelID)
	})
}

// Tests for Delete

func TestStore_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("valid delete", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		createModel(t, store, newTestModel("delete-me", "Delete Me"))

		// Verify file exists before deletion
		filePath := filepath.Join(baseDir, "delete-me.json")
		_, err := os.Stat(filePath)
		require.NoError(t, err)

		err = store.Delete(ctx, "delete-me")
		require.NoError(t, err)

		// Verify file is removed
		_, err = os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))

		// Verify GetByID returns not found
		got, err := store.GetByID(ctx, "delete-me")
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrModelNotFound)
		assert.Nil(t, got)

		// Verify List does not include the deleted model
		models, err := store.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, models)
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		err := store.Delete(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrModelNotFound)
	})

	t.Run("invalid ID returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		tests := []struct {
			name string
			id   string
		}{
			{name: "path traversal", id: "../../etc/passwd"},
			{name: "uppercase", id: "BadID"},
			{name: "empty", id: ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				err := store.Delete(ctx, tt.id)
				require.Error(t, err)
				assert.ErrorIs(t, err, agent.ErrInvalidModelID)
			})
		}
	})

	t.Run("delete does not affect other models", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createModel(t, store, newTestModel("keep-this", "Keep This"))
		createModel(t, store, newTestModel("delete-this", "Delete This"))

		err := store.Delete(ctx, "delete-this")
		require.NoError(t, err)

		// The other model should still be accessible
		got, err := store.GetByID(ctx, "keep-this")
		require.NoError(t, err)
		assert.Equal(t, "Keep This", got.Name)

		models, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "keep-this", models[0].ID)
	})
}

// Tests for rebuildIndex

func TestStore_RebuildIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("index survives rebuild", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)

		// Create some models
		createModel(t, store, newTestModel("alpha", "Alpha Model"))
		createModel(t, store, newTestModel("beta", "Beta Model"))
		createModel(t, store, newTestModel("gamma", "Gamma Model"))

		// Rebuild the index (simulates restart)
		err := store.rebuildIndex()
		require.NoError(t, err)

		// Verify all models are still accessible
		got, err := store.GetByID(ctx, "alpha")
		require.NoError(t, err)
		assert.Equal(t, "Alpha Model", got.Name)

		got, err = store.GetByID(ctx, "beta")
		require.NoError(t, err)
		assert.Equal(t, "Beta Model", got.Name)

		got, err = store.GetByID(ctx, "gamma")
		require.NoError(t, err)
		assert.Equal(t, "Gamma Model", got.Name)

		// Verify list still works and is sorted
		models, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, models, 3)
		assert.Equal(t, "Alpha Model", models[0].Name)
		assert.Equal(t, "Beta Model", models[1].Name)
		assert.Equal(t, "Gamma Model", models[2].Name)

		// Verify files are still on disk
		entries, err := os.ReadDir(baseDir)
		require.NoError(t, err)
		jsonFiles := 0
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".json" {
				jsonFiles++
			}
		}
		assert.Equal(t, 3, jsonFiles)
	})

	t.Run("new store picks up existing files", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()

		// Create store and add models
		store1, err := New(baseDir)
		require.NoError(t, err)
		createModel(t, store1, newTestModel("existing-model", "Existing Model"))

		// Create a new store pointing at the same directory
		store2, err := New(baseDir)
		require.NoError(t, err)

		got, err := store2.GetByID(ctx, "existing-model")
		require.NoError(t, err)
		assert.Equal(t, "Existing Model", got.Name)
	})

	t.Run("rebuild skips invalid JSON files", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		createModel(t, store, newTestModel("valid-model", "Valid Model"))

		// Write an invalid JSON file
		invalidPath := filepath.Join(baseDir, "corrupt.json")
		require.NoError(t, os.WriteFile(invalidPath, []byte("{invalid json}"), 0600))

		// Rebuild should not fail; it should skip the invalid file
		err := store.rebuildIndex()
		require.NoError(t, err)

		// Valid model should still be accessible
		got, err := store.GetByID(ctx, "valid-model")
		require.NoError(t, err)
		assert.Equal(t, "Valid Model", got.Name)
	})

	t.Run("rebuild skips directories", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		createModel(t, store, newTestModel("my-model", "My Model"))

		// Create a subdirectory that should be ignored
		require.NoError(t, os.Mkdir(filepath.Join(baseDir, "subdir"), 0750))

		err := store.rebuildIndex()
		require.NoError(t, err)

		models, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "my-model", models[0].ID)
	})

	t.Run("rebuild skips non-json files", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		createModel(t, store, newTestModel("my-model", "My Model"))

		// Write a non-JSON file
		txtPath := filepath.Join(baseDir, "readme.txt")
		require.NoError(t, os.WriteFile(txtPath, []byte("hello"), 0600))

		err := store.rebuildIndex()
		require.NoError(t, err)

		models, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "my-model", models[0].ID)
	})
}

// Tests for modelFilePath

func TestStore_ModelFilePath(t *testing.T) {
	t.Parallel()

	t.Run("valid ID produces correct path", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)

		p, err := store.modelFilePath("my-model")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(baseDir, "my-model.json"), p)
	})
}

// Integration-style tests

func TestStore_CreateAndRetrieveFullRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := setupTestStore(t)

	model := &agent.ModelConfig{
		ID:               "full-model",
		Name:             "Full Model",
		Provider:         "anthropic",
		Model:            "claude-opus-4-6",
		APIKey:           "sk-ant-test",
		BaseURL:          "https://api.anthropic.com",
		ContextWindow:    200000,
		MaxOutputTokens:  8192,
		InputCostPer1M:   15.0,
		OutputCostPer1M:  75.0,
		SupportsThinking: true,
		Description:      "Full featured model for testing",
	}

	// Create
	err := store.Create(ctx, model)
	require.NoError(t, err)

	// Get
	got, err := store.GetByID(ctx, "full-model")
	require.NoError(t, err)
	assert.Equal(t, model.ID, got.ID)
	assert.Equal(t, model.Name, got.Name)
	assert.Equal(t, model.Provider, got.Provider)
	assert.Equal(t, model.Model, got.Model)
	assert.Equal(t, model.APIKey, got.APIKey)
	assert.Equal(t, model.BaseURL, got.BaseURL)
	assert.Equal(t, model.ContextWindow, got.ContextWindow)
	assert.Equal(t, model.MaxOutputTokens, got.MaxOutputTokens)
	assert.Equal(t, model.InputCostPer1M, got.InputCostPer1M)
	assert.Equal(t, model.OutputCostPer1M, got.OutputCostPer1M)
	assert.Equal(t, model.SupportsThinking, got.SupportsThinking)
	assert.Equal(t, model.Description, got.Description)

	// Update
	model.Name = "Updated Full Model"
	model.Provider = "openai"
	err = store.Update(ctx, model)
	require.NoError(t, err)

	got, err = store.GetByID(ctx, "full-model")
	require.NoError(t, err)
	assert.Equal(t, "Updated Full Model", got.Name)
	assert.Equal(t, "openai", got.Provider)

	// List
	models, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)

	// Delete
	err = store.Delete(ctx, "full-model")
	require.NoError(t, err)

	models, err = store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, models)
}
