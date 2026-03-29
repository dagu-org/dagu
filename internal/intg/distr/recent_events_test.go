// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package distr_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/service/eventfeed"
	"github.com/stretchr/testify/require"
)

func TestRecentEventsDistributedWorkerAndStaleRepairShareFeed(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: distributed-worker-failure
worker_selector:
  test: "true"
steps:
  - name: fail-step
    command: sh -c 'echo fail >&2; exit 1'
`,
		withWorkerCount(0),
		withStaleThresholds(testStaleHeartbeatThreshold, testStaleLeaseThreshold),
		withZombieDetectionInterval(testZombieDetectorInterval),
	)
	defer f.cleanup()

	workerCmd, _ := startWorkerProcess(t, f, "event-worker", "test=true")

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	firstStatus := f.waitForStatus(core.Failed, 20*time.Second)
	require.Equal(t, core.Failed, firstStatus.Status)

	secondDAG := f.coord.DAG(t, `
type: graph
name: distributed-stale-repair
worker_selector:
  test: "true"
steps:
  - name: sleep-step
    command: sleep 300
`)
	f.dagWrapper = &secondDAG

	require.NoError(t, f.enqueue())
	f.waitForQueued()

	running := f.waitForStatus(core.Running, 20*time.Second)
	require.Equal(t, "event-worker", running.WorkerID)
	require.NotEmpty(t, running.AttemptKey)

	require.NoError(t, cmdutil.KillProcessGroup(workerCmd, os.Kill))

	secondStatus := f.waitForStatus(core.Failed, 20*time.Second)
	require.Equal(t, core.Failed, secondStatus.Status)

	require.Eventually(t, func() bool {
		result, err := f.coord.EventFeedService.Query(context.Background(), eventfeed.QueryFilter{
			Type:  eventfeed.EventTypeFailed,
			Limit: 20,
		})
		if err != nil {
			return false
		}

		var sawWorkerFailure bool
		var sawStaleRepair bool
		for _, entry := range result.Entries {
			if entry.DAGName == "distributed-worker-failure" && entry.DAGRunID == firstStatus.DAGRunID {
				sawWorkerFailure = true
			}
			if entry.DAGName == "distributed-stale-repair" && entry.DAGRunID == secondStatus.DAGRunID {
				sawStaleRepair = true
			}
		}
		return sawWorkerFailure && sawStaleRepair
	}, 20*time.Second, 100*time.Millisecond)
}
