// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package wait

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/stretchr/testify/require"
)

func TestExecutorRunWaitsForDuration(t *testing.T) {
	t.Parallel()

	exec, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opDuration}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"duration": "5ms",
			},
		},
	})
	require.NoError(t, err)

	start := time.Now()
	require.NoError(t, exec.Run(context.Background()))
	require.GreaterOrEqual(t, time.Since(start), 5*time.Millisecond)
}

func TestExecutorRunWaitsUntilFileExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "ready.flag")
	exec, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opFile}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"path":          target,
				"poll_interval": "1ms",
			},
		},
	})
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(5 * time.Millisecond)
		errCh <- os.WriteFile(target, []byte("ready"), 0o600)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, exec.Run(ctx))
	require.NoError(t, <-errCh)
}

func TestExecutorRunWaitsForHTTPStatus(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	exec, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opHTTP}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"url":           server.URL,
				"status":        http.StatusNoContent,
				"poll_interval": "1ms",
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, exec.Run(context.Background()))
	require.GreaterOrEqual(t, calls.Load(), int32(3))
}

func TestValidateStepDefersReferenceFields(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		op     string
		config map[string]any
	}{
		{name: "duration", op: opDuration, config: map[string]any{"duration": "${params.delay}"}},
		{name: "until", op: opUntil, config: map[string]any{"until": "${steps.plan.outputs.deadline}"}},
		{name: "state", op: opFile, config: map[string]any{"path": "ready.flag", "state": "${params.state}"}},
		{
			name:   "http",
			op:     opHTTP,
			config: map[string]any{"url": "${steps.plan.outputs.url}", "status": "${steps.plan.outputs.code}"},
		},
		{name: "pollInterval", op: opDuration, config: map[string]any{"duration": "5ms", "poll_interval": "${params.interval}"}},
		{name: "braceEnvDuration", op: opDuration, config: map[string]any{"duration": "${DELAY}"}},
		{name: "bareEnvURL", op: opHTTP, config: map[string]any{"url": "$SERVICE_URL", "status": "${STATUS_CODE}"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, validateStep(ir.Step{
				Commands:       []ir.CommandEntry{{Command: tt.op}},
				ExecutorConfig: ir.ExecutorConfig{Type: "wait", Config: tt.config},
			}))
		})
	}
}

func TestValidateStepStillRejectsLiteralValues(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		op     string
		config map[string]any
		want   string
	}{
		{name: "duration", op: opDuration, config: map[string]any{"duration": "bogus"}, want: "duration must be a duration"},
		{name: "until", op: opUntil, config: map[string]any{"until": "tomorrow"}, want: "until must be RFC3339 timestamp"},
		{name: "state", op: opFile, config: map[string]any{"path": "ready.flag", "state": "soon"}, want: `state must be "exists" or "missing"`},
		{name: "url", op: opHTTP, config: map[string]any{"url": "ftp://example.com/ready"}, want: "url must be an absolute HTTP URL"},
		{name: "status", op: opHTTP, config: map[string]any{"url": "http://example.com/", "status": 99}, want: "status must be between 100 and 599"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateStep(ir.Step{
				Commands:       []ir.CommandEntry{{Command: tt.op}},
				ExecutorConfig: ir.ExecutorConfig{Type: "wait", Config: tt.config},
			})
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestNewExecutorRejectsNonHTTPURL(t *testing.T) {
	t.Parallel()

	_, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opHTTP}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"url": "ftp://example.com/ready",
			},
		},
	})
	require.ErrorContains(t, err, "url must be an absolute HTTP URL")
}

func TestExecutorRunStopsWhenContextIsCanceled(t *testing.T) {
	t.Parallel()

	exec, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opDuration}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"duration": "1h",
			},
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, exec.Run(ctx), context.Canceled)
}
