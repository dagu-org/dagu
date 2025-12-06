package integration_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestHandlerOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dagYAML      string
		setupFunc    func(*testing.T, *core.DAG) // Optional: verify DAG parsing
		runFunc      func(*testing.T, context.Context, *test.Agent)
		validateFunc func(*testing.T, *execution.DAGRunStatus)
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
			validateFunc: func(t *testing.T, status *execution.DAGRunStatus) {
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
			validateFunc: func(t *testing.T, status *execution.DAGRunStatus) {
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
			validateFunc: func(t *testing.T, status *execution.DAGRunStatus) {
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
			validateFunc: func(t *testing.T, status *execution.DAGRunStatus) {
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
			validateFunc: func(t *testing.T, status *execution.DAGRunStatus) {
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
			validateFunc: func(t *testing.T, status *execution.DAGRunStatus) {
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
			validateFunc: func(t *testing.T, status *execution.DAGRunStatus) {
				require.Equal(t, core.Succeeded, status.Status)
				require.NotNil(t, status.OnExit, "exit handler should have been executed")
				require.Equal(t, core.NodeSucceeded, status.OnExit.Status)
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
