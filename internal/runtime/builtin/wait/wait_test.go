// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package wait

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/runtime"
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
		{name: "embeddedBraceEnvURL", op: opHTTP, config: map[string]any{"url": "${BASE_URL}/health"}},
		{name: "embeddedBareEnvURL", op: opHTTP, config: map[string]any{"url": "$BASE_URL/health"}},
		{name: "embeddedEnvDuration", op: opDuration, config: map[string]any{"duration": "${SECS}s"}},
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

func TestParseStatus(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		raw           any
		want          int
		wantErrSubstr string
	}{
		{name: "unset", raw: nil, want: http.StatusOK},
		{name: "integer", raw: http.StatusNoContent, want: http.StatusNoContent},
		{name: "yamlInteger", raw: uint64(http.StatusNoContent), want: http.StatusNoContent},
		{name: "int32", raw: int32(http.StatusNoContent), want: http.StatusNoContent},
		{name: "jsonNumber", raw: json.Number("204"), want: http.StatusNoContent},
		{name: "wholeFloat", raw: float64(http.StatusNoContent), want: http.StatusNoContent},
		{name: "negativeInteger", raw: int64(-1), wantErrSubstr: "status must be between 100 and 599"},
		{name: "overflowingUint", raw: uint64(1<<32 + http.StatusOK), wantErrSubstr: "status must be between 100 and 599"},
		{name: "fractional", raw: 204.5, wantErrSubstr: "status must be between 100 and 599"},
		{name: "numericString", raw: "204", want: http.StatusNoContent},
		{name: "badString", raw: "soon", wantErrSubstr: "status must be between 100 and 599"},
		{name: "outOfRange", raw: 99, wantErrSubstr: "status must be between 100 and 599"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseStatus(tt.raw)
			if tt.wantErrSubstr != "" {
				require.ErrorContains(t, err, tt.wantErrSubstr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestValidateStepDefersUntilReferences(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateStep(ir.Step{
		Commands: []ir.CommandEntry{{Command: opUntil}},
		ExecutorConfig: ir.ExecutorConfig{Type: "wait", Config: map[string]any{
			"until":    "${params.release_at}",
			"offset":   "${params.shift}",
			"timezone": "${params.zone}",
			"if_past":  "${params.on_past}",
			"window":   "${params.grace}",
		}},
	}))
}

func TestParseUntilClockTimeDefersResolution(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	cfg.Until = "23:59"
	parsed, err := parseConfig(opUntil, cfg, false)
	require.NoError(t, err)
	require.True(t, parsed.until.IsZero())
}

func TestParseUntilRejectsBadFields(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		config config
		op     string
		want   string
	}{
		{name: "until", config: config{Until: "tomorrow"}, op: opUntil, want: "until must be RFC3339 timestamp or HH:MM"},
		{name: "timezone", config: config{Until: "2026-01-02T03:04:05Z", Timezone: "Mars/Olympus"}, op: opUntil, want: "timezone must be an IANA name"},
		{name: "offset", config: config{Until: "2026-01-02T03:04:05Z", Offset: "bogus"}, op: opUntil, want: "offset must be a duration"},
		{name: "ifPast", config: config{Until: "2026-01-02T03:04:05Z", IfPast: "retry"}, op: opUntil, want: "if_past must be"},
		{name: "windowMissing", config: config{Until: "2026-01-02T03:04:05Z", IfPast: "window"}, op: opUntil, want: "window is required"},
		{name: "windowMismatch", config: config{Until: "2026-01-02T03:04:05Z", Window: "5m"}, op: opUntil, want: "window requires if_past"},
		{name: "windowBad", config: config{Until: "2026-01-02T03:04:05Z", IfPast: "window", Window: "bogus"}, op: opUntil, want: "window must be a duration"},
		{name: "offsetOnDuration", config: config{Duration: "5s", Offset: "1m"}, op: opDuration, want: "only valid for until"},
		{
			name:   "untilFieldsReportedInOrder",
			config: config{Duration: "5s", Offset: "1m", Timezone: "UTC", IfPast: "fail", Window: "1m"},
			op:     opDuration,
			want:   "offset is only valid for until",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := defaultConfig()
			cfg.Until = tt.config.Until
			cfg.Offset = tt.config.Offset
			cfg.Timezone = tt.config.Timezone
			cfg.IfPast = tt.config.IfPast
			cfg.Window = tt.config.Window
			cfg.Duration = tt.config.Duration
			_, err := parseConfig(tt.op, cfg, false)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestClockDeadlineAppliesOffsetBeforeRoll(t *testing.T) {
	t.Parallel()

	loc := time.UTC
	base := time.Date(2026, 1, 2, 0, 0, 0, 0, loc)
	now := time.Date(2026, 1, 2, 9, 5, 0, 0, loc)

	got, err := clockDeadline(base, "09:00", loc, 10*time.Minute, ifPastContinue, now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 1, 2, 9, 10, 0, 0, loc), got)
}

func TestClockDeadlineHonoursIfPast(t *testing.T) {
	t.Parallel()

	loc := time.UTC
	base := time.Date(2026, 1, 2, 0, 0, 0, 0, loc)
	now := time.Date(2026, 1, 2, 9, 5, 0, 0, loc)

	for _, tt := range []struct {
		name   string
		ifPast string
		want   time.Time
	}{
		{name: "continueRollsForward", ifPast: ifPastContinue, want: time.Date(2026, 1, 3, 9, 0, 0, 0, loc)},
		{name: "failKeepsLogicalDay", ifPast: ifPastFail, want: time.Date(2026, 1, 2, 9, 0, 0, 0, loc)},
		{name: "windowKeepsLogicalDay", ifPast: ifPastWindow, want: time.Date(2026, 1, 2, 9, 0, 0, 0, loc)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := clockDeadline(base, "09:00", loc, 0, tt.ifPast, now)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExecutorIfPastAppliesToClockTime(t *testing.T) {
	t.Parallel()

	loc := time.Local
	scheduled := time.Now().In(loc).AddDate(0, 0, -1).Format(time.RFC3339Nano)
	ctx := runtime.NewContext(context.Background(), &ir.DAG{Name: "test"}, "run-1", "",
		runtime.WithScheduleTime(scheduled))

	exec, err := newExecutor(ctx, ir.Step{
		Commands: []ir.CommandEntry{{Command: opUntil}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"until":   "00:00",
				"if_past": "fail",
			},
		},
	})
	require.NoError(t, err)
	require.ErrorContains(t, exec.Run(ctx), "in the past")

	exec, err = newExecutor(ctx, ir.Step{
		Commands: []ir.CommandEntry{{Command: opUntil}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"until":   "00:00",
				"if_past": "window",
				"window":  "72h",
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, exec.Run(ctx))
}

func TestClockDeadlineUsesLogicalDayAndTimezone(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("Europe/London")
	require.NoError(t, err)
	base := time.Date(2026, 1, 2, 0, 0, 0, 0, loc)
	now := time.Date(2026, 1, 2, 8, 0, 0, 0, loc)

	got, err := clockDeadline(base, "09:00", loc, 0, ifPastContinue, now)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 1, 2, 9, 0, 0, 0, loc), got)
}

func TestLogicalDayUsesScheduleTime(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("Europe/London")
	require.NoError(t, err)
	ctx := runtime.NewContext(context.Background(), &ir.DAG{Name: "test"}, "run-1", "",
		runtime.WithScheduleTime("2026-01-02T23:30:00Z"))

	base := logicalDay(ctx, loc)
	require.Equal(t, 2026, base.Year())
	require.Equal(t, time.January, base.Month())
	require.Equal(t, 2, base.Day())
	require.Equal(t, 23, base.Hour())
}

func TestExecutorRFC3339OffsetAppliedOnce(t *testing.T) {
	t.Parallel()

	// Deadline lands 1m in the past after a single +2m shift; a doubled
	// offset would land 1m in the future and block instead of succeeding.
	until := time.Now().Add(-3 * time.Minute).UTC().Format(time.RFC3339Nano)
	exec, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opUntil}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"until":   until,
				"offset":  "2m",
				"if_past": "window",
				"window":  "2m",
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, exec.Run(context.Background()))
}

func TestExecutorIfPastFail(t *testing.T) {
	t.Parallel()

	exec, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opUntil}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"until":   "2000-01-01T00:00:00Z",
				"if_past": "fail",
			},
		},
	})
	require.NoError(t, err)
	require.ErrorContains(t, exec.Run(context.Background()), "in the past")
}

func TestExecutorIfPastWindow(t *testing.T) {
	t.Parallel()

	recent := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	exec, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opUntil}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"until":   recent,
				"if_past": "window",
				"window":  "5m",
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, exec.Run(context.Background()))
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

// A reference that survives run-time resolution means it did not resolve, so
// the error says so instead of only reporting the field as malformed.
func TestNewExecutorRejectsUnresolvedReference(t *testing.T) {
	t.Parallel()

	_, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opHTTP}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"url": "${BASE_URL}/health",
			},
		},
	})
	require.ErrorContains(t, err, "url must be an absolute HTTP URL")
	require.ErrorContains(t, err, "unresolved reference")
}

func TestDeadlineLoggedToStderr(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	exec, err := newExecutor(context.Background(), ir.Step{
		Commands: []ir.CommandEntry{{Command: opUntil}},
		ExecutorConfig: ir.ExecutorConfig{
			Type: "wait",
			Config: map[string]any{
				"until": "2000-01-01T00:00:00Z",
			},
		},
	})
	require.NoError(t, err)
	exec.SetStdout(&stdout)
	exec.SetStderr(&stderr)

	require.NoError(t, exec.Run(context.Background()))
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "wait: deadline")
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
