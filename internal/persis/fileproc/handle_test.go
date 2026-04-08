// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileproc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

func TestProcHandle(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	meta := testProcMetaFromRun(exec.NewDAGRunRef("test_proc", "run-1"))

	fileName := procFilePath(tmpDir, exec.NewUTC(time.Now()), meta)
	proc := NewProcHandler(fileName, meta, 0, 0)

	ctx := context.Background()
	err := proc.startHeartbeat(ctx)
	require.NoError(t, err)

	// File is created synchronously by startHeartbeat
	_, err = os.Stat(fileName)
	require.NoError(t, err)

	// Stop the process
	err = proc.Stop(ctx)
	require.NoError(t, err)

	// Check if the file is deleted
	_, err = os.Stat(fileName)
	require.Error(t, err, "file should be deleted")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestProcHandle_Restart(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := context.Background()

	meta := testProcMetaFromRun(exec.NewDAGRunRef("test_proc", "run-1"))
	fileName := procFilePath(tmpDir, exec.NewUTC(time.Now()), meta)
	proc := NewProcHandler(fileName, meta, 0, 0)

	err := proc.startHeartbeat(ctx)
	require.NoError(t, err)

	// Restart the process
	err = proc.Stop(ctx)
	require.NoError(t, err)

	err = proc.startHeartbeat(ctx)
	require.NoError(t, err)

	// Check if the file is created again
	_, err = os.Stat(fileName)
	require.NoError(t, err)

	err = proc.Stop(ctx)
	require.NoError(t, err)

	// Check if the file is deleted again
	_, err = os.Stat(fileName)
	require.Error(t, err, "file should be deleted")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestProcHandle_RemovesEmptyParentDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create a subdirectory for the proc file
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0750)
	require.NoError(t, err)

	meta := testProcMetaFromRun(exec.NewDAGRunRef("test_proc", "run-1"))
	fileName := procFilePath(subDir, exec.NewUTC(time.Now()), meta)
	proc := NewProcHandler(fileName, meta, 0, 0)
	dagDir := filepath.Dir(fileName)

	err = proc.startHeartbeat(ctx)
	require.NoError(t, err)

	// Check if the file is created
	_, err = os.Stat(fileName)
	require.NoError(t, err)

	// Stop the process
	err = proc.Stop(ctx)
	require.NoError(t, err)

	// Check if the file is deleted
	_, err = os.Stat(fileName)
	require.ErrorIs(t, err, os.ErrNotExist)

	// Check if the DAG directory is also removed.
	_, err = os.Stat(dagDir)
	require.ErrorIs(t, err, os.ErrNotExist, "empty DAG directory should be removed")
}

func TestProcHandle_StartPublishesInitializedFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := context.Background()

	meta := testProcMetaFromRun(exec.NewDAGRunRef("test_proc", "run-1"))
	fileName := procFilePath(tmpDir, exec.NewUTC(time.Now()), meta)
	proc := NewProcHandler(fileName, meta, 0, 0)

	require.NoError(t, proc.startHeartbeat(ctx))
	t.Cleanup(func() {
		_ = proc.Stop(ctx)
	})

	entries, err := os.ReadDir(filepath.Dir(fileName))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, filepath.Base(fileName), entries[0].Name())

	entry, err := readProcEntry(fileName, meta.Name, time.Minute, time.Now())
	require.NoError(t, err)
	require.Equal(t, meta, entry.Meta)
}
