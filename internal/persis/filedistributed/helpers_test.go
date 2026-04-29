// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filedistributed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSortedFiles_IgnoresAtomicWriteTempFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	finalFile := filepath.Join(dir, "worker.json")
	require.NoError(t, os.WriteFile(finalFile, []byte(`{"worker_id":"worker-1"}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "worker.json.tmp.1234"), []byte(`{"worker_id":"partial"`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0750))

	files, err := sortedFiles(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{finalFile}, files)
}
