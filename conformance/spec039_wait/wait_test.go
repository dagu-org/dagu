// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec039_wait holds black-box conformance tests for
// Spec 039: Wait Actions (wait.duration, wait.until, wait.file, wait.http).
package spec039_wait_test

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// TestWaitDurationBlocks proves wait.duration actually
// blocks for approximately the configured duration rather than returning
// immediately.
func TestWaitDurationBlocks(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	start := time.Now()
	result := dagu.Run("start", "duration_basic.yaml")
	elapsed := time.Since(start)
	result.ExpectExitCode(0)
	require.GreaterOrEqualf(t, elapsed, 300*time.Millisecond,
		"expected at least the configured 300ms to elapse, took %s", elapsed)
}

func TestWaitDurationMissing(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "duration_missing.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("duration is required")
}

func TestWaitDurationInvalid(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "duration_invalid.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("duration must be a duration")
}

func TestWaitDurationZero(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "duration_zero.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("duration must be greater than 0")
}

// TestWaitUntilPast proves that a with.until
// timestamp already in the past succeeds right away rather than being
// treated as an error or hanging.
func TestWaitUntilPast(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	start := time.Now()
	result := dagu.Run("start", "until_past.yaml")
	elapsed := time.Since(start)
	result.ExpectExitCode(0)
	require.Lessf(t, elapsed, 5*time.Second,
		"expected an already-past until timestamp to succeed immediately, took %s", elapsed)
}

// TestWaitUntilFuture proves a with.until
// timestamp far in the future genuinely blocks the step (rather than, say,
// silently succeeding) by pairing it with a step timeout_sec and confirming
// the step is cancelled instead of completing.
func TestWaitUntilFuture(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "until_future_timeout.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("step timed out")
}

func TestWaitUntilMissing(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "until_missing.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("until is required")
}

func TestWaitUntilBadFormat(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "until_bad_format.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("until must be RFC3339 timestamp")
}

// TestWaitFileExists proves wait.file (default state:
// exists) genuinely polls: the step is still running well after it starts
// (the file does not exist yet), and only completes once the test creates
// the file.
func TestWaitFileExists(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	env := []string{"DAGU_HOME=" + filepath.Join(t.TempDir(), "dagu")}
	const runID = "wait-file-exists"

	proc := dagu.StartWithEnv(env, "start", "--run-id="+runID, "file_wait_exists.yaml")
	defer proc.Stop()

	select {
	case <-proc.Done():
		t.Fatalf("wait.file returned before the file existed: %s", proc.FailureOutput())
	case <-time.After(300 * time.Millisecond):
	}

	dagu.WriteFile("ready.txt", "")

	select {
	case <-proc.Done():
	case <-time.After(harness.WaitTimeout(t)):
		t.Fatal("wait.file did not complete after the file appeared")
	}

	status := dagu.RunWithEnv(env, "status", "--run-id="+runID, "file_wait_exists.yaml")
	status.ExpectExitCode(0)
	require.Containsf(t, status.Stdout(), "Result: Succeeded",
		"expected the run to succeed, got:\n%s", status.Stdout())
}

// TestWaitFileGone proves the state: missing
// variant: the step blocks while the file still exists and only completes
// once the test removes it.
func TestWaitFileGone(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	dagu.WriteFile("present.txt", "x")
	env := []string{"DAGU_HOME=" + filepath.Join(t.TempDir(), "dagu")}
	const runID = "wait-file-gone"

	proc := dagu.StartWithEnv(env, "start", "--run-id="+runID, "file_wait_missing.yaml")
	defer proc.Stop()

	select {
	case <-proc.Done():
		t.Fatalf("wait.file returned before the file was removed: %s", proc.FailureOutput())
	case <-time.After(300 * time.Millisecond):
	}

	require.NoError(t, os.Remove(dagu.ProjectPath("present.txt")))

	select {
	case <-proc.Done():
	case <-time.After(harness.WaitTimeout(t)):
		t.Fatal("wait.file did not complete after the file was removed")
	}

	status := dagu.RunWithEnv(env, "status", "--run-id="+runID, "file_wait_missing.yaml")
	status.ExpectExitCode(0)
	require.Containsf(t, status.Stdout(), "Result: Succeeded",
		"expected the run to succeed, got:\n%s", status.Stdout())
}

func TestWaitFileNoPath(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "file_missing_path.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("path is required")
}

func TestWaitFileBadState(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "file_bad_state.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("state")
}

// waitHTTPTestPort is fixed rather than dynamically allocated: with.url is a
// plain string field, resolved only at execution time, so DAG-build-time
// config-schema validation sees it unresolved -- a $VAR or ${VAR} reference
// in with.url fails URL-format validation before the DAG ever runs. A
// static fixture therefore needs a literal, fixed port.
const waitHTTPTestPort = "18923"

// TestWaitHTTPPolls proves wait.http genuinely polls: the
// test server returns a non-matching status for the first two requests and
// only starts returning the expected 200 afterward, so the step succeeding
// proves it retried rather than checking once.
func TestWaitHTTPPolls(t *testing.T) {
	var requests atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:"+waitHTTPTestPort)
	if err != nil {
		t.Skipf("Skipping: could not bind fixed test port %s: %v", waitHTTPTestPort, err)
	}
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "http_wait_ready.yaml")
	result.ExpectExitCode(0)
	require.GreaterOrEqualf(t, requests.Load(), int32(3),
		"expected at least 3 requests (2 failing + 1 succeeding), got %d", requests.Load())
}

func TestWaitHTTPNoURL(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "http_missing_url.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("url is required")
}

func TestWaitHTTPBadURL(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "http_bad_url.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("must be an absolute HTTP URL")
}

func TestWaitHTTPBadStatus(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "http_bad_status.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("status")
}
