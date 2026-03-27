// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRetryCommand(t *testing.T) {
	t.Run("RetryDAGWithFilePath", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, `params: "p1"
steps:
  - name: "1"
    command: echo param is $1
`)

		// Run a DAG.
		args := []string{"start", `--params="foo"`, dagFile.Location}
		th.RunCommand(t, cmd.Start(), test.CmdTest{Args: args})

		// Find the dag-run ID.
		dagStore := th.DAGStore
		ctx := context.Background()

		dag, err := dagStore.GetMetadata(ctx, dagFile.Location)
		require.NoError(t, err)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(ctx, dag)
		require.NoError(t, err)
		require.Equal(t, dagRunStatus.Status, core.Succeeded)
		require.NotNil(t, dagRunStatus.Status)

		// Retry with the dag-run ID using file path.
		args = []string{"retry", fmt.Sprintf("--run-id=%s", dagRunStatus.DAGRunID), dagFile.Location}
		th.RunCommand(t, cmd.Retry(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{`[1=foo]`},
		})
	})

	t.Run("RetryDAGWithName", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, `params: "p1"
steps:
  - name: "1"
    command: echo param is $1
`)

		// Run a DAG.
		args := []string{"start", `--params="bar"`, dagFile.Location}
		th.RunCommand(t, cmd.Start(), test.CmdTest{Args: args})

		// Find the dag-run ID.
		dagStore := th.DAGStore
		ctx := context.Background()

		dag, err := dagStore.GetMetadata(ctx, dagFile.Location)
		require.NoError(t, err)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(ctx, dag)
		require.NoError(t, err)
		require.Equal(t, dagRunStatus.Status, core.Succeeded)
		require.NotNil(t, dagRunStatus.Status)

		// Retry with the dag-run ID using DAG name.
		args = []string{"retry", fmt.Sprintf("--run-id=%s", dagRunStatus.DAGRunID), dag.Name}
		th.RunCommand(t, cmd.Retry(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{`[1=bar]`},
		})
	})

	t.Run("QueuedCatchupRegeneratesLogAndPreservesTriggerType", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, `name: queued-catchup-dag
steps:
  - name: "1"
    command: echo queued catchup
`)

		runID := "queued-catchup-run"
		attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dagFile.DAG, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		scheduleTime := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		status := transform.NewStatusBuilder(dagFile.DAG).Create(
			runID,
			core.Queued,
			0,
			time.Time{},
			transform.WithAttemptID(attempt.ID()),
			transform.WithTriggerType(core.TriggerTypeCatchUp),
			transform.WithQueuedAt(stringutil.FormatTime(time.Now())),
			transform.WithScheduleTime(stringutil.FormatTime(scheduleTime)),
		)
		writeStatus(t, th.Context, attempt, status)

		args := []string{"retry", fmt.Sprintf("--run-id=%s", runID), dagFile.Location}
		th.RunCommand(t, cmd.Retry(), test.CmdTest{Args: args})

		latestAttempt, err := th.DAGRunStore.FindAttempt(th.Context, exec.NewDAGRunRef(dagFile.Name, runID))
		require.NoError(t, err)

		latestStatus, err := latestAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, latestStatus.Status)
		require.Equal(t, core.TriggerTypeCatchUp, latestStatus.TriggerType)
		require.NotEmpty(t, latestStatus.Log)
		require.FileExists(t, latestStatus.Log)
	})

	t.Run("QueuedRetryCreatesNewAttempt", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, `name: queued-retry-dag
steps:
  - name: "1"
    command: echo queued retry
`)

		runID := "queued-retry-run"
		attempt, err := th.DAGRunStore.CreateAttempt(th.Context, dagFile.DAG, time.Now(), runID, exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		logPath := filepath.Join(th.Config.Paths.LogDir, "queued-retry-test.log")
		require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0o750))

		status := transform.NewStatusBuilder(dagFile.DAG).Create(
			runID,
			core.Queued,
			0,
			time.Time{},
			transform.WithAttemptID(attempt.ID()),
			transform.WithTriggerType(core.TriggerTypeRetry),
			transform.WithQueuedAt(stringutil.FormatTime(time.Now())),
			transform.WithLogFilePath(logPath),
		)
		writeStatus(t, th.Context, attempt, status)

		args := []string{"retry", fmt.Sprintf("--run-id=%s", runID), dagFile.Location}
		th.RunCommand(t, cmd.Retry(), test.CmdTest{Args: args})

		latestAttempt, err := th.DAGRunStore.FindAttempt(th.Context, exec.NewDAGRunRef(dagFile.Name, runID))
		require.NoError(t, err)
		require.NotEqual(t, attempt.ID(), latestAttempt.ID())

		latestStatus, err := latestAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, latestStatus.Status)
		require.Equal(t, core.TriggerTypeRetry, latestStatus.TriggerType)
	})

	t.Run("TrueRetryKeepsRetryTriggerType", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, `name: retry-trigger-dag
steps:
  - name: "1"
    command: echo retry trigger
`)

		th.RunCommand(t, cmd.Start(), test.CmdTest{Args: []string{"start", dagFile.Location}})

		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dagFile.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, status.Status)

		args := []string{"retry", fmt.Sprintf("--run-id=%s", status.DAGRunID), dagFile.Location}
		th.RunCommand(t, cmd.Retry(), test.CmdTest{Args: args})

		latestAttempt, err := th.DAGRunStore.FindAttempt(th.Context, exec.NewDAGRunRef(dagFile.Name, status.DAGRunID))
		require.NoError(t, err)

		latestStatus, err := latestAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, latestStatus.Status)
		require.Equal(t, core.TriggerTypeRetry, latestStatus.TriggerType)
	})
}

func TestRetryCommand_BuiltExecutableRestoresExplicitEnv(t *testing.T) {
	th := test.SetupCommand(t, test.WithBuiltExecutable())
	markerPath := th.TempFile(t, "retry-marker", nil)
	require.NoError(t, os.Remove(markerPath))

	dag := th.DAG(t, fmt.Sprintf(`name: built-retry-explicit-env
env:
  - EXPORTED_SECRET: ${CMD_RETRY_EXPLICIT_ENV}
steps:
  - name: "capture"
    shell: bash
    command: |
      if [ ! -f %[1]q ]; then
        touch %[1]q
        printf '%%s|%%s' "$EXPORTED_SECRET" "${CMD_RETRY_EXPLICIT_ENV:-}"
        exit 1
      fi
      printf '%%s|%%s' "$EXPORTED_SECRET" "${CMD_RETRY_EXPLICIT_ENV:-}"
    output: RESULT
`, markerPath))

	_, err := test.RunBuiltCLICommand(t, th.Helper, []string{"CMD_RETRY_EXPLICIT_ENV=from-host"}, "start", dag.Location)
	require.Error(t, err)

	initialStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Failed, initialStatus.Status)

	initialAttempt, err := th.DAGRunStore.FindAttempt(th.Context, exec.NewDAGRunRef(dag.Name, initialStatus.DAGRunID))
	require.NoError(t, err)

	test.RunBuiltCLI(t, th.Helper, nil, "retry", fmt.Sprintf("--run-id=%s", initialStatus.DAGRunID), dag.Location)

	retriedAttempt, err := th.DAGRunStore.FindAttempt(th.Context, exec.NewDAGRunRef(dag.Name, initialStatus.DAGRunID))
	require.NoError(t, err)
	require.NotEqual(t, initialAttempt.ID(), retriedAttempt.ID())

	retriedStatus, err := retriedAttempt.ReadStatus(th.Context)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, retriedStatus.Status)
	require.Equal(t, "from-host|", test.StatusOutputValue(t, retriedStatus, "RESULT"))
}

func writeStatus(t *testing.T, ctx context.Context, attempt exec.DAGRunAttempt, status exec.DAGRunStatus) {
	t.Helper()

	require.NoError(t, attempt.Open(ctx))
	require.NoError(t, attempt.Write(ctx, status))
	require.NoError(t, attempt.Close(ctx))
}
