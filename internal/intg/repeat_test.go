// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeatPolicy_WithLimit(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	th := test.Setup(t)

	counterFile := filepath.Join(t.TempDir(), "counter")

	// Load DAG with repeat limit and condition
	dag := th.DAG(t, fmt.Sprintf(`env:
  - COUNTER_FILE: %s
steps:
  - script: |
      COUNT=0
      if [ -f "$COUNTER_FILE" ]; then
        COUNT=$(cat "$COUNTER_FILE")
      fi
      COUNT=$((COUNT + 1))
      echo "$COUNT" > "$COUNTER_FILE"
      echo "Count: $COUNT"
      echo "$COUNT"
    output: FINAL_COUNT
    repeat_policy:
      repeat: until
      limit: 5
      interval_sec: 0
      condition: "`+"`"+`[ -f %s ] && cat %s || echo 0`+"`"+`"
      expected: "10"
`, counterFile, counterFile, counterFile))
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	th := test.Setup(t)

	counterFile := filepath.Join(t.TempDir(), "counter")

	dag := th.DAG(t, fmt.Sprintf(`env:
  - COUNTER_FILE: %s
steps:
  - script: |
      COUNT=0
      if [ -f "$COUNTER_FILE" ]; then
        COUNT=$(cat "$COUNTER_FILE")
      fi
      COUNT=$((COUNT + 1))
      echo "$COUNT" > "$COUNTER_FILE"
      echo "Count: $COUNT"
      if [ "$COUNT" -le 2 ]; then
        exit 1
      else
        rm -f "$COUNTER_FILE"
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
`, counterFile))
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
	t.Parallel()
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
	t.Parallel()
	th := test.Setup(t)

	counterFile := filepath.Join(t.TempDir(), "counter")

	dag := th.DAG(t, fmt.Sprintf(`env:
  - COUNTER_FILE: %s
steps:
  - command: |
      #!/bin/bash
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
      interval_sec: 0
`, counterFile))
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

func TestRepeatPolicy_LimitFromEnvVar(t *testing.T) {
	t.Setenv("REPEAT_LIMIT", "3")
	th := test.Setup(t)

	dag := th.DAG(t, `steps:
  - command: echo "repeating"
    repeat_policy:
      repeat: true
      limit: $REPEAT_LIMIT
      interval_sec: 0
`)
	agent := dag.Agent()

	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err)

	dag.AssertLatestStatus(t, core.Succeeded)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount)
}

func TestRepeatPolicy_IntervalSecFromEnvVar(t *testing.T) {
	t.Setenv("INTERVAL_SEC", "0")
	th := test.Setup(t)

	dag := th.DAG(t, `steps:
  - command: echo "repeating with env interval"
    repeat_policy:
      repeat: true
      limit: 3
      interval_sec: $INTERVAL_SEC
`)
	agent := dag.Agent()

	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err)

	dag.AssertLatestStatus(t, core.Succeeded)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount)
}

func TestRepeatPolicy_MaxIntervalSecFromEnvVar(t *testing.T) {
	t.Setenv("MAX_INTERVAL", "10")
	th := test.Setup(t)

	dag := th.DAG(t, `steps:
  - command: echo "repeating with env max interval"
    repeat_policy:
      repeat: true
      limit: 2
      interval_sec: 0
      max_interval_sec: $MAX_INTERVAL
`)
	agent := dag.Agent()

	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err)

	dag.AssertLatestStatus(t, core.Succeeded)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, 2, dagRunStatus.Nodes[0].DoneCount)
}

func TestRepeatPolicy_LimitFromCommandSubstitution(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	dag := th.DAG(t, "steps:\n  - command: echo \"repeating with cmd sub\"\n    repeat_policy:\n      repeat: true\n      limit: \"`echo 3`\"\n      interval_sec: 0\n")
	agent := dag.Agent()

	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err)

	dag.AssertLatestStatus(t, core.Succeeded)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, 3, dagRunStatus.Nodes[0].DoneCount)
}

func TestRepeatPolicy_MultipleDynamicFields(t *testing.T) {
	t.Setenv("DYN_LIMIT", "4")
	t.Setenv("DYN_INTERVAL", "0")
	th := test.Setup(t)

	dag := th.DAG(t, `steps:
  - command: echo "multiple dynamic fields"
    repeat_policy:
      repeat: true
      limit: $DYN_LIMIT
      interval_sec: $DYN_INTERVAL
`)
	agent := dag.Agent()

	ctx, cancel := context.WithTimeout(agent.Context, 10*time.Second)
	defer cancel()

	err := agent.Run(ctx)
	require.NoError(t, err)

	dag.AssertLatestStatus(t, core.Succeeded)

	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Len(t, dagRunStatus.Nodes, 1)
	assert.Equal(t, 4, dagRunStatus.Nodes[0].DoneCount)
}
