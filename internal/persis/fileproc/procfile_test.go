// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileproc

import (
	"encoding/binary"
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
