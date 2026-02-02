package intg_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestInlineSubDAG(t *testing.T) {
	t.Run("SimpleExecution", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: run-local-child
    call: local-child
    params: "NAME=World"
    output: SUB_RESULT

  - echo "Child said ${SUB_RESULT.outputs.GREETING}"

---

name: local-child
params:
  - NAME
steps:
  - command: echo "Hello, ${NAME}!"
    output: GREETING

  - echo "Greeting was ${GREETING}"
`)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 2)
		require.Equal(t, "run-local-child", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

		logContent, err := os.ReadFile(dagRunStatus.Nodes[1].Stdout)
		require.NoError(t, err)
		require.Contains(t, string(logContent), "Child said Hello, World!")
	})

	t.Run("TwoLevelNesting", func(t *testing.T) {
		th := test.Setup(t)

		dag := th.DAG(t, `
steps:
  - name: call_child
    call: child
    params: "MSG=hello"

---

name: child
params: "MSG=default"
steps:
  - name: echo_msg
    command: echo "${MSG}_from_child"
    output: RESULT
`)

		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
	})

	t.Run("ThreeLevelNesting", func(t *testing.T) {
		// 3-level nesting: root -> middle -> leaf
		th := test.Setup(t)

		dag := th.DAG(t, `
steps:
  - name: call_middle
    call: middle
    params: "MSG=hello"

---

name: middle
params: "MSG=default"
steps:
  - name: call_leaf
    call: leaf
    params: "MSG=${MSG}_middle"

---

name: leaf
params: "MSG=default"
steps:
  - name: echo_msg
    command: echo "${MSG}_from_leaf"
    output: RESULT
`)

		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
	})

	t.Run("ThreeLevelNestingWithOutputPassing", func(t *testing.T) {
		// middle-dag calls leaf-dag with parameter passing
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: run-middle-dag
    call: middle-dag
    params: "ROOT_PARAM=FromRoot"

---

name: middle-dag
params:
  - ROOT_PARAM
steps:
  - command: echo "Received ${ROOT_PARAM}"
    output: MIDDLE_OUTPUT

  - name: run-leaf-dag
    call: leaf-dag
    params: "MIDDLE_PARAM=${MIDDLE_OUTPUT} LEAF_PARAM=FromMiddle"

---

name: leaf-dag
params:
  - MIDDLE_PARAM
  - LEAF_PARAM
steps:
  - command: |
      echo "Middle: ${MIDDLE_PARAM}, Leaf: ${LEAF_PARAM}"
`)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "run-middle-dag", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)
		require.Len(t, dagRunStatus.Nodes[0].SubRuns, 1, "middle-dag should have one sub-run")
	})

	t.Run("ParallelExecution", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: parallel-tasks
    call: worker-dag
    parallel:
      items:
        - TASK_ID=1 TASK_NAME=alpha
        - TASK_ID=2 TASK_NAME=beta
        - TASK_ID=3 TASK_NAME=gamma
      maxConcurrent: 2

---

name: worker-dag
params:
  - TASK_ID
  - TASK_NAME
steps:
  - echo "Starting task ${TASK_ID} - ${TASK_NAME}"
  - echo "Processing ${TASK_NAME} with ID ${TASK_ID}"
  - echo "Completed ${TASK_NAME}"
`)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "parallel-tasks", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)
	})

	t.Run("ConditionalExecution", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
env:
  - ENVIRONMENT: production
steps:
  - name: check-env
    command: echo "${ENVIRONMENT}"
    output: ENV_TYPE

  - name: run-prod-dag
    call: production-dag
    preconditions:
      - condition: "${ENV_TYPE}"
        expected: "production"

  - name: run-dev-dag
    call: development-dag
    preconditions:
      - condition: "${ENV_TYPE}"
        expected: "development"

---

name: production-dag
steps:
  - echo "Deploying to production"
  - echo "Verifying production deployment"

---

name: development-dag
steps:
  - echo "Building for development"
  - echo "Running development tests"
`)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 3)
		require.Equal(t, "check-env", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)
		require.Equal(t, "run-prod-dag", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[1].Status)
		require.Equal(t, "run-dev-dag", dagRunStatus.Nodes[2].Step.Name)
		require.Equal(t, core.NodeSkipped, dagRunStatus.Nodes[2].Status)
	})

	t.Run("OutputPassingBetweenDAGs", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: generate-data
    call: generator-dag
    output: GEN_OUTPUT

  - name: process-data
    call: processor-dag
    params: "INPUT_DATA=${GEN_OUTPUT.outputs.DATA}"

---

name: generator-dag
steps:
  - command: echo "test-value-42"
    output: DATA

---

name: processor-dag
params:
  - INPUT_DATA
steps:
  - command: echo "Processing ${INPUT_DATA}"
    output: RESULT

  - command: |
      echo "Validated: ${RESULT}"
`)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 2)
		require.Equal(t, "generate-data", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)
		require.Equal(t, "process-data", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[1].Status)
	})

	t.Run("NonExistentReference", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: run-missing-dag
    call: non-existent-dag

---

name: some-other-dag
steps:
  - echo "test"
`)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.Error(t, err)
		require.Contains(t, err.Error(), "non-existent-dag")

		testDAG.AssertLatestStatus(t, core.Failed)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "run-missing-dag", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeFailed, dagRunStatus.Nodes[0].Status)
	})

	t.Run("ComplexDependencies", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
type: graph
steps:
  - name: setup
    command: echo "Setting up"
    output: SETUP_STATUS

  - name: task1
    call: task-dag
    params: "TASK_NAME=Task1 SETUP=${SETUP_STATUS}"
    output: TASK1_RESULT

  - name: task2
    call: task-dag
    params: "TASK_NAME=Task2 SETUP=${SETUP_STATUS}"
    output: TASK2_RESULT

  - name: combine
    command: |
      echo "Combining ${TASK1_RESULT.outputs.RESULT} and ${TASK2_RESULT.outputs.RESULT}"
    depends:
      - task1
      - task2

---

name: task-dag
params:
  - TASK_NAME
  - SETUP
steps:
  - command: echo "${TASK_NAME} processing with ${SETUP}"
    output: RESULT
`)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		require.Len(t, dagRunStatus.Nodes, 4)
		require.Equal(t, "setup", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)
		require.Equal(t, "task1", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[1].Status)
		require.Equal(t, "task2", dagRunStatus.Nodes[2].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[2].Status)
		require.Equal(t, "combine", dagRunStatus.Nodes[3].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[3].Status)

		logContent, err := os.ReadFile(dagRunStatus.Nodes[3].Stdout)
		require.NoError(t, err)
		require.Contains(t, string(logContent), "Combining")
		require.Contains(t, string(logContent), "Task1 processing with Setting up")
		require.Contains(t, string(logContent), "Task2 processing with Setting up")
	})

	t.Run("PartialSuccessParallel", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: parallel-tasks
    call: worker-dag
    parallel:
      items:
        - TASK_ID=1 TASK_NAME=alpha
---

name: worker-dag
params:
  - TASK_ID
  - TASK_NAME
steps:
  - command: exit 1
    continueOn:
      failure: true

  - exit 0
`)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.PartiallySucceeded)
	})

	t.Run("PartialSuccessSubDAG", func(t *testing.T) {
		th := test.Setup(t)

		testDAG := th.DAG(t, `
steps:
  - name: parallel-tasks
    call: worker-dag
---

name: worker-dag
params:
  - TASK_ID
  - TASK_NAME
steps:
  - command: exit 1
    continueOn:
      failure: true

  - exit 0
`)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.PartiallySucceeded)
	})
}

func TestExternalSubDAG(t *testing.T) {
	t.Run("BasicOutputCapture", func(t *testing.T) {
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
			ExpectedOut: []string{"DAG run finished"},
		})

		ctx := context.Background()
		ref := exec.NewDAGRunRef("parent_basic", dagRunID)
		parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		parentStatus, err := parentAttempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded.String(), parentStatus.Status.String())

		subNode := parentStatus.Nodes[0]
		require.Equal(t, core.NodeSucceeded.String(), subNode.Status.String())

		subAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, subNode.SubRuns[0].DAGRunID)
		require.NoError(t, err)

		subStatus, err := subAttempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded.String(), subStatus.Status.String())

		basicStep := subStatus.Nodes[0]
		require.Equal(t, core.NodeSucceeded.String(), basicStep.Status.String())

		require.NotNil(t, basicStep.OutputVariables, "OutputVariables should not be nil")
		variables := basicStep.OutputVariables.Variables()
		require.Contains(t, variables, "STEP_OUTPUT")
		require.Contains(t, variables["STEP_OUTPUT"], "hello_from_sub")
	})

	t.Run("RetrySubDAGRun", func(t *testing.T) {
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
			ExpectedOut: []string{"DAG run finished"},
		})

		// Update the sub_2 status to "failed" to simulate a retry
		ctx := context.Background()
		ref := exec.NewDAGRunRef("parent", dagRunID)
		parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		updateStatus := func(rec exec.DAGRunAttempt, dagRunStatus *exec.DAGRunStatus) {
			err = rec.Open(ctx)
			require.NoError(t, err)
			err = rec.Write(ctx, *dagRunStatus)
			require.NoError(t, err)
			err = rec.Close(ctx)
			require.NoError(t, err)
		}

		// Find and update sub_1 node status
		parentStatus, err := parentAttempt.ReadStatus(ctx)
		require.NoError(t, err)

		sub1Node := parentStatus.Nodes[0]
		sub1Node.Status = core.NodeFailed
		updateStatus(parentAttempt, parentStatus)

		// Find and update sub_1 dag-run status
		sub1Attempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, sub1Node.SubRuns[0].DAGRunID)
		require.NoError(t, err)

		sub1Status, err := sub1Attempt.ReadStatus(ctx)
		require.NoError(t, err)

		// Find and update sub_2 node status
		sub2Node := sub1Status.Nodes[0]
		sub2Node.Status = core.NodeFailed
		updateStatus(sub1Attempt, sub1Status)

		// Find and update sub_2 dag-run status
		sub2Attempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, sub2Node.SubRuns[0].DAGRunID)
		require.NoError(t, err)

		sub2Status, err := sub2Attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded.String(), sub2Status.Status.String())

		// Update the step in sub_2 to "failed"
		sub2Status.Nodes[0].Status = core.NodeFailed
		updateStatus(sub2Attempt, sub2Status)

		// Verify sub_2 is now "failed"
		sub2Status, err = sub2Attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeFailed.String(), sub2Status.Nodes[0].Status.String())

		// Retry the DAG
		args = []string{"retry", "--run-id", dagRunID, "parent"}
		th.RunCommand(t, cmd.Retry(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{"DAG run finished"},
		})

		// Check if the sub_2 status is now "success"
		sub2Attempt, err = th.DAGRunStore.FindSubAttempt(ctx, ref, sub2Node.SubRuns[0].DAGRunID)
		require.NoError(t, err)
		sub2Status, err = sub2Attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded.String(), sub2Status.Nodes[0].Status.String())
		require.Equal(t, "parent", sub2Status.Root.Name)
		require.Equal(t, dagRunID, sub2Status.Root.ID)
	})

	t.Run("RetryPolicyWithOutputCapture", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		counterFile := filepath.Join("/tmp", "retry_counter_"+dagRunID)
		defer func() { _ = os.Remove(counterFile) }()

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
			ExpectedOut: []string{"DAG run finished"},
		})

		ctx := context.Background()
		ref := exec.NewDAGRunRef("parent_retry", dagRunID)
		parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		parentStatus, err := parentAttempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded.String(), parentStatus.Status.String())

		subNode := parentStatus.Nodes[0]
		require.Equal(t, core.NodeSucceeded.String(), subNode.Status.String())

		subAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, subNode.SubRuns[0].DAGRunID)
		require.NoError(t, err)

		subStatus, err := subAttempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded.String(), subStatus.Status.String())

		retryStep := subStatus.Nodes[0]
		require.Equal(t, core.NodeSucceeded.String(), retryStep.Status.String())
		require.NotNil(t, retryStep.OutputVariables)

		variables := retryStep.OutputVariables.Variables()
		require.Contains(t, variables, "STEP_OUTPUT")
		require.Contains(t, variables["STEP_OUTPUT"], "output_attempt_2_success")
	})
}

func TestRetryPolicy(t *testing.T) {
	t.Run("BasicOutputCapture", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		counterFile := filepath.Join("/tmp", "retry_counter_basic_"+dagRunID)
		defer func() { _ = os.Remove(counterFile) }()

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
			ExpectedOut: []string{"DAG run finished"},
		})

		ctx := context.Background()
		ref := exec.NewDAGRunRef("basic_retry", dagRunID)
		attempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		dagRunStatus, err := attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded.String(), dagRunStatus.Status.String())

		retryStep := dagRunStatus.Nodes[0]
		require.Equal(t, core.NodeSucceeded.String(), retryStep.Status.String())
		require.NotNil(t, retryStep.OutputVariables)

		variables := retryStep.OutputVariables.Variables()
		require.Contains(t, variables, "STEP_OUTPUT")
		require.Contains(t, variables["STEP_OUTPUT"], "output_attempt_2_success")
	})

	t.Run("NoRetryOutputCapture", func(t *testing.T) {
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
			ExpectedOut: []string{"DAG run finished"},
		})

		ctx := context.Background()
		ref := exec.NewDAGRunRef("no_retry", dagRunID)
		attempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		dagRunStatus, err := attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.NodeSucceeded.String(), dagRunStatus.Status.String())

		successStep := dagRunStatus.Nodes[0]
		require.Equal(t, core.NodeSucceeded.String(), successStep.Status.String())
		require.NotNil(t, successStep.OutputVariables)

		variables := successStep.OutputVariables.Variables()
		require.Contains(t, variables, "STEP_OUTPUT")
		require.Contains(t, variables["STEP_OUTPUT"], "output_first_attempt_success")
	})
}
