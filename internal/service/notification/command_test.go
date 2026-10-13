// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package notification

import (
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

	svc := New(nil, nil, WithPublicURL("https://dagu.example.com"))
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

	executionLog := executionLogPath(text)
	require.NotEmpty(t, executionLog)
	_, statErr := os.Stat(executionLog)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestDeliverCommandIncludesStderrOnFailure(t *testing.T) {
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
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
	assert.ErrorContains(t, err, "status 7")
}

func TestDeliverCommandTimesOut(t *testing.T) {
	t.Parallel()

	svc := New(nil, nil)
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

func captureCommand(outPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`(echo {{dag.name}}& echo %%DAG_NAME%%& echo %%STATUS%%& echo %%EXECUTION_LOG%%)> "%s" & if not "%%EXECUTION_LOG%%"=="" type "%%EXECUTION_LOG%%" >> "%s"`, outPath, outPath)
	}
	return fmt.Sprintf("printf '%%s\\n' '{{dag.name}}' \"$DAG_NAME\" \"$STATUS\" \"$EXECUTION_LOG\" > %q && if [ -n \"$EXECUTION_LOG\" ]; then cat \"$EXECUTION_LOG\" >> %q; fi", outPath, outPath)
}

func executionLogPath(text string) string {
	for _, line := range strings.Split(text, "\n") {
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
