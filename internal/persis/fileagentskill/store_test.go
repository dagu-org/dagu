package fileagentskill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func newTestSkill(id, name string) *agent.Skill {
	return &agent.Skill{
		ID:            id,
		SchemaVersion: 1,
		Name:          name,
		Type:          agent.SkillTypeCustom,
		Knowledge:     "Test knowledge content",
	}
}

func createSkill(t *testing.T, store *Store, skill *agent.Skill) {
	t.Helper()
	err := store.Create(context.Background(), skill)
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
		baseDir := filepath.Join(tmpDir, "skills")
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

	t.Run("valid skill", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		skill := newTestSkill("kubernetes", "Kubernetes")

		err := store.Create(ctx, skill)
		require.NoError(t, err)

		// Verify file was created
		filePath := filepath.Join(baseDir, "kubernetes.yaml")
		_, err = os.Stat(filePath)
		require.NoError(t, err)

		// Verify can be retrieved
		got, err := store.GetByID(ctx, "kubernetes")
		require.NoError(t, err)
		assert.Equal(t, skill.ID, got.ID)
		assert.Equal(t, skill.Name, got.Name)
		assert.Equal(t, skill.Type, got.Type)
		assert.Equal(t, skill.Knowledge, got.Knowledge)
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
				skill := newTestSkill(tt.id, "Test Skill")
				err := store.Create(ctx, skill)
				require.Error(t, err)
				assert.ErrorIs(t, err, agent.ErrInvalidSkillID)
			})
		}
	})

	t.Run("duplicate ID returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		skill := newTestSkill("my-skill", "My Skill")
		createSkill(t, store, skill)

		duplicate := newTestSkill("my-skill", "Different Name")
		err := store.Create(ctx, duplicate)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrSkillAlreadyExists)
	})

	t.Run("duplicate name returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		skill := newTestSkill("skill-one", "Same Name")
		createSkill(t, store, skill)

		duplicate := newTestSkill("skill-two", "Same Name")
		err := store.Create(ctx, duplicate)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrSkillNameAlreadyExists)
	})

	t.Run("nil skill returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		err := store.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "skill cannot be nil")
	})

	t.Run("empty name returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		skill := newTestSkill("valid-id", "")

		err := store.Create(ctx, skill)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "skill name is required")
	})
}

// Tests for GetByID

func TestStore_GetByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		skill := newTestSkill("test-skill", "Test Skill")
		skill.Description = "A test skill"
		skill.Version = "1.0.0"
		skill.Author = "Test Author"
		skill.Tags = []string{"test", "example"}
		skill.SchemaVersion = 1
		createSkill(t, store, skill)

		got, err := store.GetByID(ctx, "test-skill")
		require.NoError(t, err)
		assert.Equal(t, skill.ID, got.ID)
		assert.Equal(t, skill.Name, got.Name)
		assert.Equal(t, skill.Description, got.Description)
		assert.Equal(t, skill.Version, got.Version)
		assert.Equal(t, skill.Author, got.Author)
		assert.Equal(t, skill.Tags, got.Tags)
		assert.Equal(t, skill.Type, got.Type)
		assert.Equal(t, skill.Knowledge, got.Knowledge)
		assert.Equal(t, skill.SchemaVersion, got.SchemaVersion)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		got, err := store.GetByID(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrSkillNotFound)
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
				assert.ErrorIs(t, err, agent.ErrInvalidSkillID)
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

		skills, err := store.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, skills)
	})

	t.Run("multiple skills sorted by name", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		// Create skills in non-alphabetical order
		createSkill(t, store, newTestSkill("skill-c", "Zulu Skill"))
		createSkill(t, store, newTestSkill("skill-a", "Alpha Skill"))
		createSkill(t, store, newTestSkill("skill-b", "Mike Skill"))

		skills, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, skills, 3)

		assert.Equal(t, "Alpha Skill", skills[0].Name)
		assert.Equal(t, "Mike Skill", skills[1].Name)
		assert.Equal(t, "Zulu Skill", skills[2].Name)
	})

	t.Run("single skill", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createSkill(t, store, newTestSkill("only-skill", "Only Skill"))

		skills, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, skills, 1)
		assert.Equal(t, "only-skill", skills[0].ID)
		assert.Equal(t, "Only Skill", skills[0].Name)
	})
}

// Tests for Update

func TestStore_Update(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("valid update", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		skill := newTestSkill("update-me", "Original Name")
		createSkill(t, store, skill)

		updated := &agent.Skill{
			ID:        "update-me",
			Name:      "Updated Name",
			Type:      agent.SkillTypeCustom,
			Knowledge: "Updated knowledge",
		}
		err := store.Update(ctx, updated)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, "update-me")
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", got.Name)
		assert.Equal(t, "Updated knowledge", got.Knowledge)
	})

	t.Run("name conflict returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createSkill(t, store, newTestSkill("skill-one", "First Skill"))
		createSkill(t, store, newTestSkill("skill-two", "Second Skill"))

		// Try to rename skill-two to the name of skill-one
		updated := &agent.Skill{
			ID:        "skill-two",
			Name:      "First Skill",
			Type:      agent.SkillTypeCustom,
			Knowledge: "Some knowledge",
		}
		err := store.Update(ctx, updated)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrSkillAlreadyExists)
	})

	t.Run("update to same name is allowed", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createSkill(t, store, newTestSkill("keep-name", "Keep This Name"))

		updated := &agent.Skill{
			ID:        "keep-name",
			Name:      "Keep This Name",
			Type:      agent.SkillTypeCustom,
			Knowledge: "New knowledge",
		}
		err := store.Update(ctx, updated)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, "keep-name")
		require.NoError(t, err)
		assert.Equal(t, "Keep This Name", got.Name)
		assert.Equal(t, "New knowledge", got.Knowledge)
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		skill := newTestSkill("nonexistent", "Does Not Exist")
		err := store.Update(ctx, skill)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrSkillNotFound)
	})

	t.Run("nil skill returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		err := store.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "skill cannot be nil")
	})

	t.Run("invalid ID returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		skill := &agent.Skill{
			ID:   "../../bad",
			Name: "Bad Skill",
		}
		err := store.Update(ctx, skill)
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrInvalidSkillID)
	})
}

// Tests for Delete

func TestStore_Delete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("valid delete", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		createSkill(t, store, newTestSkill("delete-me", "Delete Me"))

		// Verify file exists before deletion
		filePath := filepath.Join(baseDir, "delete-me.yaml")
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
		assert.ErrorIs(t, err, agent.ErrSkillNotFound)
		assert.Nil(t, got)

		// Verify List does not include the deleted skill
		skills, err := store.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, skills)
	})

	t.Run("not found returns error", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)

		err := store.Delete(ctx, "nonexistent")
		require.Error(t, err)
		assert.ErrorIs(t, err, agent.ErrSkillNotFound)
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
				assert.ErrorIs(t, err, agent.ErrInvalidSkillID)
			})
		}
	})

	t.Run("delete does not affect other skills", func(t *testing.T) {
		t.Parallel()
		store, _ := setupTestStore(t)
		createSkill(t, store, newTestSkill("keep-this", "Keep This"))
		createSkill(t, store, newTestSkill("delete-this", "Delete This"))

		err := store.Delete(ctx, "delete-this")
		require.NoError(t, err)

		// The other skill should still be accessible
		got, err := store.GetByID(ctx, "keep-this")
		require.NoError(t, err)
		assert.Equal(t, "Keep This", got.Name)

		skills, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, skills, 1)
		assert.Equal(t, "keep-this", skills[0].ID)
	})
}

// Tests for rebuildIndex

func TestStore_RebuildIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("index survives rebuild", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)

		// Create some skills
		createSkill(t, store, newTestSkill("alpha", "Alpha Skill"))
		createSkill(t, store, newTestSkill("beta", "Beta Skill"))
		createSkill(t, store, newTestSkill("gamma", "Gamma Skill"))

		// Rebuild the index (simulates restart)
		err := store.rebuildIndex()
		require.NoError(t, err)

		// Verify all skills are still accessible
		got, err := store.GetByID(ctx, "alpha")
		require.NoError(t, err)
		assert.Equal(t, "Alpha Skill", got.Name)

		got, err = store.GetByID(ctx, "beta")
		require.NoError(t, err)
		assert.Equal(t, "Beta Skill", got.Name)

		got, err = store.GetByID(ctx, "gamma")
		require.NoError(t, err)
		assert.Equal(t, "Gamma Skill", got.Name)

		// Verify list still works and is sorted
		skills, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, skills, 3)
		assert.Equal(t, "Alpha Skill", skills[0].Name)
		assert.Equal(t, "Beta Skill", skills[1].Name)
		assert.Equal(t, "Gamma Skill", skills[2].Name)

		// Verify files are still on disk
		entries, err := os.ReadDir(baseDir)
		require.NoError(t, err)
		yamlFiles := 0
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".yaml" {
				yamlFiles++
			}
		}
		assert.Equal(t, 3, yamlFiles)
	})

	t.Run("new store picks up existing files", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()

		// Create store and add skills
		store1, err := New(baseDir)
		require.NoError(t, err)
		createSkill(t, store1, newTestSkill("existing-skill", "Existing Skill"))

		// Create a new store pointing at the same directory
		store2, err := New(baseDir)
		require.NoError(t, err)

		got, err := store2.GetByID(ctx, "existing-skill")
		require.NoError(t, err)
		assert.Equal(t, "Existing Skill", got.Name)
	})

	t.Run("rebuild skips invalid YAML files", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		createSkill(t, store, newTestSkill("valid-skill", "Valid Skill"))

		// Write an invalid YAML file
		invalidPath := filepath.Join(baseDir, "corrupt.yaml")
		require.NoError(t, os.WriteFile(invalidPath, []byte("{{invalid yaml}}"), 0600))

		// Rebuild should not fail; it should skip the invalid file
		err := store.rebuildIndex()
		require.NoError(t, err)

		// Valid skill should still be accessible
		got, err := store.GetByID(ctx, "valid-skill")
		require.NoError(t, err)
		assert.Equal(t, "Valid Skill", got.Name)
	})

	t.Run("rebuild skips non-yaml files", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)
		createSkill(t, store, newTestSkill("my-skill", "My Skill"))

		// Write non-YAML files
		require.NoError(t, os.WriteFile(filepath.Join(baseDir, "readme.txt"), []byte("hello"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(baseDir, "data.json"), []byte("{}"), 0600))

		err := store.rebuildIndex()
		require.NoError(t, err)

		skills, err := store.List(ctx)
		require.NoError(t, err)
		require.Len(t, skills, 1)
		assert.Equal(t, "my-skill", skills[0].ID)
	})

	t.Run("rebuild picks up directory-based skills", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()

		// Create a directory-based skill manually
		skillDir := filepath.Join(baseDir, "dir-skill")
		require.NoError(t, os.MkdirAll(skillDir, 0750))

		skillContent := `id: dir-skill
schema_version: 1
name: Directory Skill
type: custom
knowledge: Knowledge from directory skill
`
		require.NoError(t, os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte(skillContent), 0600))

		// Create store — should pick up the directory-based skill
		store, err := New(baseDir)
		require.NoError(t, err)

		got, err := store.GetByID(ctx, "dir-skill")
		require.NoError(t, err)
		assert.Equal(t, "Directory Skill", got.Name)
		assert.Equal(t, "Knowledge from directory skill", got.Knowledge)
	})
}

// Tests for skillFilePath

func TestStore_SkillFilePath(t *testing.T) {
	t.Parallel()

	t.Run("valid ID produces correct path", func(t *testing.T) {
		t.Parallel()
		store, baseDir := setupTestStore(t)

		p, err := store.skillFilePath("my-skill")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(baseDir, "my-skill.yaml"), p)
	})
}

// Integration-style tests

func TestStore_CreateAndRetrieveFullRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := setupTestStore(t)

	skill := &agent.Skill{
		ID:            "full-skill",
		SchemaVersion: 1,
		Name:          "Full Skill",
		Description:   "A comprehensive test skill",
		Version:       "2.1.0",
		Author:        "Test Author",
		Tags:          []string{"test", "integration", "full"},
		Type:          agent.SkillTypeCustom,
		Knowledge:     "Detailed knowledge content for testing.\n\nMultiple paragraphs.",
	}

	// Create
	err := store.Create(ctx, skill)
	require.NoError(t, err)

	// Get
	got, err := store.GetByID(ctx, "full-skill")
	require.NoError(t, err)
	assert.Equal(t, skill.ID, got.ID)
	assert.Equal(t, skill.SchemaVersion, got.SchemaVersion)
	assert.Equal(t, skill.Name, got.Name)
	assert.Equal(t, skill.Description, got.Description)
	assert.Equal(t, skill.Version, got.Version)
	assert.Equal(t, skill.Author, got.Author)
	assert.Equal(t, skill.Tags, got.Tags)
	assert.Equal(t, skill.Type, got.Type)
	assert.Equal(t, skill.Knowledge, got.Knowledge)

	// Update
	skill.Name = "Updated Full Skill"
	skill.Knowledge = "Updated knowledge content"
	err = store.Update(ctx, skill)
	require.NoError(t, err)

	got, err = store.GetByID(ctx, "full-skill")
	require.NoError(t, err)
	assert.Equal(t, "Updated Full Skill", got.Name)
	assert.Equal(t, "Updated knowledge content", got.Knowledge)

	// List
	skills, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, skills, 1)

	// Delete
	err = store.Delete(ctx, "full-skill")
	require.NoError(t, err)

	skills, err = store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, skills)
}

// Concurrency test

func TestStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, _ := setupTestStore(t)

	const goroutines = 10

	// Pre-create some skills
	for i := range goroutines {
		id := fmt.Sprintf("skill-%03d", i)
		createSkill(t, store, newTestSkill(id, fmt.Sprintf("Skill %03d", i)))
	}

	var wg sync.WaitGroup

	// Concurrent reads
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("skill-%03d", idx)
			_, _ = store.GetByID(ctx, id)
		}(i)
	}

	// Concurrent lists
	for range goroutines {
		wg.Go(func() {
			_, _ = store.List(ctx)
		})
	}

	// Concurrent updates
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("skill-%03d", idx)
			skill := newTestSkill(id, fmt.Sprintf("Updated Skill %03d", idx))
			_ = store.Update(ctx, skill)
		}(i)
	}

	wg.Wait()

	// Verify store is still consistent
	skills, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, skills, goroutines)
}
