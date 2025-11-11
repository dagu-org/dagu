package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRetrySubDAGRun(t *testing.T) {
	// Get DAG path
	th := test.SetupCommand(t)

	th.CreateDAGFile(t, "parent.yaml", `
steps:
  - name: parent
    call: sub_1
    params: "PARAM=FOO"
`)

	th.CreateDAGFile(t, "sub_1.yaml", `
params: "PARAM=BAR"
steps:
  - name: sub_2
    call: sub_2
    params: "PARAM=$PARAM"
`)

	th.CreateDAGFile(t, "sub_2.yaml", `
params: "PARAM=BAZ"
steps:
  - name: sub_2
    command: echo "Hello, $PARAM"
`)

	dagRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", dagRunID, "parent"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"dag-run finished"},
	})

	// Update the sub_2 status to "failed" to simulate a retry
	// First, find the sub_2 dag-run ID to update its status
	ctx := context.Background()
	ref := execution.NewDAGRunRef("parent", dagRunID)
	parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	updateStatus := func(rec execution.DAGRunAttempt, dagRunStatus *execution.DAGRunStatus) {
		err = rec.Open(ctx)
		require.NoError(t, err)
		err = rec.Write(ctx, *dagRunStatus)
		require.NoError(t, err)
		err = rec.Close(ctx)
		require.NoError(t, err)
	}

	// (1) Find the sub_1 node and update its status to "failed"
	parentStatus, err := parentAttempt.ReadStatus(ctx)
	require.NoError(t, err)

	sub1Node := parentStatus.Nodes[0]
	sub1Node.Status = core.NodeFailed
	updateStatus(parentAttempt, parentStatus)

	// (2) Find the sub_1 dag-run ID to update its status
	sub1Attempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, sub1Node.SubRuns[0].DAGRunID)
	require.NoError(t, err)

	sub1Status, err := sub1Attempt.ReadStatus(ctx)
	require.NoError(t, err)

	// (3) Find the sub_2 node and update its status to "failed"
	sub2Node := sub1Status.Nodes[0]
	sub2Node.Status = core.NodeFailed
	updateStatus(sub1Attempt, sub1Status)

	// (4) Find the sub_2 dag-run ID to update its status
	sub2Attempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, sub2Node.SubRuns[0].DAGRunID)
	require.NoError(t, err)

	sub2Status, err := sub2Attempt.ReadStatus(ctx)
	require.NoError(t, err)

	require.Equal(t, core.NodeSucceeded.String(), sub2Status.Status.String())

	// (5) Update the step in sub_2 to "failed" to simulate a retry
	sub2Status.Nodes[0].Status = core.NodeFailed
	updateStatus(sub2Attempt, sub2Status)

	// (6) Check if the sub_2 status is now "failed"
	sub2Status, err = sub2Attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeFailed.String(), sub2Status.Nodes[0].Status.String())

	// Retry the DAG

	args = []string{"retry", "--run-id", dagRunID, "parent"}
	th.RunCommand(t, cmd.Retry(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"dag-run finished"},
	})

	// Check if the sub_2 status is now "success"
	sub2Attempt, err = th.DAGRunStore.FindSubAttempt(ctx, ref, sub2Node.SubRuns[0].DAGRunID)
	require.NoError(t, err)
	sub2Status, err = sub2Attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeSucceeded.String(), sub2Status.Nodes[0].Status.String())

	require.Equal(t, "parent", sub2Status.Root.Name, "parent")
	require.Equal(t, dagRunID, sub2Status.Root.ID)
}

func TestRetryPolicySubDAGRunWithOutputCapture(t *testing.T) {
	th := test.SetupCommand(t)

	dagRunID := uuid.Must(uuid.NewV7()).String()

	// Generate a unique counter file path for this test
	counterFile := filepath.Join("/tmp", "retry_counter_"+dagRunID)

	th.CreateDAGFile(t, "parent_retry.yaml", `
steps:
  - name: call_sub
    call: sub_retry
    output: SUB_OUTPUT
`)

	th.CreateDAGFile(t, "sub_retry.yaml", `
steps:
  - name: retry_step
    command: |
      COUNTER_FILE="`+counterFile+`"
      if [ ! -f "$COUNTER_FILE" ]; then
        echo "1" > "$COUNTER_FILE"
        echo "output_attempt_1"
        exit 1
      else
        COUNT=$(cat "$COUNTER_FILE")
        if [ "$COUNT" -eq "1" ]; then
          echo "2" > "$COUNTER_FILE"
          echo "output_attempt_2_success"
          exit 0
        fi
      fi
    output: STEP_OUTPUT
    retryPolicy:
      limit: 2
      intervalSec: 1
`)
	args := []string{"start", "--run-id", dagRunID, "parent_retry"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"dag-run finished"},
	})

	// Verify parent DAG completed successfully
	ctx := context.Background()
	ref := execution.NewDAGRunRef("parent_retry", dagRunID)
	parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	parentStatus, err := parentAttempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeSucceeded.String(), parentStatus.Status.String())

	// Find sub DAG run
	subNode := parentStatus.Nodes[0]
	require.Equal(t, core.NodeSucceeded.String(), subNode.Status.String())

	subAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, subNode.SubRuns[0].DAGRunID)
	require.NoError(t, err)

	subStatus, err := subAttempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeSucceeded.String(), subStatus.Status.String())

	// Verify the step in sub DAG completed successfully after retry
	retryStep := subStatus.Nodes[0]
	require.Equal(t, core.NodeSucceeded.String(), retryStep.Status.String())

	// Verify output was captured from the successful retry attempt
	require.NotNil(t, retryStep.OutputVariables, "OutputVariables should not be nil")
	variables := retryStep.OutputVariables.Variables()
	t.Logf("Retry step output variables: %+v", variables)
	t.Logf("Retry step status: %s", retryStep.Status.String())
	t.Logf("Retry step retry count: %d", retryStep.RetryCount)
	require.Contains(t, variables, "STEP_OUTPUT", "Output variable STEP_OUTPUT should exist")
	require.Contains(t, variables["STEP_OUTPUT"], "output_attempt_2_success", "Output should contain success message from retry")
}

func TestBasicSubDAGOutputCapture(t *testing.T) {
	th := test.SetupCommand(t)

	th.CreateDAGFile(t, "parent_basic.yaml", `
steps:
  - name: call_sub
    call: sub_basic
    output: SUB_OUTPUT
`)

	th.CreateDAGFile(t, "sub_basic.yaml", `
steps:
  - name: basic_step
    command: echo "hello_from_sub"
    output: STEP_OUTPUT
`)

	dagRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", dagRunID, "parent_basic"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"dag-run finished"},
	})

	// Verify parent DAG completed successfully
	ctx := context.Background()
	ref := execution.NewDAGRunRef("parent_basic", dagRunID)
	parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	parentStatus, err := parentAttempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeSucceeded.String(), parentStatus.Status.String())

	// Find sub DAG run
	subNode := parentStatus.Nodes[0]
	require.Equal(t, core.NodeSucceeded.String(), subNode.Status.String())

	subAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, subNode.SubRuns[0].DAGRunID)
	require.NoError(t, err)

	subStatus, err := subAttempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeSucceeded.String(), subStatus.Status.String())

	// Verify the step in sub DAG completed successfully
	basicStep := subStatus.Nodes[0]
	require.Equal(t, core.NodeSucceeded.String(), basicStep.Status.String())

	// Debug: Print all output variables
	if basicStep.OutputVariables != nil {
		variables := basicStep.OutputVariables.Variables()
		t.Logf("Output variables: %+v", variables)
		require.Contains(t, variables, "STEP_OUTPUT", "Output variable STEP_OUTPUT should exist")
		require.Contains(t, variables["STEP_OUTPUT"], "hello_from_sub", "Output should contain expected text")
	} else {
		t.Logf("OutputVariables is nil")
		require.Fail(t, "OutputVariables should not be nil")
	}
}

func TestRetryPolicyBasicOutputCapture(t *testing.T) {
	th := test.SetupCommand(t)

	dagRunID := uuid.Must(uuid.NewV7()).String()
	counterFile := filepath.Join("/tmp", "retry_counter_basic_"+dagRunID)
	defer func() {
		// Clean up the counter file after the test
		_ = os.Remove(counterFile)
	}()

	th.CreateDAGFile(t, "basic_retry.yaml", `
steps:
  - name: retry_step
    command: |
      COUNTER_FILE="`+counterFile+`"
      if [ ! -f "$COUNTER_FILE" ]; then
        echo "1" > "$COUNTER_FILE"
        echo "output_attempt_1"
        exit 1
      else
        COUNT=$(cat "$COUNTER_FILE")
        if [ "$COUNT" -eq "1" ]; then
          echo "2" > "$COUNTER_FILE"
          echo "output_attempt_2_success"
          exit 0
        fi
      fi
    output: STEP_OUTPUT
    retryPolicy:
      limit: 2
      intervalSec: 1
`)

	args := []string{"start", "--run-id", dagRunID, "basic_retry"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"dag-run finished"},
	})

	// Verify DAG completed successfully
	ctx := context.Background()
	ref := execution.NewDAGRunRef("basic_retry", dagRunID)
	attempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	dagRunStatus, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeSucceeded.String(), dagRunStatus.Status.String())

	// Verify the step completed successfully after retry
	retryStep := dagRunStatus.Nodes[0]
	require.Equal(t, core.NodeSucceeded.String(), retryStep.Status.String())

	// Debug retry output
	require.NotNil(t, retryStep.OutputVariables, "OutputVariables should not be nil")
	variables := retryStep.OutputVariables.Variables()
	t.Logf("Basic retry step output variables: %+v", variables)
	t.Logf("Basic retry step status: %s", retryStep.Status.String())
	t.Logf("Basic retry step retry count: %d", retryStep.RetryCount)

	// Check if counter file was created and what it contains
	counterFileContent, err := os.ReadFile(counterFile)
	if err != nil {
		t.Logf("Counter file error: %v", err)
	} else {
		t.Logf("Counter file content: %s", string(counterFileContent))
	}

	// This is the test - does retry capture output properly?
	require.Contains(t, variables, "STEP_OUTPUT", "Output variable STEP_OUTPUT should exist")
	require.Contains(t, variables["STEP_OUTPUT"], "output_attempt_2_success", "Output should contain success message from retry")
}

func TestNoRetryPolicyOutputCapture(t *testing.T) {
	th := test.SetupCommand(t)

	th.CreateDAGFile(t, "no_retry.yaml", `
steps:
  - name: success_step
    command: echo "output_first_attempt_success"
    output: STEP_OUTPUT
`)

	dagRunID := uuid.Must(uuid.NewV7()).String()
	args := []string{"start", "--run-id", dagRunID, "no_retry"}
	th.RunCommand(t, cmd.Start(), test.CmdTest{
		Args:        args,
		ExpectedOut: []string{"dag-run finished"},
	})

	// Verify DAG completed successfully
	ctx := context.Background()
	ref := execution.NewDAGRunRef("no_retry", dagRunID)
	attempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
	require.NoError(t, err)

	dagRunStatus, err := attempt.ReadStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, core.NodeSucceeded.String(), dagRunStatus.Status.String())

	// Verify the step completed successfully on first attempt
	successStep := dagRunStatus.Nodes[0]
	require.Equal(t, core.NodeSucceeded.String(), successStep.Status.String())

	// Debug output for first attempt success
	require.NotNil(t, successStep.OutputVariables, "OutputVariables should not be nil")
	variables := successStep.OutputVariables.Variables()
	t.Logf("No retry step output variables: %+v", variables)
	t.Logf("No retry step status: %s", successStep.Status.String())
	t.Logf("No retry step retry count: %d", successStep.RetryCount)

	// This should work - first attempt success captures output
	require.Contains(t, variables, "STEP_OUTPUT", "Output variable STEP_OUTPUT should exist")
	require.Contains(t, variables["STEP_OUTPUT"], "output_first_attempt_success", "Output should contain success message from first attempt")
}
