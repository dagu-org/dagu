// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceFileWithRetry(t *testing.T) {
	t.Parallel()

	t.Run("overwrites existing target", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		source := filepath.Join(dir, "source.txt")
		target := filepath.Join(dir, "target.txt")
		require.NoError(t, os.WriteFile(source, []byte("new"), 0o600))
		require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))

		require.NoError(t, ReplaceFileWithRetry(source, target))

		data, err := os.ReadFile(target)
		require.NoError(t, err)
		require.Equal(t, []byte("new"), data)
		require.NoFileExists(t, source)
	})

	t.Run("creates missing target", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		source := filepath.Join(dir, "source.txt")
		target := filepath.Join(dir, "target.txt")
		require.NoError(t, os.WriteFile(source, []byte("new"), 0o600))

		require.NoError(t, ReplaceFileWithRetry(source, target))

		data, err := os.ReadFile(target)
		require.NoError(t, err)
		require.Equal(t, []byte("new"), data)
		require.NoFileExists(t, source)
	})
}
