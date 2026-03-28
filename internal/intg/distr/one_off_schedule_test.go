// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package distr_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/stretchr/testify/require"
)

func TestOneOffScheduleRunsDistributed(t *testing.T) {
	scheduledAt := time.Date(2026, 3, 29, 2, 10, 0, 0, time.UTC)
	armedAt := scheduledAt.Add(-time.Minute)

	f := newTestFixture(t, `
name: distributed-one-off-test
schedule:
  start:
    - at: "`+scheduledAt.Format(time.RFC3339)+`"
worker_selector:
  test: "true"
steps:
  - name: echo-step
    command: echo "distributed-one-off"
`)
	defer f.cleanup()

	f.coord.Config.Scheduler.RetryFailureWindow = 0

	var callCount atomic.Int32
	f.startSchedulerWithClock(30*time.Second, func() time.Time {
		if callCount.Add(1) <= 2 {
			return armedAt
		}
		return scheduledAt
	})

	status := f.waitForStatus(core.Succeeded, 20*time.Second)

	oneOffSchedule, err := core.NewOneOffSchedule(scheduledAt.Format(time.RFC3339))
	require.NoError(t, err)

	require.Equal(
		t,
		scheduler.GenerateOneOffRunID(f.dagWrapper.Name, oneOffSchedule.Fingerprint(), scheduledAt),
		status.DAGRunID,
	)
	require.Equal(t, stringutil.FormatTime(scheduledAt), status.ScheduleTime)
	f.assertWorkerID(status, "worker-1")
	f.assertAllNodesSucceeded(status)
}
