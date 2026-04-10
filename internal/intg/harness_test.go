// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dagucloud/dagu/internal/cmd"
	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRetryRestoresHarnessConfigFromBaseConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses cat as a fake harness provider")
	}

	baseDir := t.TempDir()
	baseConfigPath := filepath.Join(baseDir, "base.yaml")
	require.NoError(t, os.WriteFile(baseConfigPath, []byte(`
harnesses:
  passthrough:
    binary: cat
    prompt_mode: stdin
harness:
  provider: passthrough
`), 0o600))

	th := test.SetupCommand(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Paths.BaseConfig = baseConfigPath
	}))

	th.CreateDAGFile(t, "harness_retry.yaml", `
steps:
  - name: review
    command: Review the repository
    script: |
      summarize the current branch
`)

	dagRunID := uuid.Must(uuid.NewV7()).String()
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        []string{"start", "--run-id", dagRunID, "harness_retry"},
		ExpectedOut: []string{"DAG run finished"},
	})

	ctx := context.Background()
	ref := exec.NewDAGRunRef("harness_retry", dagRunID)
	attempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	status, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, status.Status)
	require.Len(t, status.Nodes, 1)
	require.Equal(t, "harness", status.Nodes[0].Step.ExecutorConfig.Type)
	require.Equal(t, "passthrough", status.Nodes[0].Step.ExecutorConfig.Config["provider"])

	stdout, err := os.ReadFile(status.Nodes[0].Stdout)
	require.NoError(t, err)
	require.Contains(t, string(stdout), "Review the repository")
	require.Contains(t, string(stdout), "summarize the current branch")

	status.Nodes[0].Status = core.NodeFailed
	require.NoError(t, th.DAGRunMgr.UpdateStatus(ctx, ref, *status))

	th.RunCommand(t, cmd.Retry(), test.CmdTest{
		Args:        []string{"retry", "--run-id", dagRunID, "harness_retry"},
		ExpectedOut: []string{"DAG run finished"},
	})

	retriedAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	retriedStatus, err := retriedAttempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, retriedStatus.Status)
	require.Len(t, retriedStatus.Nodes, 1)
	require.Equal(t, core.NodeSucceeded, retriedStatus.Nodes[0].Status)

	retriedStdout, err := os.ReadFile(retriedStatus.Nodes[0].Stdout)
	require.NoError(t, err)
	require.Contains(t, string(retriedStdout), "Review the repository")
	require.Contains(t, string(retriedStdout), "summarize the current branch")
}
