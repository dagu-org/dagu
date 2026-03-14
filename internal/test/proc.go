// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/fileproc"
	"github.com/stretchr/testify/require"
)

const procFilePattern = "proc_*_%s.proc"

func newProcStore(cfg *config.Config) *fileproc.Store {
	return fileproc.New(
		cfg.Paths.ProcDir,
		fileproc.WithHeartbeatInterval(cfg.Proc.HeartbeatInterval),
		fileproc.WithHeartbeatSyncInterval(cfg.Proc.HeartbeatSyncInterval),
		fileproc.WithStaleThreshold(cfg.Proc.StaleThreshold),
	)
}

func procGroupDir(procDir, groupName, dagName string) string {
	return filepath.Join(procDir, groupName, dagName)
}

// WaitForProcFile returns the heartbeat proc file for the given dag-run once it exists.
func WaitForProcFile(t *testing.T, procDir, groupName string, dagRun exec.DAGRunRef, timeout time.Duration) string {
	t.Helper()

	var match string
	require.Eventually(t, func() bool {
		pattern := filepath.Join(procGroupDir(procDir, groupName, dagRun.Name), fmt.Sprintf(procFilePattern, dagRun.ID))
		matches, err := filepath.Glob(pattern)
		require.NoError(t, err)
		if len(matches) == 0 {
			return false
		}
		match = matches[0]
		return true
	}, timeout, 25*time.Millisecond, "timed out waiting for proc file for %s", dagRun.String())

	return match
}

// RequireHeartbeatAdvance verifies the proc file heartbeat updates within the timeout.
func RequireHeartbeatAdvance(t *testing.T, procFile string, timeout time.Duration) {
	t.Helper()

	initialValue, initialModTime, err := readHeartbeat(procFile)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		value, modTime, err := readHeartbeat(procFile)
		require.NoError(t, err)
		return value > initialValue || modTime.After(initialModTime)
	}, timeout, 25*time.Millisecond, "proc heartbeat did not advance for %s", procFile)
}

// CreateStaleProcFile writes a stale proc heartbeat file for the given dag-run.
func CreateStaleProcFile(
	t *testing.T,
	procDir string,
	groupName string,
	dagRun exec.DAGRunRef,
	startedAt time.Time,
	age time.Duration,
) string {
	t.Helper()

	dir := procGroupDir(procDir, groupName, dagRun.Name)
	require.NoError(t, os.MkdirAll(dir, 0o750))

	staleTime := startedAt.Add(-age).UTC()
	fileName := fmt.Sprintf("proc_%sZ_%s.proc", staleTime.Format("20060102_150405"), dagRun.ID)
	procFile := filepath.Join(dir, fileName)

	data := make([]byte, 8)
	staleUnix := staleTime.Unix()
	require.GreaterOrEqual(t, staleUnix, int64(0), "stale heartbeat timestamp must be after unix epoch")
	binary.BigEndian.PutUint64(data, uint64(staleUnix)) //nolint:gosec // staleUnix is validated non-negative above
	require.NoError(t, os.WriteFile(procFile, data, 0o600))
	require.NoError(t, os.Chtimes(procFile, staleTime, staleTime))

	return procFile
}

// ReadRunStatus loads the persisted status for the given dag-run reference.
func ReadRunStatus(ctx context.Context, t *testing.T, store exec.DAGRunStore, dagRun exec.DAGRunRef) *exec.DAGRunStatus {
	t.Helper()

	attempt, err := store.FindAttempt(ctx, dagRun)
	require.NoError(t, err)
	status, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	return status
}

func readHeartbeat(procFile string) (uint64, time.Time, error) {
	info, err := os.Stat(procFile)
	if err != nil {
		return 0, time.Time{}, err
	}

	data, err := os.ReadFile(procFile) //nolint:gosec // procFile is created in an isolated test temp directory
	if err != nil {
		return 0, time.Time{}, err
	}
	if len(data) < 8 {
		return 0, time.Time{}, fmt.Errorf("heartbeat file %s is too short", procFile)
	}

	return binary.BigEndian.Uint64(data[:8]), info.ModTime(), nil
}
