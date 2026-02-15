package intg_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeatPolicy_WithLimit(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with repeat limit
	dag := th.DAG(t, `steps:
  - command: echo "Executing step"
    repeat_policy:
      repeat: true
      limit: 3
      interval_sec: 0
`)
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, core.Succeeded)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

	// Verify it executed exactly 3 times (as per limit)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 3 times")
}

func TestRepeatPolicy_WithLimitAndCondition(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with repeat limit and condition
	dag := th.DAG(t, `steps:
  - script: |
      COUNTER_FILE="/tmp/boltbase_repeat_counter_test2"
      COUNT=0
      if [ -f "$COUNTER_FILE" ]; then
        COUNT=$(cat "$COUNTER_FILE")
      fi
      COUNT=$((COUNT + 1))
      echo "$COUNT" > "$COUNTER_FILE"
      echo "Count: $COUNT"
      # Output the count so we can verify in test
      echo "$COUNT"
      # Clean up on final run
      if [ "$COUNT" -ge 5 ]; then
        rm -f "$COUNTER_FILE"
      fi
    output: FINAL_COUNT
    repeat_policy:
      repeat: until
      limit: 5
      interval_sec: 0
      condition: "`+"`"+`[ -f /tmp/boltbase_repeat_counter_test2 ] && cat /tmp/boltbase_repeat_counter_test2 || echo 0`+"`"+`"
      expected: "10"
`)
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, core.Succeeded)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

	// Verify it stopped at the limit (5) even though condition would continue
	assert.Equal(t, 5, dagRunStatus.Nodes[0].DoneCount, "Step should have stopped at limit of 5")
}

func TestRepeatPolicy_WithLimitReachedBeforeCondition(t *testing.T) {
	th := test.Setup(t)

	// Load DAG that repeats with a limit
	dag := th.DAG(t, `steps:
  - command: echo "Checking for flag file"
    repeat_policy:
      repeat: true
      limit: 3
      interval_sec: 0
`)
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, core.Succeeded)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify it stopped at limit (3)
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount, "Step should have stopped at limit of 3")
}

func TestRepeatPolicy_BooleanModeWhileUnconditional(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with boolean repeat mode (should repeat while step succeeds, like unconditional while)
	dag := th.DAG(t, `steps:
  - command: echo "Unconditional while loop using boolean mode"
    repeat_policy:
      repeat: true
      limit: 3
      interval_sec: 0
`)
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, core.Succeeded)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

	// Verify it executed exactly 3 times (as per limit)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 3 times")
}

func TestRepeatPolicy_UntilWithExitCode(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with until mode and exitCode (should repeat until step returns exit code 0)
	dag := th.DAG(t, `steps:
  - script: |
      COUNT_FILE="/tmp/boltbase_repeat_until_unconditional_test"
      COUNT=0
      if [ -f "$COUNT_FILE" ]; then
        COUNT=$(cat "$COUNT_FILE")
      fi
      COUNT=$((COUNT + 1))
      echo "$COUNT" > "$COUNT_FILE"
      echo "Count: $COUNT"
      if [ "$COUNT" -le 2 ]; then
        exit 1
      else
        rm -f "$COUNT_FILE"
        exit 0
      fi
    repeat_policy:
      # Using backward compatibility mode: exitCode only infers "while" mode
      # but we can test "until" behavior with explicit condition that inverts logic
      repeat: "until"
      exit_code: [0]  # Repeat until we get exit code 0
      interval_sec: 0
    continue_on:
      exit_code: [1]
`)
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, core.Succeeded)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

	// Verify it executed exactly 3 times (until it gets exit code 0)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 3 times until exit code 0")
}

func TestRepeatPolicy_BackwardCompatibilityTrue(t *testing.T) {
	th := test.Setup(t)

	// Load DAG with repeat: true (should work as "while" mode)
	dag := th.DAG(t, `steps:
  - command: echo "Boolean true compatibility test"
    repeat_policy:
      repeat: true
      limit: 4
      interval_sec: 0
`)
	agent := dag.Agent()

	// Run with timeout
	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	// Verify successful completion
	dag.AssertLatestStatus(t, core.Succeeded)

	// Get the latest status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	// Verify the step completed successfully
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

	// Verify it executed exactly 4 times (as per limit, confirming repeat: true works)
	assert.Equal(t, 4, dagRunStatus.Nodes[0].DoneCount, "Step should have executed exactly 4 times")
}

func TestRepeatPolicy_OnExitCode(t *testing.T) {
	counterFile := "/tmp/boltbase-test-counter-repeat-on-exitcode"
	_ = os.Remove(counterFile)
	t.Cleanup(func() {
		_ = os.Remove(counterFile)
	})

	th := test.Setup(t)

	dag := th.DAG(t, `steps:
  - command: |
      #!/bin/bash
      COUNTER_FILE="/tmp/boltbase-test-counter-repeat-on-exitcode"
      if [ ! -f "$COUNTER_FILE" ]; then
          echo 1 > "$COUNTER_FILE"
          exit 1
      fi

      count=$(cat "$COUNTER_FILE")
      if [ "$count" -lt 3 ]; then
          echo $((count + 1)) > "$COUNTER_FILE"
          exit 1
      else
          echo $((count + 1)) > "$COUNTER_FILE"
          exit 0
      fi
    repeat_policy:
      exit_code: [1]
      limit: 5
      interval_sec: 0.1
`)
	agent := dag.Agent()

	ctx, cancel := context.WithTimeout(agent.Context, 15*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err, "DAG should complete successfully")

	dag.AssertLatestStatus(t, core.Succeeded)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.NotNil(t, dagRunStatus)

	require.Len(t, dagRunStatus.Nodes, 1)
	nodeStatus := dagRunStatus.Nodes[0]

	assert.Equal(t, core.NodeSucceeded, nodeStatus.Status, "The final status of the node should be Success")
	assert.True(t, nodeStatus.Repeated, "The step should be marked as repeated")
	assert.GreaterOrEqual(t, nodeStatus.DoneCount, 3, "The step should have executed at least 3 times")
}
