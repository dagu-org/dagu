// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/require"
)

func TestAgentStatusPreservesSeedMetadata(t *testing.T) {
	t.Parallel()

	seed := &exec.DAGRunStatus{
		TriggerType:   core.TriggerTypeScheduler,
		CreatedAt:     12345,
		QueuedAt:      "2026-03-12T00:00:00Z",
		ScheduledTime: "2026-03-12T00:01:00Z",
	}

	a := New(
		"run-1",
		&core.DAG{Name: "critical-dag"},
		"",
		"",
		runtime.Manager{},
		nil,
		Options{
			TriggerType:   core.TriggerTypeUnknown,
			ScheduledTime: "2026-03-12T00:02:00Z",
			StatusSeed:    seed,
		},
	)

	status := a.Status(context.Background())
	require.Equal(t, core.TriggerTypeScheduler, status.TriggerType)
	require.Equal(t, int64(12345), status.CreatedAt)
	require.Equal(t, "2026-03-12T00:00:00Z", status.QueuedAt)
	require.Equal(t, "2026-03-12T00:02:00Z", status.ScheduledTime)
}

func TestAgentStatusFallsBackToRetryTargetMetadata(t *testing.T) {
	t.Parallel()

	retryTarget := &exec.DAGRunStatus{
		TriggerType:   core.TriggerTypeManual,
		CreatedAt:     54321,
		QueuedAt:      "2026-03-12T00:03:00Z",
		ScheduledTime: "2026-03-12T00:04:00Z",
	}

	a := New(
		"run-2",
		&core.DAG{Name: "critical-dag"},
		"",
		"",
		runtime.Manager{},
		nil,
		Options{
			RetryTarget: retryTarget,
			TriggerType: core.TriggerTypeRetry,
		},
	)

	status := a.Status(context.Background())
	require.Equal(t, core.TriggerTypeRetry, status.TriggerType)
	require.Equal(t, int64(54321), status.CreatedAt)
	require.Equal(t, "2026-03-12T00:03:00Z", status.QueuedAt)
	require.Equal(t, "2026-03-12T00:04:00Z", status.ScheduledTime)
}
