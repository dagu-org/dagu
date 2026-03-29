// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileagentskill

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedExampleSkills_NoBundledExamplesNoop(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	exampleIDs := ExampleSkillIDs()

	seeded := SeedExampleSkills(baseDir)

	if len(exampleIDs) == 0 {
		assert.False(t, seeded)
		_, err := os.Stat(filepath.Join(baseDir, examplesMarkerFile))
		assert.True(t, os.IsNotExist(err))
		return
	}

	assert.True(t, seeded)

	_, err := os.Stat(filepath.Join(baseDir, examplesMarkerFile))
	assert.NoError(t, err)

	for _, id := range exampleIDs {
		skillPath := filepath.Join(baseDir, id, skillFilename)
		info, err := os.Stat(skillPath)
		require.NoError(t, err, "expected %s to exist", skillPath)
		assert.True(t, info.Size() > 0)
	}
}

func TestSeedExampleSkills_MarkerPreventsReCreation(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	exampleIDs := ExampleSkillIDs()

	if len(exampleIDs) == 0 {
		assert.False(t, SeedExampleSkills(baseDir))
		_, err := os.Stat(filepath.Join(baseDir, examplesMarkerFile))
		assert.True(t, os.IsNotExist(err))
		return
	}

	assert.True(t, SeedExampleSkills(baseDir))

	// Delete one skill directory.
	require.NoError(t, os.RemoveAll(filepath.Join(baseDir, exampleIDs[0])))

	// Second seed should not re-create (marker exists).
	assert.False(t, SeedExampleSkills(baseDir))

	// Deleted skill stays deleted.
	_, err := os.Stat(filepath.Join(baseDir, exampleIDs[0], skillFilename))
	assert.True(t, os.IsNotExist(err))
}

func TestSeedExampleSkills_ExistingSkillsSkipSeed(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()

	// Manually create a skill directory before seeding.
	customDir := filepath.Join(baseDir, "custom-skill")
	require.NoError(t, os.MkdirAll(customDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(customDir, skillFilename), []byte("---\nname: Custom\n---\ntest"), 0600))

	assert.False(t, SeedExampleSkills(baseDir))

	// No marker file should be created.
	_, err := os.Stat(filepath.Join(baseDir, examplesMarkerFile))
	assert.True(t, os.IsNotExist(err))
}

func TestSeedExampleSkills_ValidContent(t *testing.T) {
	t.Parallel()
	baseDir := t.TempDir()
	exampleIDs := ExampleSkillIDs()

	if len(exampleIDs) == 0 {
		assert.False(t, SeedExampleSkills(baseDir))
		store, err := New(baseDir)
		require.NoError(t, err)
		skills, err := store.List(context.Background())
		require.NoError(t, err)
		assert.Empty(t, skills)
		return
	}

	require.True(t, SeedExampleSkills(baseDir))

	store, err := New(baseDir)
	require.NoError(t, err)

	skills, err := store.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, skills, len(exampleIDs))

	for _, skill := range skills {
		assert.NotEmpty(t, skill.Name, "skill %s should have a name", skill.ID)
		assert.NotEmpty(t, skill.Description, "skill %s should have a description", skill.ID)
		assert.NotEmpty(t, skill.Tags, "skill %s should have tags", skill.ID)
		assert.NotEmpty(t, skill.Knowledge, "skill %s should have knowledge", skill.ID)
	}
}
