package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryPolicy_WithExponentialBackoff(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with retry backoff
	dag := th.DAGWithYAML(t, "retry-with-backoff", []byte(`
name: retry-with-backoff
steps:
  - name: failing-step
    command: |
      echo "Attempt at $(date +%s.%N)"
      exit 1
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: 2.0
      exitCode: [1]
`))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	// Run the DAG (it will fail due to retry limit)
	_ = agent.Run(ctx)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step failed after retries
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeError, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, "failing-step", dagRunStatus.Nodes[0].Step.Name)

	// Verify it retried exactly 3 times
	assert.Equal(t, 3, dagRunStatus.Nodes[0].RetryCount, "Step should have retried exactly 3 times")

	// Verify total time is approximately correct
	// Expected: initial + 1s + 2s + 4s = 7s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 7*time.Second, "Total time should be at least 7 seconds")
}

func TestRetryPolicy_WithBackoffBoolean(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with boolean backoff
	dag := th.DAGWithYAML(t, "retry-with-backoff-bool", []byte(`
name: retry-with-backoff-bool
steps:
  - name: failing-step
    command: |
      echo "Attempt at $(date +%s.%N)"
      exit 1
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: true  # Should use default 2.0 multiplier
      exitCode: [1]
`))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	// Run the DAG (it will fail due to retry limit)
	_ = agent.Run(ctx)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step failed after retries
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeError, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].RetryCount)

	// Verify timing (backoff: true should use 2.0 multiplier)
	// Expected: initial + 1s + 2s + 4s = 7s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 7*time.Second, "Total time should be at least 7 seconds")
}

func TestRetryPolicy_WithMaxInterval(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with max interval cap
	dag := th.DAGWithYAML(t, "retry-with-backoff-max", []byte(`
name: retry-with-backoff-max
steps:
  - name: failing-step
    command: |
      echo "Attempt at $(date +%s.%N)"
      exit 1
    retryPolicy:
      limit: 5
      intervalSec: 1
      backoff: 3.0
      maxIntervalSec: 5  # Cap at 5 seconds
      exitCode: [1]
`))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	// Run the DAG (it will fail due to retry limit)
	_ = agent.Run(ctx)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify it retried 5 times
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, 5, dagRunStatus.Nodes[0].RetryCount)

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
	th := test.Setup(t)

	// Load DAG with repeat backoff
	dag := th.DAGWithYAML(t, "repeat-with-backoff", []byte(`
name: repeat-with-backoff
steps:
  - name: repeat-step
    command: |
      echo "Execution at $(date +%s.%N)"
      exit 0
    repeatPolicy:
      repeat: while
      limit: 4
      intervalSec: 1
      backoff: 2.0
      exitCode: [0]  # Repeat while exit code is 0
`))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify it repeated exactly 4 times
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, 4, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 4 times")

	// Verify timing
	// Expected intervals: initial, then 1s, 2s, 4s = 7s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 7*time.Second, "Total time should be at least 7 seconds")
}

func TestRepeatPolicy_WithMaxInterval(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with max interval cap
	dag := th.DAGWithYAML(t, "repeat-with-backoff-max", []byte(`
name: repeat-with-backoff-max
steps:
  - name: repeat-step
    command: |
      echo "Execution at $(date +%s.%N)"
      exit 0
    repeatPolicy:
      repeat: while
      limit: 5
      intervalSec: 1
      backoff: 3.0
      maxIntervalSec: 5  # Cap at 5 seconds
      exitCode: [0]  # Repeat while exit code is 0
`))
	agent := dag.Agent()

	// Record start time
	startTime := time.Now()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 30*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, status.Success)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify it repeated exactly 5 times
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, 5, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 5 times")

	// Verify timing with max interval cap
	// backoff: 3.0, intervalSec: 1, maxIntervalSec: 5
	// Expected intervals: initial, then 1s, 3s, 5s (capped), 5s (capped)
	// Total: initial + 1 + 3 + 5 + 5 = 14s minimum
	totalTime := time.Since(startTime)
	assert.GreaterOrEqual(t, totalTime, 14*time.Second, "Total time should be at least 14 seconds")
	// Should not exceed 20 seconds (with some tolerance)
	assert.LessOrEqual(t, totalTime, 20*time.Second, "Total time should not exceed 20 seconds")
}
