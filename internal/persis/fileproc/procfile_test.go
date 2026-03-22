// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileproc

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

func TestReadProcEntry_ShortFileErrors(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	meta := exec.ProcMeta{
		StartedAt:    time.Now().Unix(),
		Name:         "short-dag",
		DAGRunID:     "run-1",
		AttemptID:    "abcdef",
		RootName:     "short-dag",
		RootDAGRunID: "run-1",
	}
	path := procFilePath(baseDir, exec.NewUTC(time.Now()), meta)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0750))

	t.Run("missing heartbeat header", func(t *testing.T) {
		require.NoError(t, os.WriteFile(path, []byte("short"), 0600))

		_, err := readProcEntry(path, "queue", time.Minute, time.Now())
		require.ErrorContains(t, err, "shorter than the 8-byte heartbeat header")
	})

	t.Run("missing metadata payload", func(t *testing.T) {
		buf := make([]byte, heartbeatSize)
		binary.BigEndian.PutUint64(buf, uint64(time.Now().Unix())) //nolint:gosec
		require.NoError(t, os.WriteFile(path, buf, 0600))

		_, err := readProcEntry(path, "queue", time.Minute, time.Now())
		require.ErrorContains(t, err, "missing metadata payload")
	})
}

func TestReadProcEntry_LegacyProcFile(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	now := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Minute)
	heartbeatAt := now.Add(-10 * time.Second)

	path := writeLegacyProcFile(t, baseDir, "legacy-group", "legacy-dag", "legacy-run", createdAt, heartbeatAt)

	entry, err := readProcEntry(path, "legacy-group", time.Minute, now)
	require.NoError(t, err)
	require.Equal(t, "legacy-group", entry.GroupName)
	require.Equal(t, path, entry.FilePath)
	require.Equal(t, heartbeatAt.Unix(), entry.LastHeartbeatAt)
	require.True(t, entry.Fresh)
	require.Equal(t, exec.ProcMeta{
		StartedAt:    createdAt.Unix(),
		Name:         "legacy-dag",
		DAGRunID:     "legacy-run",
		AttemptID:    legacyProcAttemptID("legacy-run"),
		RootName:     "legacy-dag",
		RootDAGRunID: "legacy-run",
	}, entry.Meta)
}

func writeLegacyProcFile(
	t *testing.T,
	baseDir, groupName, dagName, dagRunID string,
	createdAt, heartbeatAt time.Time,
) string {
	t.Helper()

	path := filepath.Join(
		baseDir,
		groupName,
		dagName,
		fmt.Sprintf("%s%s_%s%s", procFilePrefix, createdAt.UTC().Format(procFileTimeFmt), dagRunID, procFileExt),
	)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))

	buf := make([]byte, heartbeatSize)
	binary.BigEndian.PutUint64(buf, uint64(heartbeatAt.UTC().Unix())) //nolint:gosec
	require.NoError(t, os.WriteFile(path, buf, 0o600))
	require.NoError(t, os.Chtimes(path, heartbeatAt, heartbeatAt))

	return path
}
