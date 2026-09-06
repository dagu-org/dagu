// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec018_parallel_foreach_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// Enqueue accepts every child without a scheduler executing the runs.
func TestParallelEnqueue(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "parallel_dag_enqueue.yaml")
	result.ExpectExitCode(0)
	dagu.ExpectFileContains(
		"parallel-enqueue-results.txt",
		`"total": 2`,
		`"queued": 2`,
		`"name": "child-enqueue-item"`,
		`"queue": "parallel-enqueue-queue"`,
	)
}

// Stop must report an aborted run after the first fan-out item starts.
func TestParallelAbort(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	env := []string{"DAGU_HOME=" + filepath.Join(t.TempDir(), "dagu")}
	const runID = "spec018-abort"

	proc := dagu.StartWithEnv(env, "start", "--run-id="+runID, "parallel_timeout_abort.yaml")

	deadline := time.Now().Add(harness.WaitTimeout(t))
	for {
		if _, err := os.Stat(dagu.ProjectPath("started-one.txt")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("item one never started: %s", proc.FailureOutput())
		}
		select {
		case <-proc.Done():
			t.Fatalf("dagu start exited before item one started: %s", proc.FailureOutput())
		case <-time.After(50 * time.Millisecond):
		}
	}
	dagu.ExpectFileContent("started-one.txt", "started\n")

	stopResult := dagu.RunWithEnv(env, "stop", "--run-id="+runID, "parallel_timeout_abort.yaml")
	stopResult.ExpectExitCode(0)

	select {
	case <-proc.Done():
	case <-time.After(harness.WaitTimeout(t)):
		t.Fatal("dagu start did not exit after dagu stop returned")
	}

	status := dagu.RunWithEnv(env, "status", "--run-id="+runID, "parallel_timeout_abort.yaml")
	status.ExpectExitCode(0)
	require.Contains(t, status.Stdout(), "Aborted")
}

// A tolerated child failure propagates partially_succeeded to the parent.
func TestParallelPartial(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	env := []string{"DAGU_HOME=" + filepath.Join(t.TempDir(), "dagu")}
	const runID = "spec018-partial"
	result := dagu.RunWithEnv(env, "start", "--run-id="+runID, "parallel_partially_succeeded.yaml")
	result.ExpectExitCode(0)
	dagu.ExpectFileContains(
		"parallel-partial-results.txt",
		`"total": 2`,
		`"succeeded": 2`,
		`"failed": 0`,
	)

	status := dagu.RunWithEnv(env, "status", "--run-id="+runID, "parallel_partially_succeeded.yaml")
	status.ExpectExitCode(0)
	require.Contains(t, status.Stdout(), "Partially Succeeded")
}
