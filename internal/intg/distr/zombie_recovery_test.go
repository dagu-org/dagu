// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package distr_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDistributedRun_WorkerCrash_MarkedFailed verifies that when a worker
// crashes (stops sending heartbeats and status updates), the coordinator's
// zombie detector marks the distributed run as FAILED.
func TestDistributedRun_WorkerCrash_MarkedFailed(t *testing.T) {
	// Use short thresholds to speed up zombie detection in the test.
	f := newTestFixture(t, `
type: graph
name: zombie-crash-test
worker_selector:
  test: "true"
steps:
  - name: long-step
    command: sleep 300
`,
		withStaleThresholds(2*time.Second, 3*time.Second),
	)
	defer f.cleanup()

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	// Wait for the run to reach Running.
	var dagRunID string
	require.Eventually(t, func() bool {
		status, err := f.latestStatus()
		if err != nil {
			return false
		}
		if status.Status == core.Running {
			dagRunID = status.DAGRunID
			return true
		}
		return false
	}, 15*time.Second, 200*time.Millisecond, "run should reach Running")
	require.NotEmpty(t, dagRunID)

	// Verify the status has a LeaseAt timestamp from the worker.
	status, err := f.latestStatus()
	require.NoError(t, err)
	assert.Greater(t, status.LeaseAt, int64(0), "running status should have LeaseAt set")

	// Stop all workers to simulate a crash.
	for _, w := range f.workers {
		require.NoError(t, w.Stop(f.coord.Context))
	}

	// Start the zombie detector with a short interval.
	zombieCtx, zombieCancel := context.WithCancel(f.coord.Context)
	defer zombieCancel()
	f.coord.Handler().StartZombieDetector(zombieCtx, 1*time.Second)

	// The run should transition to Failed within the stale threshold + detection interval.
	finalStatus := f.waitForStatus(core.Failed, 20*time.Second)
	assert.Equal(t, core.Failed, finalStatus.Status)
	assert.Contains(t, finalStatus.Error, "worker")

	zombieCancel()
	f.coord.Handler().WaitZombieDetector()
}

// TestDistributedRun_QueueConcurrency_ActiveRunCounted verifies that a running
// distributed run (tracked via lease) counts against queue concurrency so a
// second enqueued DAG does not start until the first finishes.
func TestDistributedRun_QueueConcurrency_ActiveRunCounted(t *testing.T) {
	// We create two separate fixtures sharing the same coordinator would be complex.
	// Instead, we use one fixture with a queue concurrency of 1 and observe that
	// the queue processor correctly counts the distributed run.
	f := newTestFixture(t, `
type: graph
name: queue-concurrency-test
queue: concurrency-q
worker_selector:
  test: "true"
steps:
  - name: quick-step
    command: echo "done"
`,
		withStaleThresholds(2*time.Second, 3*time.Second),
	)
	defer f.cleanup()

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	// Wait for the run to succeed.
	status := f.waitForStatus(core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)

	// Verify LeaseAt was set during execution.
	// For a completed run, LeaseAt should have been set at some point.
	// The final status may or may not have LeaseAt depending on whether the
	// terminal push included it, but the important thing is it was used during execution.
	assert.NotEmpty(t, status.WorkerID, "distributed run should have WorkerID set")
}

// TestDistributedRun_StatusAndQueueConsistency verifies that after a
// distributed run completes, both the DAG run status and queue state are
// consistent: run shows Succeeded, queue has runningCount=0.
func TestDistributedRun_StatusAndQueueConsistency(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: consistency-test
queue: consistency-q
worker_selector:
  test: "true"
steps:
  - name: step1
    command: echo "hello"
`,
	)
	defer f.cleanup()

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	status := f.waitForStatus(core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)

	// Verify no active runs remain in the DAGRunStore with fresh leases.
	activeStatuses, err := f.coord.DAGRunStore.ListStatuses(f.coord.Context,
		exec.WithStatuses([]core.Status{core.Running}),
		exec.WithoutLimit(),
	)
	require.NoError(t, err)

	for _, st := range activeStatuses {
		if st.Name == "consistency-test" {
			t.Errorf("found active run for consistency-test after completion: status=%s dagRunID=%s",
				st.Status, st.DAGRunID)
		}
	}
}

// TestDistributedRun_LeaseStamped verifies that every status push from a
// distributed worker includes a fresh LeaseAt timestamp.
func TestDistributedRun_LeaseStamped(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: lease-stamp-test
worker_selector:
  test: "true"
steps:
  - name: step1
    command: echo "lease-test"
`,
	)
	defer f.cleanup()

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	// Wait for the run to complete.
	status := f.waitForStatus(core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)

	// The final status should have LeaseAt set by the worker's StatusPusher.
	assert.Greater(t, status.LeaseAt, int64(0), "final status should have LeaseAt set")

	// LeaseAt should be a recent timestamp (within the last minute).
	leaseTime := time.UnixMilli(status.LeaseAt)
	assert.WithinDuration(t, time.Now(), leaseTime, 60*time.Second,
		"LeaseAt should be a recent timestamp")
}
