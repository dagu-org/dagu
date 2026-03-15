// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package distr_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/require"
)

const (
	distrTestProcHeartbeatInterval = 150 * time.Millisecond
	distrTestProcStaleThreshold    = 3 * time.Second
)

func TestExecution_ProcHeartbeat_DirectStart(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: worker-proc-heartbeat-start
worker_selector:
  test: "true"
steps:
  - name: sleep
    command: sleep 2
`, withProcConfig(distrTestProcHeartbeatInterval, distrTestProcHeartbeatInterval, distrTestProcStaleThreshold))
	defer f.cleanup()

	f.startScheduler(30 * time.Second)
	require.NoError(t, f.start())

	status := f.waitForStatus(core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)
}

func TestExecution_ProcHeartbeat_QueuedDispatch(t *testing.T) {
	f := newTestFixture(t, `
type: graph
name: worker-proc-heartbeat-queued
worker_selector:
  test: "true"
steps:
  - name: sleep
    command: sleep 2
`, withProcConfig(distrTestProcHeartbeatInterval, distrTestProcHeartbeatInterval, distrTestProcStaleThreshold))
	defer f.cleanup()

	require.NoError(t, f.enqueue())
	f.waitForQueued()
	f.startScheduler(30 * time.Second)

	status := f.waitForStatus(core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)
}
