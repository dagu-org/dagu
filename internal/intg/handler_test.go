package intg_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dagYAML      string
		setupFunc    func(*testing.T, *core.DAG) // Optional: verify DAG parsing
		runFunc      func(*testing.T, context.Context, *test.Agent)
		validateFunc func(*testing.T, *exec.DAGRunStatus)
	}{
		{
			name: "InitHandler_Success",
			dagYAML: `
handlerOn:
  init:
    command: "true"

steps:
  - name: step1
    command: "true"
`,
			setupFunc: func(t *testing.T, dag *core.DAG) {
				require.NotNil(t, dag.HandlerOn.Init)
				require.Equal(t, "onInit", dag.HandlerOn.Init.Name)
			},
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
				require.NotNil(t, status.OnInit, "init handler should have been executed")
				require.Equal(t, core.NodeSucceeded, status.OnInit.Status)
			},
		},
		{
			name: "InitHandler_Failure_StopsExecution",
			dagYAML: `
handlerOn:
  init:
    command: exit 1
  exit:
    command: "true"

steps:
  - name: step1
    command: "echo should-not-run"
`,
			runFunc: func(_ *testing.T, ctx context.Context, agent *test.Agent) {
				_ = agent.Run(ctx)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				// Init failure causes DAG to be aborted (canceled internally)
				require.Equal(t, core.Aborted, status.Status)
				require.NotNil(t, status.OnInit, "init handler should have been executed")
				require.Equal(t, core.NodeFailed, status.OnInit.Status)

				// Exit handler should have run
				require.NotNil(t, status.OnExit, "exit handler should have been executed")
				require.Equal(t, core.NodeSucceeded, status.OnExit.Status)

				// Steps should not have run
				require.Len(t, status.Nodes, 1)
				require.Equal(t, core.NodeNotStarted, status.Nodes[0].Status)
			},
		},
		{
			name: "InitHandler_PreconditionSkip_StepsRun",
			dagYAML: `
handlerOn:
  init:
    command: "echo init-should-not-run"
    precondition: "false"

steps:
  - name: step1
    command: "true"
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
				require.NotNil(t, status.OnInit, "init handler node should exist")
				require.Equal(t, core.NodeSkipped, status.OnInit.Status)

				// Steps should have run
				require.Equal(t, core.NodeSucceeded, status.Nodes[0].Status)
			},
		},
		{
			name: "InitHandler_DAGPreconditionFails_InitNotRun",
			dagYAML: `
precondition: "false"

handlerOn:
  init:
    command: "echo init-should-not-run"

steps:
  - name: step1
    command: "true"
`,
			runFunc: func(_ *testing.T, ctx context.Context, agent *test.Agent) {
				_ = agent.Run(ctx)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				require.Equal(t, core.Aborted, status.Status)

				// Init handler should not have run (DAG precondition failed first)
				// OnInit may be nil or NotStarted when the runner doesn't execute it
				if status.OnInit != nil {
					require.Equal(t, core.NodeNotStarted, status.OnInit.Status)
				}

				// Steps should not have run - they remain in NotStarted state when preconditions fail
				// and the runner immediately exits
				require.NotEmpty(t, status.Nodes)
				// The node could be NotStarted or Aborted depending on timing
				nodeStatus := status.Nodes[0].Status
				require.True(t, nodeStatus == core.NodeNotStarted || nodeStatus == core.NodeAborted,
					"expected NotStarted or Aborted, got %v", nodeStatus)
			},
		},
		{
			name: "FailureHandler",
			dagYAML: `
handlerOn:
  failure:
    command: "true"

steps:
  - name: failing-step
    command: exit 1
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunError(t)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				require.Equal(t, core.Failed, status.Status)
				require.NotNil(t, status.OnFailure, "failure handler should have been executed")
				require.Equal(t, core.NodeSucceeded, status.OnFailure.Status)
			},
		},
		{
			name: "SuccessHandler",
			dagYAML: `
handlerOn:
  success:
    command: "true"

steps:
  - name: passing-step
    command: "true"
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
				require.NotNil(t, status.OnSuccess, "success handler should have been executed")
				require.Equal(t, core.NodeSucceeded, status.OnSuccess.Status)
			},
		},
		{
			name: "ExitHandler",
			dagYAML: `
handlerOn:
  exit:
    command: "true"

steps:
  - name: passing-step
    command: "true"
`,
			runFunc: func(t *testing.T, _ context.Context, agent *test.Agent) {
				agent.RunSuccess(t)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
				require.NotNil(t, status.OnExit, "exit handler should have been executed")
				require.Equal(t, core.NodeSucceeded, status.OnExit.Status)
			},
		},
		{
			name: "WaitHandler_ExecutedOnWaitStatus",
			dagYAML: `
handlerOn:
  wait:
    command: "true"

steps:
  - name: wait-step
    type: hitl
`,
			setupFunc: func(t *testing.T, dag *core.DAG) {
				require.NotNil(t, dag.HandlerOn.Wait)
				require.Equal(t, "onWait", dag.HandlerOn.Wait.Name)
			},
			runFunc: func(_ *testing.T, ctx context.Context, agent *test.Agent) {
				_ = agent.Run(ctx)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				require.Equal(t, core.Waiting, status.Status)
				require.NotNil(t, status.OnWait, "wait handler should have been executed")
				require.Equal(t, core.NodeSucceeded, status.OnWait.Status)
			},
		},
		{
			name: "WaitHandler_FailureDoesNotBlockWaitStatus",
			dagYAML: `
handlerOn:
  wait:
    command: exit 1

steps:
  - name: wait-step
    type: hitl
`,
			runFunc: func(_ *testing.T, ctx context.Context, agent *test.Agent) {
				_ = agent.Run(ctx)
			},
			validateFunc: func(t *testing.T, status *exec.DAGRunStatus) {
				// DAG should still be in Wait status even if handler failed
				require.Equal(t, core.Waiting, status.Status)
				require.NotNil(t, status.OnWait, "wait handler should have been executed")
				require.Equal(t, core.NodeFailed, status.OnWait.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := test.Setup(t)
			dag := th.DAG(t, tt.dagYAML)

			if tt.setupFunc != nil {
				tt.setupFunc(t, dag.DAG)
			}

			agent := dag.Agent()
			tt.runFunc(t, th.Context, agent)

			status := agent.Status(th.Context)
			tt.validateFunc(t, &status)
		})
	}
}

// TestHandlerOn_Abort tests the abort handler which requires special async handling
func TestHandlerOn_Abort(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	dag := th.DAG(t, `
handlerOn:
  abort:
    command: "true"

steps:
  - name: long-running
    command: sleep 10
`)

	// Verify parsing: abort field maps to internal Cancel
	require.NotNil(t, dag.HandlerOn.Cancel)
	require.Equal(t, "onCancel", dag.HandlerOn.Cancel.Name)

	dagAgent := dag.Agent()

	done := make(chan struct{})
	go func() {
		_ = dagAgent.Run(th.Context)
		close(done)
	}()

	// Wait for the DAG to start running
	dag.AssertLatestStatus(t, core.Running)

	// Abort the DAG
	dagAgent.Abort()

	// Wait for completion
	<-done

	// Verify the abort handler was executed
	status := dagAgent.Status(th.Context)
	require.Equal(t, core.Aborted, status.Status)
	require.NotNil(t, status.OnCancel, "abort handler should have been executed")
	require.Equal(t, core.NodeSucceeded, status.OnCancel.Status)
}

// TestHandlerOn_EnvironmentVariables tests that special environment variables
// are accessible from each handler type.
//
// Environment variables availability by handler:
//
// | Variable                   | Init   | Success   | Failure    | Cancel  | Exit      |
// |----------------------------|--------|-----------|------------|---------|-----------|
// | DAG_NAME                   | ✓      | ✓         | ✓          | ✓       | ✓         |
// | DAG_RUN_ID                 | ✓      | ✓         | ✓          | ✓       | ✓         |
// | DAG_RUN_LOG_FILE           | ✓      | ✓         | ✓          | ✓       | ✓         |
// | DAG_RUN_STEP_NAME          | onInit | onSuccess | onFailure  | onCancel| onExit    |
// | DAG_RUN_STATUS             | running| succeeded | failed     | aborted | succeeded/failed |
// | DAG_RUN_STEP_STDOUT_FILE   | ✗      | ✗         | ✗          | ✗       | ✗         |
// | DAG_RUN_STEP_STDERR_FILE   | ✗      | ✗         | ✗          | ✗       | ✗         |
//
// Note: DAG_RUN_STATUS in init handler is "running" because the DAG run has started
// but steps haven't executed yet. DAG_RUN_STEP_STDOUT_FILE and DAG_RUN_STEP_STDERR_FILE
// are not set for handlers because node.SetupEnv is not called during handler execution.
func TestHandlerOn_EnvironmentVariables(t *testing.T) {
	t.Parallel()

	// Helper to extract value from "KEY=value" format
	extractValue := func(output string) string {
		if idx := strings.Index(output, "="); idx != -1 {
			return output[idx+1:]
		}
		return output
	}

	t.Run("InitHandler_BaseEnvVars", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Test that basic env vars (DAG_NAME, DAG_RUN_ID, DAG_RUN_LOG_FILE, DAG_RUN_STEP_NAME)
		// are available in the init handler
		dag := th.DAG(t, `
handlerOn:
  init:
    command: |
      echo "name:${DAG_NAME}|runid:${DAG_RUN_ID}|logfile:${DAG_RUN_LOG_FILE}|stepname:${DAG_RUN_STEP_NAME}"
    output: INIT_ENV_OUTPUT

steps:
  - name: step1
    command: "true"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnInit, "init handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnInit.Status)
		require.NotNil(t, status.OnInit.OutputVariables, "init handler should have output variables")

		output, ok := status.OnInit.OutputVariables.Load("INIT_ENV_OUTPUT")
		require.True(t, ok, "INIT_ENV_OUTPUT should be set")

		outputStr := extractValue(output.(string))

		// Verify DAG_NAME is set and non-empty
		assert.Contains(t, outputStr, "name:", "output should contain name prefix")
		assert.NotContains(t, outputStr, "name:|", "DAG_NAME should not be empty")

		// Verify DAG_RUN_ID is set (UUID format)
		assert.Contains(t, outputStr, "runid:", "output should contain runid prefix")
		assert.NotContains(t, outputStr, "runid:|", "DAG_RUN_ID should not be empty")

		// Verify DAG_RUN_LOG_FILE is set and contains .log
		assert.Contains(t, outputStr, "logfile:", "output should contain logfile prefix")
		assert.Contains(t, outputStr, ".log", "DAG_RUN_LOG_FILE should contain .log")

		// Verify DAG_RUN_STEP_NAME is set to "onInit"
		assert.Contains(t, outputStr, "stepname:onInit", "DAG_RUN_STEP_NAME should be 'onInit'")
	})

	t.Run("InitHandler_DAGRunStatus_IsRunning", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// DAG_RUN_STATUS in init handler is "running" because the DAG run has started
		// but steps haven't completed yet. This value is technically correct but not
		// as useful as having the final status (which isn't known yet).
		dag := th.DAG(t, `
handlerOn:
  init:
    command: echo "${DAG_RUN_STATUS}"
    output: INIT_STATUS

steps:
  - name: step1
    command: "true"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnInit)
		require.NotNil(t, status.OnInit.OutputVariables)

		output, ok := status.OnInit.OutputVariables.Load("INIT_STATUS")
		require.True(t, ok)

		// DAG_RUN_STATUS is "running" for init handler because steps haven't completed
		outputStr := extractValue(output.(string))
		assert.Equal(t, "running", outputStr, "DAG_RUN_STATUS should be 'running' in init handler")
	})

	t.Run("SuccessHandler_AllEnvVars", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		dag := th.DAG(t, `
handlerOn:
  success:
    command: |
      echo "name:${DAG_NAME}|status:${DAG_RUN_STATUS}|stepname:${DAG_RUN_STEP_NAME}"
    output: SUCCESS_ENV_OUTPUT

steps:
  - name: step1
    command: "true"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnSuccess, "success handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnSuccess.Status)
		require.NotNil(t, status.OnSuccess.OutputVariables)

		output, ok := status.OnSuccess.OutputVariables.Load("SUCCESS_ENV_OUTPUT")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Verify DAG_NAME is set
		assert.NotContains(t, outputStr, "name:|", "DAG_NAME should not be empty")

		// Verify DAG_RUN_STATUS is "succeeded"
		assert.Contains(t, outputStr, "status:succeeded", "DAG_RUN_STATUS should be 'succeeded'")

		// Verify DAG_RUN_STEP_NAME is "onSuccess"
		assert.Contains(t, outputStr, "stepname:onSuccess", "DAG_RUN_STEP_NAME should be 'onSuccess'")
	})

	t.Run("FailureHandler_AllEnvVars", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		dag := th.DAG(t, `
handlerOn:
  failure:
    command: |
      echo "name:${DAG_NAME}|status:${DAG_RUN_STATUS}|stepname:${DAG_RUN_STEP_NAME}"
    output: FAILURE_ENV_OUTPUT

steps:
  - name: failing-step
    command: exit 1
`)
		agent := dag.Agent()
		agent.RunError(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnFailure, "failure handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnFailure.Status)
		require.NotNil(t, status.OnFailure.OutputVariables)

		output, ok := status.OnFailure.OutputVariables.Load("FAILURE_ENV_OUTPUT")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Verify DAG_NAME is set
		assert.NotContains(t, outputStr, "name:|", "DAG_NAME should not be empty")

		// Verify DAG_RUN_STATUS is "failed"
		assert.Contains(t, outputStr, "status:failed", "DAG_RUN_STATUS should be 'failed'")

		// Verify DAG_RUN_STEP_NAME is "onFailure"
		assert.Contains(t, outputStr, "stepname:onFailure", "DAG_RUN_STEP_NAME should be 'onFailure'")
	})

	t.Run("ExitHandler_AllEnvVars_OnSuccess", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		dag := th.DAG(t, `
handlerOn:
  exit:
    command: |
      echo "name:${DAG_NAME}|status:${DAG_RUN_STATUS}|stepname:${DAG_RUN_STEP_NAME}"
    output: EXIT_ENV_OUTPUT

steps:
  - name: step1
    command: "true"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnExit, "exit handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnExit.Status)
		require.NotNil(t, status.OnExit.OutputVariables)

		output, ok := status.OnExit.OutputVariables.Load("EXIT_ENV_OUTPUT")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Verify DAG_NAME is set
		assert.NotContains(t, outputStr, "name:|", "DAG_NAME should not be empty")

		// Verify DAG_RUN_STATUS is "succeeded" (exit runs after success)
		assert.Contains(t, outputStr, "status:succeeded", "DAG_RUN_STATUS should be 'succeeded'")

		// Verify DAG_RUN_STEP_NAME is "onExit"
		assert.Contains(t, outputStr, "stepname:onExit", "DAG_RUN_STEP_NAME should be 'onExit'")
	})

	t.Run("ExitHandler_AllEnvVars_OnFailure", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		dag := th.DAG(t, `
handlerOn:
  exit:
    command: |
      echo "status:${DAG_RUN_STATUS}"
    output: EXIT_ENV_OUTPUT

steps:
  - name: failing-step
    command: exit 1
`)
		agent := dag.Agent()
		agent.RunError(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnExit, "exit handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnExit.Status)
		require.NotNil(t, status.OnExit.OutputVariables)

		output, ok := status.OnExit.OutputVariables.Load("EXIT_ENV_OUTPUT")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Verify DAG_RUN_STATUS is "failed" (exit runs after failure)
		assert.Contains(t, outputStr, "status:failed", "DAG_RUN_STATUS should be 'failed'")
	})

	t.Run("CancelHandler_AllEnvVars", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		dag := th.DAG(t, `
handlerOn:
  abort:
    command: |
      echo "name:${DAG_NAME}|status:${DAG_RUN_STATUS}|stepname:${DAG_RUN_STEP_NAME}"
    output: CANCEL_ENV_OUTPUT

steps:
  - name: long-running
    command: sleep 10
`)
		dagAgent := dag.Agent()

		done := make(chan struct{})
		go func() {
			_ = dagAgent.Run(th.Context)
			close(done)
		}()

		dag.AssertLatestStatus(t, core.Running)
		dagAgent.Abort()
		<-done

		status := dagAgent.Status(th.Context)
		require.NotNil(t, status.OnCancel, "cancel handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnCancel.Status)
		require.NotNil(t, status.OnCancel.OutputVariables)

		output, ok := status.OnCancel.OutputVariables.Load("CANCEL_ENV_OUTPUT")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Verify DAG_NAME is set
		assert.NotContains(t, outputStr, "name:|", "DAG_NAME should not be empty")

		// Verify DAG_RUN_STATUS is "aborted"
		assert.Contains(t, outputStr, "status:aborted", "DAG_RUN_STATUS should be 'aborted'")

		// Verify DAG_RUN_STEP_NAME is "onCancel"
		assert.Contains(t, outputStr, "stepname:onCancel", "DAG_RUN_STEP_NAME should be 'onCancel'")
	})

	t.Run("StepOutputVars_NotAvailableInHandlers", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// DAG_RUN_STEP_STDOUT_FILE and DAG_RUN_STEP_STDERR_FILE are NOT set
		// for handlers because node.SetupEnv is not called for handler execution
		dag := th.DAG(t, `
handlerOn:
  success:
    command: |
      echo "stdout:${DAG_RUN_STEP_STDOUT_FILE:-UNSET}|stderr:${DAG_RUN_STEP_STDERR_FILE:-UNSET}"
    output: HANDLER_STEP_FILES

steps:
  - name: step1
    command: "true"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnSuccess)
		require.NotNil(t, status.OnSuccess.OutputVariables)

		output, ok := status.OnSuccess.OutputVariables.Load("HANDLER_STEP_FILES")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// These should be unset/empty in handlers
		assert.Contains(t, outputStr, "stdout:UNSET", "DAG_RUN_STEP_STDOUT_FILE should not be set in handler")
		assert.Contains(t, outputStr, "stderr:UNSET", "DAG_RUN_STEP_STDERR_FILE should not be set in handler")
	})

	t.Run("Handlers_CanAccessStepOutputVariables", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Handlers can access output variables from steps that have completed
		dag := th.DAG(t, `
handlerOn:
  success:
    command: |
      echo "step_output:${STEP_OUTPUT}"
    output: SUCCESS_WITH_STEP_OUTPUT

steps:
  - name: producer
    command: echo "produced_value"
    output: STEP_OUTPUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnSuccess)
		require.NotNil(t, status.OnSuccess.OutputVariables)

		output, ok := status.OnSuccess.OutputVariables.Load("SUCCESS_WITH_STEP_OUTPUT")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Success handler should be able to access step output
		assert.Contains(t, outputStr, "step_output:produced_value", "handler should access step output")
	})

	t.Run("InitHandler_CannotAccessStepOutputVariables", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Init handler runs BEFORE steps, so it cannot access step outputs
		dag := th.DAG(t, `
handlerOn:
  init:
    command: |
      echo "step_output:${STEP_OUTPUT:-NOT_YET_AVAILABLE}"
    output: INIT_STEP_ACCESS

steps:
  - name: producer
    command: echo "produced_value"
    output: STEP_OUTPUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.NotNil(t, status.OnInit)
		require.NotNil(t, status.OnInit.OutputVariables)

		output, ok := status.OnInit.OutputVariables.Load("INIT_STEP_ACCESS")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Init handler cannot access step output (steps haven't run yet)
		assert.Contains(t, outputStr, "step_output:NOT_YET_AVAILABLE",
			"init handler should not access step outputs")
	})

	t.Run("WaitHandler_DAG_WAITING_STEPS_EnvVar", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// DAG_WAITING_STEPS should contain comma-separated list of waiting step names
		dag := th.DAG(t, `
handlerOn:
  wait:
    command: |
      echo "waiting_steps:${DAG_WAITING_STEPS}"
    output: WAIT_HANDLER_OUTPUT

steps:
  - name: first-step
    command: "true"

  - name: wait-step
    type: hitl
    depends:
      - first-step
`)
		agent := dag.Agent()
		_ = agent.Run(th.Context)

		status := agent.Status(th.Context)
		require.Equal(t, core.Waiting, status.Status)
		require.NotNil(t, status.OnWait, "wait handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnWait.Status)
		require.NotNil(t, status.OnWait.OutputVariables)

		output, ok := status.OnWait.OutputVariables.Load("WAIT_HANDLER_OUTPUT")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Verify DAG_WAITING_STEPS contains the wait step name
		assert.Contains(t, outputStr, "waiting_steps:wait-step",
			"DAG_WAITING_STEPS should contain 'wait-step'")
	})

	t.Run("WaitHandler_EnvVarFormat", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Verify DAG_WAITING_STEPS format is correct (comma-separated list)
		// This test has a step with a hyphenated name to verify proper formatting
		dag := th.DAG(t, `
handlerOn:
  wait:
    command: |
      echo "waiting_steps:${DAG_WAITING_STEPS}"
    output: WAIT_HANDLER_OUTPUT

steps:
  - name: setup-step
    command: "true"

  - name: approval-gate
    type: hitl
    depends:
      - setup-step
`)
		agent := dag.Agent()
		_ = agent.Run(th.Context)

		status := agent.Status(th.Context)
		require.Equal(t, core.Waiting, status.Status)
		require.NotNil(t, status.OnWait)
		require.NotNil(t, status.OnWait.OutputVariables)

		output, ok := status.OnWait.OutputVariables.Load("WAIT_HANDLER_OUTPUT")
		require.True(t, ok)

		outputStr := extractValue(output.(string))

		// Verify DAG_WAITING_STEPS contains the wait step name with proper formatting
		assert.Contains(t, outputStr, "waiting_steps:approval-gate",
			"DAG_WAITING_STEPS should contain 'approval-gate' with correct format")
	})
}
