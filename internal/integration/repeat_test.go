package integration_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeatPolicy_WithLimit(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with repeat limit
	dag := th.DAG(t, filepath.Join("integration", "repeat-with-limit.yaml"))
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify the step completed successfully
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, scheduler.NodeStatusSuccess, status.Nodes[0].Status)
	assert.Equal(t, "repeat-step", status.Nodes[0].Step.Name)

	// Verify it executed exactly 3 times (as per limit)
	assert.Equal(t, 3, status.Nodes[0].DoneCount, "Step should have executed exactly 3 times")
}

func TestRepeatPolicy_WithLimitAndCondition(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with repeat limit and condition
	dag := th.DAG(t, filepath.Join("integration", "repeat-limit-condition.yaml"))
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify the step completed successfully
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, scheduler.NodeStatusSuccess, status.Nodes[0].Status)

	// Verify it stopped at the limit (5) even though condition would continue
	assert.Equal(t, 5, status.Nodes[0].DoneCount, "Step should have stopped at limit of 5")
}

func TestRepeatPolicy_WithLimitReachedBeforeCondition(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG that repeats with a limit
	dag := th.DAG(t, filepath.Join("integration", "repeat-limit-before-condition.yaml"))
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify it stopped at limit (3)
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, scheduler.NodeStatusSuccess, status.Nodes[0].Status)
	assert.Equal(t, 3, status.Nodes[0].DoneCount, "Step should have stopped at limit of 3")
}
