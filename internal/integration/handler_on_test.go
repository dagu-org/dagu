package integration_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestHandlerOn(t *testing.T) {
	t.Parallel()

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
