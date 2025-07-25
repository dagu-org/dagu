package integration_test

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestLocalDAGExecution(t *testing.T) {
	t.Run("SimpleLocalDAG", func(t *testing.T) {
		// Create a DAG with local child DAGs using separator
		yamlContent := `
steps:
  - name: run-local-child
    run: local-child
    params: "NAME=World"
    output: CHILD_RESULT

  - name: use-output
    command: echo "Child said ${CHILD_RESULT.outputs.GREETING}"
    depends: 
      - run-local-child

---

name: local-child
params:
  - NAME
steps:
  - name: greet
    command: echo "Hello, ${NAME}!"
    output: GREETING
  
  - name: confirm
    command: echo "Greeting was ${GREETING}"
    output: CONFIRMATION
    depends: 
      - greet
`
		// Setup test helper
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		// Run the DAG
		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Verify successful completion
		testDAG.AssertLatestStatus(t, status.Success)

		// Get the full run status
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Verify the first step (run-local-child) completed successfully
		// Note: The child DAG's output is not directly visible in the parent's stdout
		require.Len(t, dagRunStatus.Nodes, 2)
		require.Equal(t, "run-local-child", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)

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
    run: worker-dag
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
  - name: start
    command: echo "Starting task ${TASK_ID} - ${TASK_NAME}"
  
  - name: process
    command: echo "Processing ${TASK_NAME} with ID ${TASK_ID}"
    depends:
      - start
  
  - name: complete
    command: echo "Completed ${TASK_NAME}"
    depends:
      - process
`
		// Setup test helper
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		// Run the DAG
		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Verify successful completion
		testDAG.AssertLatestStatus(t, status.Success)

		// Get the full run status
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// For parallel execution, we should have one step that ran multiple instances
		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "parallel-tasks", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)
	})

	t.Run("NestedLocalDAGs", func(t *testing.T) {
		// Test that nested local DAGs beyond 1 level are not supported
		// This should fail because middle-dag tries to run leaf-dag, but leaf-dag
		// is not visible to middle-dag (only to root-dag)
		yamlContent := `
steps:
  - name: run-middle-dag
    run: middle-dag
    params: "ROOT_PARAM=FromRoot"

---

name: middle-dag
params:
  - ROOT_PARAM
steps:
  - name: process-root-param
    command: echo "Received ${ROOT_PARAM}"
    output: MIDDLE_OUTPUT
  
  - name: run-leaf-dag
    run: leaf-dag
    params: "MIDDLE_PARAM=${MIDDLE_OUTPUT} LEAF_PARAM=FromMiddle"
    depends:
      - process-root-param

---

name: leaf-dag
params:
  - MIDDLE_PARAM
  - LEAF_PARAM
steps:
  - name: final-task
    command: |
      echo "Middle: ${MIDDLE_PARAM}, Leaf: ${LEAF_PARAM}"
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		err := agent.Run(agent.Context)
		// The root DAG execution will fail because middle-dag fails
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed")

		// This should fail because middle-dag cannot see leaf-dag
		testDAG.AssertLatestStatus(t, status.Error)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Root DAG should have one step that tried to run middle-dag
		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "run-middle-dag", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, status.NodeError, dagRunStatus.Nodes[0].Status)
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
    run: production-dag
    depends:
      - check-env
    preconditions:
      - condition: "${ENV_TYPE}"
        expected: "production"

  - name: run-dev-dag
    run: development-dag
    depends:
      - check-env
    preconditions:
      - condition: "${ENV_TYPE}"
        expected: "development"

---

name: production-dag
steps:
  - name: prod-deploy
    command: echo "Deploying to production"
  
  - name: prod-verify
    command: echo "Verifying production deployment"
    depends:
      - prod-deploy

---

name: development-dag
steps:
  - name: dev-build
    command: echo "Building for development"
  
  - name: dev-test
    command: echo "Running development tests"
    depends:
      - dev-build
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, status.Success)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Should have 3 steps: check-env, run-prod-dag, run-dev-dag
		require.Len(t, dagRunStatus.Nodes, 3)

		// Check environment step
		require.Equal(t, "check-env", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)

		// Production DAG should run
		require.Equal(t, "run-prod-dag", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[1].Status)

		// Development DAG should be skipped
		require.Equal(t, "run-dev-dag", dagRunStatus.Nodes[2].Step.Name)
		require.Equal(t, status.NodeSkipped, dagRunStatus.Nodes[2].Status)
	})

	t.Run("LocalDAGWithOutputPassing", func(t *testing.T) {
		// Test passing outputs between local DAGs
		yamlContent := `
steps:
  - name: generate-data
    run: generator-dag
    output: GEN_OUTPUT

  - name: process-data
    run: processor-dag
    params: "INPUT_DATA=${GEN_OUTPUT.outputs.DATA}"
    depends:
      - generate-data

---

name: generator-dag
steps:
  - name: create-data
    command: echo "test-value-42"
    output: DATA

---

name: processor-dag
params:
  - INPUT_DATA
steps:
  - name: parse-data
    command: echo "Processing ${INPUT_DATA}"
    output: RESULT
  
  - name: validate-data
    command: |
      echo "Validated: ${RESULT}"
    depends:
      - parse-data
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, status.Success)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Should have 2 steps
		require.Len(t, dagRunStatus.Nodes, 2)

		// First step generates data
		require.Equal(t, "generate-data", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)

		// Second step processes data
		require.Equal(t, "process-data", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[1].Status)
	})

	t.Run("LocalDAGReferencesNonExistent", func(t *testing.T) {
		// Test error when referencing non-existent local DAG
		yamlContent := `
steps:
  - name: run-missing-dag
    run: non-existent-dag

---

name: existing-dag
steps:
  - name: task
    command: echo "test"
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
		testDAG.AssertLatestStatus(t, status.Error)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Should have one step that failed
		require.Len(t, dagRunStatus.Nodes, 1)
		require.Equal(t, "run-missing-dag", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, status.NodeError, dagRunStatus.Nodes[0].Status)
	})

	t.Run("LocalDAGWithComplexDependencies", func(t *testing.T) {
		// Test complex dependencies between local DAGs
		yamlContent := `
steps:
  - name: setup
    command: echo "Setting up"
    output: SETUP_STATUS

  - name: task1
    run: task-dag
    params: "TASK_NAME=Task1 SETUP=${SETUP_STATUS}"
    depends:
      - setup
    output: TASK1_RESULT

  - name: task2
    run: task-dag
    params: "TASK_NAME=Task2 SETUP=${SETUP_STATUS}"
    depends:
      - setup
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
  - name: process
    command: echo "${TASK_NAME} processing with ${SETUP}"
    output: RESULT
`
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		testDAG.AssertLatestStatus(t, status.Success)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, testDAG.DAG)
		require.NoError(t, err)

		// Should have 4 steps: setup, task1, task2, combine
		require.Len(t, dagRunStatus.Nodes, 4)

		// Verify each step
		require.Equal(t, "setup", dagRunStatus.Nodes[0].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[0].Status)

		require.Equal(t, "task1", dagRunStatus.Nodes[1].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[1].Status)

		require.Equal(t, "task2", dagRunStatus.Nodes[2].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[2].Status)

		require.Equal(t, "combine", dagRunStatus.Nodes[3].Step.Name)
		require.Equal(t, status.NodeSuccess, dagRunStatus.Nodes[3].Status)

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
    run: worker-dag
    parallel:
      items:
        - TASK_ID=1 TASK_NAME=alpha
---

name: worker-dag
params:
  - TASK_ID
  - TASK_NAME
steps:
  - name: s1
    command: exit 1
    continueOn:
      failure: true
  
  - name: s2
    command: exit 0
`
		// Setup test helper
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		// Run the DAG
		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Verify successful completion
		testDAG.AssertLatestStatus(t, status.PartialSuccess)
	})

	t.Run("PartialSuccessChildDAG", func(t *testing.T) {
		// Create a DAG with parallel execution of local DAGs
		yamlContent := `
steps:
  - name: parallel-tasks
    run: worker-dag
---

name: worker-dag
params:
  - TASK_ID
  - TASK_NAME
steps:
  - name: s1
    command: exit 1
    continueOn:
      failure: true
  
  - name: s2
    command: exit 0
`
		// Setup test helper
		th := test.Setup(t)

		// Load the DAG using helper
		testDAG := th.DAG(t, yamlContent)

		// Run the DAG
		agent := testDAG.Agent()
		require.NoError(t, agent.Run(agent.Context))

		// Verify successful completion
		testDAG.AssertLatestStatus(t, status.PartialSuccess)
	})
}
