// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package distr_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/worker"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testStaleHeartbeatThreshold = 2 * time.Second
	testStaleLeaseThreshold     = 3 * time.Second
	testZombieDetectorInterval  = 500 * time.Millisecond
)

// TestDistributedRun_WorkerCrash_MarkedFailed verifies that a hard-killed worker
// is treated as a crash and the coordinator's zombie detector marks the run FAILED.
func TestDistributedRun_WorkerCrash_MarkedFailed(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: zombie-crash-test
worker_selector:
  test: "true"
steps:
  - name: long-step
    command: sleep 300
`,
		withWorkerCount(0),
		withStaleThresholds(testStaleHeartbeatThreshold, testStaleLeaseThreshold),
		withZombieDetectionInterval(testZombieDetectorInterval),
	)
	defer f.cleanup()

	workerCmd, _ := startWorkerProcess(t, f, "crash-worker", "test=true")

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(90 * time.Second)

	status := f.waitForStatus(core.Running, 15*time.Second)
	require.Equal(t, core.Running, status.Status)
	require.NotEmpty(t, status.AttemptKey)
	require.Equal(t, "crash-worker", status.WorkerID)
	lease := waitForLease(t, f, status.AttemptKey, 5*time.Second)
	require.Equal(t, "crash-worker", lease.WorkerID)

	require.NoError(t, cmdutil.KillProcessGroup(workerCmd, os.Kill))

	finalStatus := f.waitForStatus(core.Failed, 20*time.Second)
	assert.Equal(t, core.Failed, finalStatus.Status)
	assert.Contains(t, finalStatus.Error, "worker")
}

func TestDistributedRun_AckedTaskWithoutInitialStatus_MarkedFailedAndCleansLease(t *testing.T) {
	t.Run("SharedNothing", func(t *testing.T) {
		testDistributedRunAckedTaskWithoutInitialStatus(t, sharedNothingMode)
	})

	t.Run("SharedStorage", func(t *testing.T) {
		testDistributedRunAckedTaskWithoutInitialStatus(t, sharedFSMode)
	})
}

// TestDistributedRun_HeartbeatRefreshKeepsQuietRunAlive verifies that a
// long-running quiet step remains RUNNING past the lease threshold because
// coordinator-owned heartbeat refreshes keep the lease fresh.
func TestDistributedRun_HeartbeatRefreshKeepsQuietRunAlive(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: quiet-heartbeat-test
worker_selector:
  test: "true"
steps:
  - name: long-step
    command: sleep 8
`,
		withStaleThresholds(testStaleHeartbeatThreshold, testStaleLeaseThreshold),
		withZombieDetectionInterval(testZombieDetectorInterval),
	)
	defer f.cleanup()

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	status := f.waitForStatus(core.Running, 15*time.Second)
	require.NotEmpty(t, status.AttemptKey)
	initialLease := waitForLease(t, f, status.AttemptKey, 5*time.Second).LastHeartbeatAt

	time.Sleep(4 * time.Second)

	status, err := f.latestStatus()
	require.NoError(t, err)
	require.Equal(t, core.Running, status.Status)
	lease := waitForLease(t, f, status.AttemptKey, 5*time.Second)
	assert.Greater(t, lease.LastHeartbeatAt, initialLease)
	assert.WithinDuration(t, time.Now(), time.UnixMilli(lease.LastHeartbeatAt), 2*time.Second)

	finalStatus := f.waitForStatus(core.Succeeded, 15*time.Second)
	assert.Equal(t, core.Succeeded, finalStatus.Status)
}

// TestDistributedRun_QueueConcurrency_ActiveRunCounted verifies that a running
// distributed run with fresh heartbeats continues to block the next queued item.
func TestDistributedRun_QueueConcurrency_ActiveRunCounted(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: queue-concurrency-test
queue: concurrency-q
worker_selector:
  test: "true"
steps:
  - name: long-step
    command: sleep 8
`,
		withStaleThresholds(testStaleHeartbeatThreshold, testStaleLeaseThreshold),
		withZombieDetectionInterval(testZombieDetectorInterval),
	)
	defer f.cleanup()

	require.NoError(t, f.enqueue())
	require.NoError(t, f.enqueue())

	require.Eventually(t, func() bool {
		count, err := f.coord.QueueStore.Len(f.coord.Context, "concurrency-q")
		return err == nil && count == 2
	}, 5*time.Second, 100*time.Millisecond, "both runs should be queued before scheduling starts")

	f.startScheduler(30 * time.Second)

	require.Eventually(t, func() bool {
		statuses, err := f.coord.DAGRunStore.ListStatuses(
			f.coord.Context,
			exec.WithExactName("queue-concurrency-test"),
			exec.WithoutLimit(),
		)
		if err != nil || len(statuses) < 2 {
			return false
		}

		var running, queued int
		for _, st := range statuses {
			switch st.Status {
			case core.Running:
				running++
			case core.Queued:
				queued++
			case core.NotStarted, core.Failed, core.Aborted, core.Succeeded, core.PartiallySucceeded, core.Waiting, core.Rejected:
			}
		}

		return running == 1 && queued == 1
	}, 15*time.Second, 100*time.Millisecond, "one run should start and one should remain queued")

	time.Sleep(4 * time.Second)

	statuses, err := f.coord.DAGRunStore.ListStatuses(
		f.coord.Context,
		exec.WithExactName("queue-concurrency-test"),
		exec.WithoutLimit(),
	)
	require.NoError(t, err)

	var running, queued int
	for _, st := range statuses {
		switch st.Status {
		case core.Running:
			running++
		case core.Queued:
			queued++
		case core.NotStarted, core.Failed, core.Aborted, core.Succeeded, core.PartiallySucceeded, core.Waiting, core.Rejected:
		}
	}

	assert.Equal(t, 1, running, "fresh distributed lease should keep the first run counted as active")
	assert.Equal(t, 1, queued, "second run should remain queued while the first run is active")

	require.Eventually(t, func() bool {
		statuses, err := f.coord.DAGRunStore.ListStatuses(
			f.coord.Context,
			exec.WithExactName("queue-concurrency-test"),
			exec.WithoutLimit(),
		)
		if err != nil || len(statuses) < 2 {
			return false
		}

		succeeded := 0
		for _, st := range statuses {
			if st.Status == core.Succeeded {
				succeeded++
			}
		}
		return succeeded == 2
	}, 30*time.Second, 200*time.Millisecond, "both queued runs should eventually complete")
}

// TestDistributedRun_StatusAndQueueConsistency verifies that after a
// distributed run completes, both the DAG run status and queue state are
// consistent: run shows Succeeded, queue has no active entries.
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

	activeStatuses, err := f.coord.DAGRunStore.ListStatuses(f.coord.Context,
		exec.WithStatuses([]core.Status{core.Running}),
		exec.WithoutLimit(),
	)
	require.NoError(t, err)

	var offendingStatus string
	for _, st := range activeStatuses {
		if st.Name == "consistency-test" {
			offendingStatus = fmt.Sprintf("status=%s dagRunID=%s", st.Status, st.DAGRunID)
			break
		}
	}
	assert.Emptyf(t, offendingStatus,
		"found active run for consistency-test after completion: %s",
		offendingStatus,
	)

	require.Eventually(t, func() bool {
		queueLen, err := f.coord.QueueStore.Len(f.coord.Context, "consistency-q")
		return err == nil && queueLen == 0
	}, 5*time.Second, 100*time.Millisecond, "queue should have no remaining entries after completion")
}

// TestDistributedRun_CoordinatorOwnsSharedLease verifies that distributed runs
// create a shared lease while active and remove it after completion.
func TestDistributedRun_CoordinatorOwnsSharedLease(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: lease-stamp-test
worker_selector:
  test: "true"
steps:
  - name: step1
    command: sleep 3
`,
	)
	defer f.cleanup()

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	status := f.waitForStatus(core.Running, 20*time.Second)
	require.Equal(t, core.Running, status.Status)
	require.NotEmpty(t, status.AttemptKey)

	lease, err := f.coord.DAGRunLeaseStore.Get(f.coord.Context, status.AttemptKey)
	require.NoError(t, err)
	require.NotNil(t, lease)
	assert.Equal(t, status.AttemptKey, lease.AttemptKey)
	assert.Equal(t, status.AttemptID, lease.AttemptID)
	assert.Equal(t, "worker-1", lease.WorkerID)
	assert.Equal(t, "test-coordinator", lease.Owner.ID)
	assert.WithinDuration(t, time.Now(), time.UnixMilli(lease.LastHeartbeatAt), 5*time.Second)

	finalStatus := f.waitForStatus(core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, finalStatus.Status)

	require.Eventually(t, func() bool {
		_, err := f.coord.DAGRunLeaseStore.Get(f.coord.Context, status.AttemptKey)
		return errors.Is(err, exec.ErrDAGRunLeaseNotFound)
	}, 10*time.Second, 100*time.Millisecond, "shared lease should be removed after completion")
}

func testDistributedRunAckedTaskWithoutInitialStatus(t *testing.T, mode workerMode) {
	t.Helper()

	opts := []fixtureOption{
		withWorkerCount(0),
		withStaleThresholds(testStaleHeartbeatThreshold, testStaleLeaseThreshold),
		withZombieDetectionInterval(testZombieDetectorInterval),
	}
	if mode == sharedFSMode {
		opts = append(opts, withWorkerMode(sharedFSMode))
	}

	f := newTestFixture(t, `
type: graph
name: ack-orphan-test
worker_selector:
  test: "true"
steps:
  - name: step1
    command: echo "recovered"
`, opts...)
	defer f.cleanup()

	labels := map[string]string{"test": "true"}
	var (
		crashWorker *worker.Worker
		abandonOnce sync.Once
	)
	afterAckHook := func(context.Context, *coordinatorv1.Task) bool {
		triggered := false
		abandonOnce.Do(func() {
			triggered = true
		})
		return triggered
	}

	switch mode {
	case sharedFSMode:
		crashWorker = f.setupSharedFSWorkerWithAfterAckHook("crash-worker", labels, afterAckHook)
	case sharedNothingMode:
		crashWorker = f.setupSharedNothingWorkerWithAfterAckHook("crash-worker", labels, "", afterAckHook)
	default:
		t.Fatalf("unsupported worker mode: %v", mode)
	}
	require.NotNil(t, crashWorker)

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	lease := waitForAnyLease(t, f, 5*time.Second)
	require.Equal(t, "crash-worker", lease.WorkerID)

	queuedStatus, err := f.latestStatus()
	require.NoError(t, err)
	require.Equal(t, core.Queued, queuedStatus.Status)
	require.Equal(t, lease.AttemptKey, queuedStatus.AttemptKey)

	finalStatus := f.waitForStatus(core.Failed, 20*time.Second)
	require.Equal(t, core.Failed, finalStatus.Status)
	assert.Equal(t, lease.AttemptKey, finalStatus.AttemptKey)
	assert.Contains(t, finalStatus.Error, "distributed run lease expired")
	assert.Contains(t, finalStatus.Error, "accepted the task claim")
	assert.Contains(t, finalStatus.Error, "owner coordinator")

	require.Eventually(t, func() bool {
		_, err := f.coord.DAGRunLeaseStore.Get(f.coord.Context, lease.AttemptKey)
		return errors.Is(err, exec.ErrDAGRunLeaseNotFound)
	}, 10*time.Second, 100*time.Millisecond, "stale distributed lease should be removed after failure")

	crashWorker.SetAfterTaskAckHook(nil)
}

func startWorkerProcess(t *testing.T, f *testFixture, workerID, labels string) (*osexec.Cmd, *bytes.Buffer) {
	t.Helper()

	args := []string{
		"worker",
		"--config", f.coord.Config.Paths.ConfigFileUsed,
		"--worker.id", workerID,
		"--worker.health-port=0",
		"--worker.coordinators", f.coord.Address(),
	}
	if labels != "" {
		args = append(args, "--worker.labels", labels)
	}

	cmd := osexec.Command(f.coord.Config.Paths.Executable, args...)
	cmdutil.SetupCommand(cmd)
	cmd.Env = append([]string{}, f.coord.ChildEnv...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	require.NoError(t, cmd.Start())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = cmd.Wait()
	}()

	t.Cleanup(func() {
		if cmd.Process != nil && (cmd.ProcessState == nil || !cmd.ProcessState.Exited()) {
			_ = cmdutil.KillProcessGroup(cmd, os.Kill)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Logf("worker process %s did not exit within 5 seconds", workerID)
		}
	})

	f.waitForWorkerRegistration(workerID, 10*time.Second)

	return cmd, &output
}

func waitForLease(t *testing.T, f *testFixture, attemptKey string, timeout time.Duration) exec.DAGRunLease {
	t.Helper()

	var lease *exec.DAGRunLease
	require.Eventually(t, func() bool {
		current, err := f.coord.DAGRunLeaseStore.Get(f.coord.Context, attemptKey)
		if err != nil {
			return false
		}
		lease = current
		return lease != nil
	}, timeout, 100*time.Millisecond, "lease %s should exist", attemptKey)

	return *lease
}

func waitForAnyLease(t *testing.T, f *testFixture, timeout time.Duration) exec.DAGRunLease {
	t.Helper()

	var lease exec.DAGRunLease
	require.Eventually(t, func() bool {
		leases, err := f.coord.DAGRunLeaseStore.ListAll(f.coord.Context)
		if err != nil || len(leases) == 0 {
			return false
		}
		lease = leases[0]
		return true
	}, timeout, 100*time.Millisecond, "a distributed lease should exist")

	return lease
}
