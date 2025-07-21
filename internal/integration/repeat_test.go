package integration_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
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
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, "repeat-step", dagRunStatus.Nodes[0].Step.Name)

	// Verify it executed exactly 3 times (as per limit)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 3 times")
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
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)

	// Verify it stopped at the limit (5) even though condition would continue
	assert.Equal(t, 5, dagRunStatus.Nodes[0].DoneCount, "Step should have stopped at limit of 5")
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
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify it stopped at limit (3)
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount, "Step should have stopped at limit of 3")
}

func TestRepeatPolicy_BooleanModeWhileUnconditional(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with boolean repeat mode (should repeat while step succeeds, like unconditional while)
	dag := th.DAG(t, filepath.Join("integration", "repeat-while-unconditional.yaml"))
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, "repeat-step", dagRunStatus.Nodes[0].Step.Name)

	// Verify it executed exactly 3 times (as per limit)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 3 times")
}

func TestRepeatPolicy_UntilWithExitCode(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with until mode and exitCode (should repeat until step returns exit code 0)
	dag := th.DAG(t, filepath.Join("integration", "repeat-until-unconditional.yaml"))
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)

	// Verify it executed exactly 3 times (until it gets exit code 0)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 3 times until exit code 0")
}

func TestRepeatPolicy_BackwardCompatibilityTrue(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with repeat: true (should work as "while" mode)
	dag := th.DAG(t, filepath.Join("integration", "repeat-backward-compatibility-true.yaml"))
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, "repeat-step", dagRunStatus.Nodes[0].Step.Name)

	// Verify it executed exactly 4 times (as per limit, confirming repeat: true works)
	assert.Equal(t, 4, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 4 times")
}
