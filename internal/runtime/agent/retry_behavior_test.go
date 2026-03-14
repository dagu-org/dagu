// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
)

func TestAgentCurrentRetryCount(t *testing.T) {
	t.Parallel()

	t.Run("InitialAttemptStartsAtZero", func(t *testing.T) {
		t.Parallel()
		a := &Agent{}
		assert.Equal(t, 0, a.currentRetryCount())
	})

	t.Run("RetryTargetIncrementsPersistedCount", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			retryTarget: &exec.DAGRunStatus{RetryCount: 1},
		}
		assert.Equal(t, 2, a.currentRetryCount())
	})

	t.Run("FailureFinalizationUsesExistingCount", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			failureFinalizationTarget: &exec.DAGRunStatus{RetryCount: 2},
			retryTarget:               &exec.DAGRunStatus{RetryCount: 99},
		}
		assert.Equal(t, 2, a.currentRetryCount())
	})
}

func TestAgentShouldDeferFailureHandling(t *testing.T) {
	t.Parallel()

	baseDAG := &core.DAG{
		RetryPolicy: &core.DAGRetryPolicy{Limit: 2},
	}

	t.Run("DefersForTopLevelRunWithRetryBudget", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                baseDAG,
			retryFailureWindow: 24 * time.Hour,
		}
		assert.True(t, a.shouldDeferFailureHandling())
	})

	t.Run("DoesNotDeferWhenWindowDisabled", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag: baseDAG,
		}
		assert.False(t, a.shouldDeferFailureHandling())
	})

	t.Run("DoesNotDeferWithoutPolicy", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                &core.DAG{},
			retryFailureWindow: 24 * time.Hour,
		}
		assert.False(t, a.shouldDeferFailureHandling())
	})

	t.Run("DoesNotDeferForSubDAGRuns", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                baseDAG,
			parentDAGRun:       exec.NewDAGRunRef("parent", "run-1"),
			retryFailureWindow: 24 * time.Hour,
		}
		assert.False(t, a.shouldDeferFailureHandling())
	})

	t.Run("DoesNotDeferDuringFailureFinalization", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                       baseDAG,
			failureFinalizationTarget: &exec.DAGRunStatus{RetryCount: 0},
			retryFailureWindow:        24 * time.Hour,
		}
		assert.False(t, a.shouldDeferFailureHandling())
	})

	t.Run("DoesNotDeferAfterRetryBudgetExhausted", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                &core.DAG{RetryPolicy: &core.DAGRetryPolicy{Limit: 1}},
			retryTarget:        &exec.DAGRunStatus{RetryCount: 0},
			retryFailureWindow: 24 * time.Hour,
		}
		assert.False(t, a.shouldDeferFailureHandling())
	})
}

func TestAgentShouldMarkFailureFinalized(t *testing.T) {
	t.Parallel()

	t.Run("NonFailedStatusesNeverFinalize", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                &core.DAG{RetryPolicy: &core.DAGRetryPolicy{Limit: 2}},
			retryFailureWindow: 24 * time.Hour,
		}
		assert.False(t, a.shouldMarkFailureFinalized(core.Succeeded))
	})

	t.Run("DeferredFailuresRemainUnfinalized", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                &core.DAG{RetryPolicy: &core.DAGRetryPolicy{Limit: 2}},
			retryFailureWindow: 24 * time.Hour,
		}
		assert.False(t, a.shouldMarkFailureFinalized(core.Failed))
	})

	t.Run("TerminalFailuresFinalizeImmediately", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                &core.DAG{RetryPolicy: &core.DAGRetryPolicy{Limit: 1}},
			retryTarget:        &exec.DAGRunStatus{RetryCount: 0},
			retryFailureWindow: 24 * time.Hour,
		}
		assert.True(t, a.shouldMarkFailureFinalized(core.Failed))
	})

	t.Run("FailureFinalizationTargetAlwaysFinalizes", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			dag:                       &core.DAG{RetryPolicy: &core.DAGRetryPolicy{Limit: 2}},
			failureFinalizationTarget: &exec.DAGRunStatus{RetryCount: 1},
			retryFailureWindow:        24 * time.Hour,
		}
		assert.True(t, a.shouldMarkFailureFinalized(core.Failed))
	})
}
