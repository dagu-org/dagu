// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePathWithinBase(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()

	resolved, err := ResolvePathWithinBase(baseDir, "nested/file.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(baseDir, "nested", "file.txt"), resolved)

	_, err = ResolvePathWithinBase(baseDir, "../escape.txt")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPathEscapesBase)
}

func TestResolveExistingPathWithinBaseRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outside.txt")
	require.NoError(t, os.WriteFile(outsidePath, []byte("secret"), 0o600))

	linkPath := filepath.Join(baseDir, "link.txt")
	if err := os.Symlink(outsidePath, linkPath); err != nil {
		t.Skipf("skipping symlink test: %v", err)
	}

	_, err := ResolveExistingPathWithinBase(baseDir, "link.txt")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPathEscapesBase)
}

func TestResolveExistingPathWithinBaseAllowsSafeSymlink(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	targetPath := filepath.Join(baseDir, "real.txt")
	require.NoError(t, os.WriteFile(targetPath, []byte("ok"), 0o600))

	linkPath := filepath.Join(baseDir, "link.txt")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("skipping symlink test: %v", err)
	}

	resolved, err := ResolveExistingPathWithinBase(baseDir, "link.txt")
	require.NoError(t, err)
	expected, err := filepath.EvalSymlinks(targetPath)
	require.NoError(t, err)
	assert.Equal(t, expected, resolved)
}

func TestResolveExistingPathWithinBasePropagatesMissingFile(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()

	_, err := ResolveExistingPathWithinBase(baseDir, "missing.txt")
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
}

func TestResolvePathWithinBaseAllowsRootBase(t *testing.T) {
	t.Parallel()

	resolved, err := ResolvePathWithinBase(string(filepath.Separator), filepath.Join("tmp", "artifact.txt"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(string(filepath.Separator), "tmp", "artifact.txt"), resolved)
}
