// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package wait

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/runtime"
	"github.com/stretchr/testify/require"
)

func dryRun(t *testing.T, workDir, op string, cfg map[string]any) error {
	t.Helper()
	step := ir.Step{
		Name:           "wait-step",
		Commands:       []ir.CommandEntry{{Command: op}},
		ExecutorConfig: ir.ExecutorConfig{Type: executorType, Config: cfg},
	}
	dag := &ir.DAG{Name: "wait-dry", WorkingDir: workDir, WorkingDirExplicit: true}
	ctx := runtime.NewContext(context.Background(), dag, "run-1", "")
	ctx = runtime.WithEnv(ctx, runtime.NewEnv(ctx, step))
	return dryRunCheck(ctx, step)
}

func TestDryRunCheckMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ready.flag"), []byte("ok"), 0o600))

	require.ErrorContains(t, dryRun(t, dir, opFile, map[string]any{"path": "missing-9f3c2b1a.txt"}),
		`field 'with.path': missing-9f3c2b1a.txt: file not found`)
	require.NoError(t, dryRun(t, dir, opFile, map[string]any{"path": "ready.flag"}))
	require.NoError(t, dryRun(t, dir, opFile, map[string]any{"path": "${steps.build.outputs.flag}"}),
		"a path still holding a reference cannot be checked")
	require.NoError(t, dryRun(t, dir, opFile, map[string]any{"path": "missing-9f3c2b1a.txt", "state": "${params.state}"}),
		"a state still holding a reference cannot be checked")
	require.NoError(t, dryRun(t, dir, opFile, map[string]any{"path": "missing-9f3c2b1a.txt", "state": "missing"}),
		"a missing target is the success condition")
	require.NoError(t, dryRun(t, dir, opDuration, map[string]any{"duration": "1s"}),
		"non-file operations have nothing to check")
}
