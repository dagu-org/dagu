// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"testing"
	"time"

	agentpkg "github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/stretchr/testify/assert"
)

func TestDAGAgentRetryFailureWindow(t *testing.T) {
	t.Parallel()

	t.Run("DefaultsFromConfigWhenNotOverridden", func(t *testing.T) {
		t.Parallel()

		th := Setup(t)
		th.Config.Scheduler.RetryFailureWindow = 6 * time.Hour
		dag := th.DAG(t, `steps:
  - "true"
`)

		agent := dag.Agent()

		assert.Equal(t, 6*time.Hour, agent.opts.RetryFailureWindow)
		assert.False(t, agent.retryFailureWindowSet)
	})

	t.Run("AllowsExplicitZeroOverride", func(t *testing.T) {
		t.Parallel()

		th := Setup(t)
		th.Config.Scheduler.RetryFailureWindow = 6 * time.Hour
		dag := th.DAG(t, `steps:
  - "true"
`)

		agent := dag.Agent(WithRetryFailureWindow(0))

		assert.Zero(t, agent.opts.RetryFailureWindow)
		assert.True(t, agent.retryFailureWindowSet)
	})

	t.Run("PreservesNonZeroWithAgentOptions", func(t *testing.T) {
		t.Parallel()

		th := Setup(t)
		th.Config.Scheduler.RetryFailureWindow = 6 * time.Hour
		dag := th.DAG(t, `steps:
  - "true"
`)

		agent := dag.Agent(WithAgentOptions(agentpkg.Options{
			RetryFailureWindow: 2 * time.Hour,
		}))

		assert.Equal(t, 2*time.Hour, agent.opts.RetryFailureWindow)
		assert.True(t, agent.retryFailureWindowSet)
	})
}
