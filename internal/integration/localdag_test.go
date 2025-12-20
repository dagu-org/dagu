package integration_test

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestLocalDAGExecution(t *testing.T) {
	t.Run("SimpleLocalDAG", func(t *testing.T) {
		// Create a DAG with local sub DAGs using separator
		yamlContent := `
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
`
		// Setup test helper
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		// Run the DAG
		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Verify successful completion
		testDAG.AssertLatestStatus(t, core.Succeeded)

		// Get the full run status
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Verify the first step (run-local-child) completed successfully
		// Note: The sub DAG's output is not directly visible in the parent's stdout
		require.Len(t, dagRunStatus.Nodes, 2)
		require.Equal(t, "run-local-child", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

		// Verify the second step output
		logContent, err := os.ReadFile(dagRunStatus.Nodes[1].Stdout)
		require.NoError(t, err)
		require.Contains(t, string(logContent), "Child said Hello, World!")
	})

	t.Run("ParallelLocalDAGExecution", func(t *testing.T) {
		// Create a DAG with parallel execution of local DAGs
		yamlContent := `
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
`
		// Setup test helper
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		// Run the DAG
		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Verify successful completion
		testDAG.AssertLatestStatus(t, core.Succeeded)

		// Get the full run status
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// For parallel execution, we should have one step that ran multiple instances
		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "parallel-tasks", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)
	})

	t.Run("NestedLocalDAGs", func(t *testing.T) {
		// Test that multi-level nested local DAGs are supported
		// middle-dag calls leaf-dag, which should work correctly
		yamlContent := `
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
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		require.NoError(t, err)

		// Multi-level nested local DAGs should succeed
		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Root DAG should have one step that ran middle-dag successfully
		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "run-middle-dag", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

		// Verify middle-dag has a sub-run
		require.Len(t, dagRunStatus.Nodes[0].SubRuns, 1, "middle-dag should have one sub-run")
	})

	t.Run("LocalDAGWithConditionalExecution", func(t *testing.T) {
		// Test conditional execution with local DAGs
		yamlContent := `
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
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Should have 3 steps: check-env, run-prod-dag, run-dev-dag
		require.Len(t, dagRunStatus.Nodes, 3)

		// Check environment step
		require.Equal(t, "check-env", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

		// Production DAG should run
		require.Equal(t, "run-prod-dag", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[1].Status)

		// Development DAG should be skipped
		require.Equal(t, "run-dev-dag", dagRunStatus.Nodes[2].Step.Name)
		require.Equal(t, core.NodeSkipped, dagRunStatus.Nodes[2].Status)
	})

	t.Run("LocalDAGWithOutputPassing", func(t *testing.T) {
		// Test passing outputs between local DAGs
		yamlContent := `
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
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Should have 2 steps
		require.Len(t, dagRunStatus.Nodes, 2)

		// First step generates data
		require.Equal(t, "generate-data", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

		// Second step processes data
		require.Equal(t, "process-data", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[1].Status)
	})

	t.Run("LocalDAGReferencesNonExistent", func(t *testing.T) {
		// Test error when referencing non-existent local DAG
		yamlContent := `
steps:
  - name: run-missing-dag
    call: non-existent-dag

---

name: some-other-dag
steps:
  - echo "test"
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		// The agent will return an error when a step fails
		require.Error(t, err)
		require.Contains(t, err.Error(), "non-existent-dag")

		// Check that the DAG failed
		testDAG.AssertLatestStatus(t, core.Failed)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Should have one step that failed
		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "run-missing-dag", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeFailed, dagRunStatus.Nodes[0].Status)
	})

	t.Run("LocalDAGWithComplexDependencies", func(t *testing.T) {
		// Test complex dependencies between local DAGs
		yamlContent := `
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
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, core.Succeeded)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Should have 4 steps: setup, task1, task2, combine
		require.Len(t, dagRunStatus.Nodes, 4)

		// Verify each step
		require.Equal(t, "setup", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[0].Status)

		require.Equal(t, "task1", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[1].Status)

		require.Equal(t, "task2", dagRunStatus.Nodes[2].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[2].Status)

		require.Equal(t, "combine", dagRunStatus.Nodes[3].Step.Name)
		require.Equal(t, core.NodeSucceeded, dagRunStatus.Nodes[3].Status)

		// Verify the combine step output
		logContent, err := os.ReadFile(dagRunStatus.Nodes[3].Stdout)
		require.NoError(t, err)
		require.Contains(t, string(logContent), "Combining")
		require.Contains(t, string(logContent), "Task1 processing with Setting up")
		require.Contains(t, string(logContent), "Task2 processing with Setting up")
	})
	t.Run("PartialSuccessParallel", func(t *testing.T) {
		// Create a DAG with parallel execution of local DAGs
		yamlContent := `
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
`
		// Setup test helper
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		// Run the DAG
		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Verify successful completion
		testDAG.AssertLatestStatus(t, core.PartiallySucceeded)
	})

	t.Run("PartialSuccessSubDAG", func(t *testing.T) {
		// Create a DAG with parallel execution of local DAGs
		yamlContent := `
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
`
		// Setup test helper
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		// Run the DAG
		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Verify successful completion
		testDAG.AssertLatestStatus(t, core.PartiallySucceeded)
	})
}
