package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestHandlerOn(t *testing.T) {
	t.Parallel()

	t.Run("InitHandler_Success", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
handlerOn:
  init:
    command: "true"

steps:
  - name: step1
    command: "true"
`)
		// Verify parsing: init field is correctly set
		require.NotNil(t, dag.HandlerOn.Init)
		require.Equal(t, "onInit", dag.HandlerOn.Init.Name)

		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.Equal(t, core.Succeeded, status.Status)
		require.NotNil(t, status.OnInit, "init handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnInit.Status)
	})

	t.Run("InitHandler_Failure_StopsExecution", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
handlerOn:
  init:
    command: exit 1
  exit:
    command: "true"

steps:
  - name: step1
    command: "echo should-not-run"
`)
		agent := dag.Agent()
		_ = agent.Run(th.Context)

		status := agent.Status(th.Context)
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
	})

	t.Run("InitHandler_PreconditionSkip_StepsRun", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
handlerOn:
  init:
    command: "echo init-should-not-run"
    precondition: "false"

steps:
  - name: step1
    command: "true"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.Equal(t, core.Succeeded, status.Status)
		require.NotNil(t, status.OnInit, "init handler node should exist")
		require.Equal(t, core.NodeSkipped, status.OnInit.Status)

		// Steps should have run
		require.Equal(t, core.NodeSucceeded, status.Nodes[0].Status)
	})

	t.Run("InitHandler_DAGPreconditionFails_InitNotRun", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
precondition: "false"

handlerOn:
  init:
    command: "echo init-should-not-run"

steps:
  - name: step1
    command: "true"
`)
		agent := dag.Agent()
		_ = agent.Run(th.Context)

		status := agent.Status(th.Context)
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
	})

	t.Run("AbortHandler", func(t *testing.T) {
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
	})

	t.Run("FailureHandler", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
handlerOn:
  failure:
    command: "true"

steps:
  - name: failing-step
    command: exit 1
`)
		agent := dag.Agent()
		agent.RunError(t)

		status := agent.Status(th.Context)
		require.Equal(t, core.Failed, status.Status)
		require.NotNil(t, status.OnFailure, "failure handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnFailure.Status)
	})

	t.Run("SuccessHandler", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
handlerOn:
  success:
    command: "true"

steps:
  - name: passing-step
    command: "true"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.Equal(t, core.Succeeded, status.Status)
		require.NotNil(t, status.OnSuccess, "success handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnSuccess.Status)
	})

	t.Run("ExitHandler", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `
handlerOn:
  exit:
    command: "true"

steps:
  - name: passing-step
    command: "true"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		status := agent.Status(th.Context)
		require.Equal(t, core.Succeeded, status.Status)
		require.NotNil(t, status.OnExit, "exit handler should have been executed")
		require.Equal(t, core.NodeSucceeded, status.OnExit.Status)
	})
}
