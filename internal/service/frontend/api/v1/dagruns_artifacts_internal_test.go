// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	openapiv1 "github.com/dagucloud/dagu/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildArtifactPreviewReturnsMetadataWithoutContentForLargeFiles(t *testing.T) {
	t.Parallel()

	archiveDir := t.TempDir()
	content := strings.Repeat("# heading\n", int(artifactPreviewMaxBytes/8)+1)
	path := filepath.Join(archiveDir, "large.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	preview, err := buildArtifactPreview(archiveDir, "large.md")
	require.NoError(t, err)
	assert.Equal(t, openapiv1.ArtifactPreviewKindMarkdown, preview.Kind)
	assert.True(t, preview.TooLarge)
	assert.False(t, preview.Truncated)
	assert.Nil(t, preview.Content)
	assert.Greater(t, preview.Size, artifactPreviewMaxBytes)
}

func TestResolveArtifactPathRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	archiveDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.txt")
	require.NoError(t, os.WriteFile(outsidePath, []byte("secret"), 0o600))

	linkPath := filepath.Join(archiveDir, "escape.txt")
	require.NoError(t, os.Symlink(outsidePath, linkPath))

	_, err := resolveArtifactPath(archiveDir, "escape.txt")
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestListArtifactTreeSkipsSymlinks(t *testing.T) {
	t.Parallel()

	archiveDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.txt")
	require.NoError(t, os.WriteFile(outsidePath, []byte("outside"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "inside.txt"), []byte("inside"), 0o600))
	require.NoError(t, os.Symlink(outsidePath, filepath.Join(archiveDir, "link.txt")))

	items, err := listArtifactTree(archiveDir)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "inside.txt", items[0].Name)
}
