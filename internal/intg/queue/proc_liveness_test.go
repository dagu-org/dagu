// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package queue_test

import (
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	queueTestProcHeartbeatInterval = 150 * time.Millisecond
	queueTestProcStaleThreshold    = time.Second
)

func TestSchedulerProcHeartbeat_QueuedRun(t *testing.T) {
	f := newFixture(t, `
name: queued-proc-heartbeat
steps:
  - name: sleep
    command: sleep 6
`, WithProcConfig(queueTestProcHeartbeatInterval, queueTestProcHeartbeatInterval, queueTestProcStaleThreshold)).
		Enqueue(1).
		StartScheduler(30 * time.Second)
	defer f.Stop()

	runID := f.runIDs[0]
	f.WaitForStatus(runID, core.Running, 10*time.Second)

	ref := exec.NewDAGRunRef(f.dag.Name, runID)
	time.Sleep(2 * time.Second)
	alive, err := f.th.ProcStore.IsRunAlive(f.th.Context, f.dag.ProcGroup(), ref)
	require.NoError(t, err)
	require.True(t, alive)

	f.WaitForStatus(runID, core.Succeeded, 20*time.Second)
}

func TestSchedulerRepairsStaleLocalRunAndCleansProcFile(t *testing.T) {
	f := newFixture(t, `
name: scheduler-stale-repair
steps:
  - name: step1
    command: echo never
`, WithProcConfig(50*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond), WithZombieConfig(50*time.Millisecond, 1))
	defer f.Stop()

	dagRunID := uuid.Must(uuid.NewV7()).String()
	ref := exec.NewDAGRunRef(f.dag.Name, dagRunID)
	attempt, err := f.th.DAGRunStore.CreateAttempt(f.th.Context, f.dag, time.Now(), dagRunID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	status := transform.NewStatusBuilder(f.dag).Create(
		dagRunID,
		core.Running,
		0,
		time.Now().Add(-2*time.Second),
		transform.WithAttemptID(attempt.ID()),
		transform.WithHierarchyRefs(ref, exec.DAGRunRef{}),
	)
	require.NotEmpty(t, status.Nodes)
	status.Nodes[0].Status = core.NodeRunning

	require.NoError(t, attempt.Open(f.th.Context))
	require.NoError(t, attempt.Write(f.th.Context, status))
	require.NoError(t, attempt.Close(f.th.Context))

	procFile := test.CreateStaleProcFileWithAttempt(
		t,
		f.th.Config.Paths.ProcDir,
		f.dag.ProcGroup(),
		ref,
		attempt.ID(),
		time.Now().Add(-2*time.Second),
		time.Second,
	)

	f.StartScheduler(10 * time.Second)
	f.WaitForStatus(dagRunID, core.Failed, 10*time.Second)

	repaired, err := f.Status(dagRunID)
	require.NoError(t, err)
	require.Equal(t, core.NodeFailed, repaired.Nodes[0].Status)
	require.Contains(t, repaired.Nodes[0].Error, "stale local process detected")

	require.Eventually(t, func() bool {
		_, err := os.Stat(procFile)
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond)
}

func TestQueueStaleProcFileDoesNotBlockDrain(t *testing.T) {
	f := newFixture(t, `
name: queue-stale-cleanup
max_active_runs: 1
steps:
  - name: echo
    command: echo hello
`, WithProcConfig(50*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond)).
		Enqueue(1)
	defer f.Stop()

	fakeRunID := uuid.Must(uuid.NewV7()).String()
	fakeRef := exec.NewDAGRunRef(f.dag.Name, fakeRunID)
	procFile := test.CreateStaleProcFile(
		t,
		f.th.Config.Paths.ProcDir,
		f.dag.ProcGroup(),
		fakeRef,
		time.Now().Add(-2*time.Second),
		time.Second,
	)

	f.StartScheduler(20 * time.Second)
	f.WaitForStatus(f.runIDs[0], core.Succeeded, 10*time.Second)

	require.Eventually(t, func() bool {
		_, err := os.Stat(procFile)
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond)
}
