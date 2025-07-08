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

func TestRetryPolicy_WithExponentialBackoff(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with retry backoff
	dag := th.DAG(t, filepath.Join("integration", "retry-with-backoff.yaml"))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	// Run the DAG (it will fail due to retry limit)
	_ = agent.Run(ctx)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify the step failed after retries
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, scheduler.NodeStatusError, status.Nodes[0].Status)
	assert.Equal(t, "failing-step", status.Nodes[0].Step.Name)

	// Verify it retried exactly 3 times
	assert.Equal(t, 3, status.Nodes[0].RetryCount, "Step should have retried exactly 3 times")

	// Verify total time is approximately correct
	// Expected: initial + 1s + 2s + 4s = 7s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 7*time.Second, "Total time should be at least 7 seconds")
}

func TestRetryPolicy_WithBackoffBoolean(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with boolean backoff
	dag := th.DAG(t, filepath.Join("integration", "retry-with-backoff-bool.yaml"))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	// Run the DAG (it will fail due to retry limit)
	_ = agent.Run(ctx)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify the step failed after retries
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, scheduler.NodeStatusError, status.Nodes[0].Status)
	assert.Equal(t, 3, status.Nodes[0].RetryCount)

	// Verify timing (backoff: true should use 2.0 multiplier)
	// Expected: initial + 1s + 2s + 4s = 7s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 7*time.Second, "Total time should be at least 7 seconds")
}

func TestRetryPolicy_WithMaxInterval(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with max interval cap
	dag := th.DAG(t, filepath.Join("integration", "retry-with-backoff-max.yaml"))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	// Run the DAG (it will fail due to retry limit)
	_ = agent.Run(ctx)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify it retried 5 times
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, 5, status.Nodes[0].RetryCount)

	// Verify timing with max interval cap
	// backoff: 3.0, intervalSec: 1, maxIntervalSec: 5
	// Expected intervals: 1s, 3s, 5s (capped), 5s (capped), 5s (capped)
	// Total: initial + 1 + 3 + 5 + 5 + 5 = 19s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 19*time.Second, "Total time should be at least 19 seconds")
	// Should not exceed 25 seconds (with some tolerance)
	assert.LessOrEqual(t, totalTime, 25*time.Second, "Total time should not exceed 25 seconds")
}

func TestRepeatPolicy_WithExponentialBackoff(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with repeat backoff
	dag := th.DAG(t, filepath.Join("integration", "repeat-with-backoff.yaml"))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify it repeated exactly 4 times
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, scheduler.NodeStatusSuccess, status.Nodes[0].Status)
	assert.Equal(t, 4, status.Nodes[0].DoneCount, "Step should have executed exactly 4 times")

	// Verify timing
	// Expected intervals: initial, then 1s, 2s, 4s = 7s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 7*time.Second, "Total time should be at least 7 seconds")
}

func TestRepeatPolicy_WithMaxInterval(t *testing.T) {
	th := test.Setup(t, test.WithDAGsDir(test.TestdataPath(t, "integration")))

	// Load DAG with max interval cap
	dag := th.DAG(t, filepath.Join("integration", "repeat-with-backoff-max.yaml"))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, scheduler.StatusSuccess)

	// Get the latest status
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, status)

	// Verify it repeated exactly 5 times
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, 5, status.Nodes[0].DoneCount, "Step should have executed exactly 5 times")

	// Verify timing with max interval cap
	// backoff: 3.0, intervalSec: 1, maxIntervalSec: 5
	// Expected intervals: initial, then 1s, 3s, 5s (capped), 5s (capped)
	// Total: initial + 1 + 3 + 5 + 5 = 14s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 14*time.Second, "Total time should be at least 14 seconds")
	// Should not exceed 20 seconds (with some tolerance)
	assert.LessOrEqual(t, totalTime, 20*time.Second, "Total time should not exceed 20 seconds")
}
