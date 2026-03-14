// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"testing"

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
}
