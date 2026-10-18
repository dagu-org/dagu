// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package notification

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/eventstore"
	"github.com/dagucloud/dagu/v2/internal/ir"
	notificationmodel "github.com/dagucloud/dagu/v2/internal/notification"
	"github.com/dagucloud/dagu/v2/internal/service/chatbridge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeliverCommandExportsRunContextAndRemovesLogFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "run.log")
	stdoutPath := filepath.Join(dir, "build.stdout")
	require.NoError(t, os.WriteFile(logPath, []byte("dag failed\n"), 0o644))
	require.NoError(t, os.WriteFile(stdoutPath, []byte("step output\n"), 0o644))
	outPath := filepath.Join(dir, "captured.txt")

	handlerErr := filepath.Join(dir, "handler.stderr")
	require.NoError(t, os.WriteFile(handlerErr, []byte("handler output\n"), 0o644))

	svc := newCommandService(WithPublicURL("https://dagu.example.com"))
	err := svc.deliverTarget(context.Background(), notificationmodel.Target{
		Type: notificationmodel.ProviderCommand,
		Command: &notificationmodel.CommandTarget{
			Command: captureCommand(outPath),
		},
	}, []chatbridge.NotificationEvent{{
		Type: eventstore.TypeDAGRunFailed,
		Status: &ir.DAGRunStatus{
			Name:     "daily-report",
			DAGRunID: "run-1",
			Status:   ir.Failed,
			Log:      logPath,
			Nodes: []*ir.Node{{
				Step:   ir.Step{Name: "build"},
				Stdout: stdoutPath,
			}},
			OnFailure: &ir.Node{
				Step:   ir.Step{Name: "notify"},
				Stderr: handlerErr,
			},
		},
		ObservedAt: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC),
	}})
	require.NoError(t, err)

	body, err := os.ReadFile(outPath)
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "daily-report")
	assert.Contains(t, text, "failed")
	assert.Contains(t, text, "dag failed")
	assert.Contains(t, text, "step output")
	assert.Contains(t, text, "# DAG log")
	assert.Contains(t, text, "# stdout build")
	assert.Contains(t, text, "# stderr notify")
	assert.Contains(t, text, "handler output")

	executionLog := executionLogPath(text)
	require.NotEmpty(t, executionLog)
	_, statErr := os.Stat(executionLog)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestDeliverCommandPassesErrorAsEnv(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell syntax")
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	outPath := filepath.Join(dir, "out.txt")
	payload := "hello'; touch " + marker + "; $(touch " + marker + ")"
	svc := newCommandService()
	err := svc.deliverTarget(context.Background(), notificationmodel.Target{
		Type: notificationmodel.ProviderCommand,
		Command: &notificationmodel.CommandTarget{
			Command: fmt.Sprintf(`printf '%%s' "$ERROR" > %q`, outPath),
		},
	}, []chatbridge.NotificationEvent{{
		Type: eventstore.TypeDAGRunFailed,
		Status: &ir.DAGRunStatus{
			Name:   "daily-report",
			Status: ir.Failed,
			Error:  payload,
		},
	}})
	require.NoError(t, err)
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	body, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, payload, string(body))
}

// A token inside quotes must not let run text, such as a step's stderr in
// run.error, run as shell syntax.
func TestDeliverCommandLeavesTokensLiteral(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell syntax")
	}

	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	outPath := filepath.Join(dir, "out.txt")
	svc := newCommandService()
	err := svc.deliverTarget(context.Background(), notificationmodel.Target{
		Type: notificationmodel.ProviderCommand,
		Command: &notificationmodel.CommandTarget{
			Command: fmt.Sprintf(`printf '%%s' "{{run.error}}" > %q`, outPath),
		},
	}, []chatbridge.NotificationEvent{{
		Type: eventstore.TypeDAGRunFailed,
		Status: &ir.DAGRunStatus{
			Name:   "daily-report",
			Status: ir.Failed,
			Error:  "$(touch " + marker + ")",
		},
	}})
	require.NoError(t, err)
	_, statErr := os.Stat(marker)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	body, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, "{{run.error}}", string(body))
}

// The command starts from the configured base environment, not the
// server process environment, which can hold Dagu's own credentials.
func TestDeliverCommandUsesConfiguredEnv(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "env.txt")
	svc := New(nil, nil, WithCommandChannels(append(os.Environ(), "BASE_ONLY=from-base")))
	t.Setenv("PROCESS_ONLY", "from-process")

	err := svc.deliverTarget(context.Background(), notificationmodel.Target{
		Type: notificationmodel.ProviderCommand,
		Command: &notificationmodel.CommandTarget{
			Command: envDumpCommand(outPath),
		},
	}, []chatbridge.NotificationEvent{{
		Type:   eventstore.TypeDAGRunFailed,
		Status: &ir.DAGRunStatus{Name: "daily-report", Status: ir.Failed},
	}})
	require.NoError(t, err)
	body, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "BASE_ONLY=from-base")
	assert.Contains(t, string(body), "DAG_NAME=daily-report")
	assert.NotContains(t, string(body), "PROCESS_ONLY")
}

func TestDeliverCommandDisabled(t *testing.T) {
	t.Parallel()

	svc := New(nil, nil)
	err := svc.deliverTarget(context.Background(), notificationmodel.Target{
		Type: notificationmodel.ProviderCommand,
		Command: &notificationmodel.CommandTarget{
			Command: failingCommand(),
		},
	}, []chatbridge.NotificationEvent{{
		Type:   eventstore.TypeDAGRunFailed,
		Status: &ir.DAGRunStatus{Name: "daily-report", Status: ir.Failed},
	}})
	assert.ErrorContains(t, err, "command channels are disabled")
}

func TestDeliverCommandTruncatesExecutionLog(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "run.log")
	require.NoError(t, os.WriteFile(logPath, bytes.Repeat([]byte("x"), 200), 0o644))
	outPath := filepath.Join(dir, "captured.txt")
	svc := newCommandService()
	svc.notificationLogLimit = 64
	err := svc.deliverTarget(context.Background(), notificationmodel.Target{
		Type: notificationmodel.ProviderCommand,
		Command: &notificationmodel.CommandTarget{
			Command: captureCommand(outPath),
		},
	}, []chatbridge.NotificationEvent{{
		Type: eventstore.TypeDAGRunFailed,
		Status: &ir.DAGRunStatus{
			Name:   "daily-report",
			Status: ir.Failed,
			Log:    logPath,
		},
	}})
	require.NoError(t, err)
	body, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "# log truncated")
	assert.NotContains(t, string(body), strings.Repeat("x", 80))
}

func TestDeliverCommandIncludesStderrOnFailure(t *testing.T) {
	t.Parallel()

	svc := newCommandService()
	err := svc.deliverTarget(context.Background(), notificationmodel.Target{
		Type: notificationmodel.ProviderCommand,
		Command: &notificationmodel.CommandTarget{
			Command: failingCommand(),
		},
	}, []chatbridge.NotificationEvent{{
		Type:   eventstore.TypeDAGRunFailed,
		Status: &ir.DAGRunStatus{Name: "daily-report", Status: ir.Failed},
	}})
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
	assert.ErrorContains(t, err, "status 7")
}

func TestDeliverCommandTimesOut(t *testing.T) {
	t.Parallel()

	svc := newCommandService()
	svc.notificationCommandTimeout = 200 * time.Millisecond
	start := time.Now()
	err := svc.deliverTarget(context.Background(), notificationmodel.Target{
		Type: notificationmodel.ProviderCommand,
		Command: &notificationmodel.CommandTarget{
			Command: slowCommand(),
		},
	}, []chatbridge.NotificationEvent{{
		Type:   eventstore.TypeDAGRunFailed,
		Status: &ir.DAGRunStatus{Name: "daily-report", Status: ir.Failed},
	}})
	require.Error(t, err)
	assert.ErrorContains(t, err, "timed out")
	assert.Less(t, time.Since(start), 5*time.Second)
}

func newCommandService(opts ...Option) *Service {
	return New(nil, nil, append([]Option{WithCommandChannels(os.Environ())}, opts...)...)
}

func envDumpCommand(outPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`set > "%s"`, outPath)
	}
	return fmt.Sprintf("env > %q", outPath)
}

func captureCommand(outPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`(echo %%DAG_NAME%%& echo %%STATUS%%& echo %%EXECUTION_LOG%%)> "%s" & if not "%%EXECUTION_LOG%%"=="" type "%%EXECUTION_LOG%%" >> "%s"`, outPath, outPath)
	}
	return fmt.Sprintf("printf '%%s\\n' \"$DAG_NAME\" \"$STATUS\" \"$EXECUTION_LOG\" > %q && if [ -n \"$EXECUTION_LOG\" ]; then cat \"$EXECUTION_LOG\" >> %q; fi", outPath, outPath)
}

func executionLogPath(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "daily-report" || line == "failed" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "dagu-notification-log-") {
			return line
		}
	}
	return ""
}

func failingCommand() string {
	if runtime.GOOS == "windows" {
		return "echo boom 1>&2& exit /b 7"
	}
	return "echo boom >&2; exit 7"
}

func slowCommand() string {
	if runtime.GOOS == "windows" {
		return "ping -n 6 127.0.0.1 >NUL"
	}
	return "sleep 5"
}
