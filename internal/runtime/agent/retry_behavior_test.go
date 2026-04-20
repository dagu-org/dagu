// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentCurrentAutoRetryCount(t *testing.T) {
	t.Parallel()

	t.Run("InitialAttemptStartsAtZero", func(t *testing.T) {
		t.Parallel()
		a := &Agent{}
		assert.Equal(t, 0, a.currentAutoRetryCount())
	})

	t.Run("RetryTargetPreservesPersistedCount", func(t *testing.T) {
		t.Parallel()
		a := &Agent{
			retryTarget: &exec.DAGRunStatus{AutoRetryCount: 1},
		}
		assert.Equal(t, 1, a.currentAutoRetryCount())
	})
}

func TestAgentSetupRetryPlanQueuedStatus(t *testing.T) {
	t.Parallel()

	t.Run("EmptyNodesUsesFreshPlan", func(t *testing.T) {
		t.Parallel()

		a := &Agent{
			dag: &core.DAG{Steps: []core.Step{
				{Name: "build", Command: "echo build"},
			}},
			retryTarget: &exec.DAGRunStatus{Status: core.Queued},
		}

		require.NoError(t, a.setupRetryPlan(context.Background()))
		require.NotNil(t, a.plan)
		require.Equal(t, core.NodeNotStarted, a.plan.GetNodeByName("build").Status())
	})

	t.Run("PersistedNodesUseRetryPlan", func(t *testing.T) {
		t.Parallel()

		a := &Agent{
			dag: &core.DAG{Steps: []core.Step{
				{Name: "build", Command: "echo build"},
				{Name: "consume", Command: "echo consume", Depends: []string{"build"}},
			}},
			retryTarget: &exec.DAGRunStatus{
				Status: core.Queued,
				Nodes: []*exec.Node{
					{
						Step:   core.Step{Name: "build", Command: "exit 1"},
						Status: core.NodeSkipped,
					},
					{
						Step:   core.Step{Name: "consume", Command: "echo old", Depends: []string{"build"}},
						Status: core.NodeFailed,
					},
				},
			},
		}

		require.NoError(t, a.setupRetryPlan(context.Background()))
		require.NotNil(t, a.plan)
		require.Equal(t, core.NodeSkipped, a.plan.GetNodeByName("build").Status())
		require.Equal(t, core.NodeNotStarted, a.plan.GetNodeByName("consume").Status())
		require.Equal(t, "echo consume", a.plan.GetNodeByName("consume").Step().Command)
	})
}
