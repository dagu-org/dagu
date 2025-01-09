package agent_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/test"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/stretchr/testify/require"
)

func TestAgent_Run(t *testing.T) {
	t.Parallel()

	t.Run("RunDAG", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.LoadDAGFile(t, "run.yaml")
		dagAgent := dag.Agent()

		dag.AssertLatestStatus(t, scheduler.StatusNone)

		go func() {
			dagAgent.RunSuccess(t)
		}()

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
	})
	t.Run("DeleteOldHistory", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.LoadDAGFile(t, "delete_old_history.yaml")
		dagAgent := dag.Agent()

		// Create a history file by running a DAG
		dagAgent.RunSuccess(t)
		dag.AssertHistoryCount(t, 1)

		// Set the retention days to 0 (delete all history files except the latest one)
		dag.HistRetentionDays = 0

		// Run the DAG again
		dagAgent = dag.Agent()
		dagAgent.RunSuccess(t)

		// Check if only the latest history file exists
		dag.AssertHistoryCount(t, 1)
	})
	t.Run("AlreadyRunning", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.LoadDAGFile(t, "is_running.yaml")
		dagAgent := dag.Agent()

		go func() {
			// Run the DAG in the background so that it is running
			dagAgent.RunSuccess(t)
		}()

		dag.AssertCurrentStatus(t, scheduler.StatusRunning)

		// Try to run the DAG again while it is running
		dagAgent = dag.Agent()
		dagAgent.RunCheckErr(t, "is already running")
	})
	t.Run("PreConditionNotMet", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.LoadDAGFile(t, "multiple_steps.yaml")

		// Set a precondition that always fails
		dag.Preconditions = []digraph.Condition{
			{Condition: "`echo 1`", Expected: "0"},
		}

		dagAgent := dag.Agent()
		dagAgent.RunCheckErr(t, "condition was not met")

		// Check if all nodes are not executed
		status := dagAgent.Status()
		require.Equal(t, scheduler.StatusCancel.String(), status.Status.String())
		require.Equal(t, scheduler.NodeStatusNone.String(), status.Nodes[0].Status.String())
		require.Equal(t, scheduler.NodeStatusNone.String(), status.Nodes[1].Status.String())
	})
	t.Run("FinishWithError", func(t *testing.T) {
		th := test.Setup(t)
		errDAG := th.LoadDAGFile(t, "error.yaml")
		dagAgent := errDAG.Agent()
		dagAgent.RunError(t)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, dagAgent.Status().Status)
	})
	t.Run("FinishWithTimeout", func(t *testing.T) {
		th := test.Setup(t)
		timeoutDAG := th.LoadDAGFile(t, "timeout.yaml")
		dagAgent := timeoutDAG.Agent()
		dagAgent.RunError(t)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, dagAgent.Status().Status)
	})
	t.Run("ReceiveSignal", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.LoadDAGFile(t, "sleep.yaml")
		dagAgent := dag.Agent()

		go func() {
			dagAgent.RunCancel(t)
		}()

		// wait for the DAG to start
		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		// send a signal to cancel the DAG
		dagAgent.Abort()

		// wait for the DAG to be canceled
		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
	t.Run("ExitHandler", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.LoadDAGFile(t, "on_exit.yaml")
		dagAgent := dag.Agent()
		dagAgent.RunSuccess(t)

		// Check if the DAG is executed successfully
		status := dagAgent.Status()
		require.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
		for _, s := range status.Nodes {
			require.Equal(t, scheduler.NodeStatusSuccess.String(), s.Status.String())
		}

		// Check if the exit handler is executed
		require.Equal(t, scheduler.NodeStatusSuccess.String(), status.OnExit.Status.String())
	})
}

func TestAgent_DryRun(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		th := test.Setup(t)

		dag := th.LoadDAGFile(t, "dry.yaml")
		dagAgent := dag.Agent(test.WithAgentOptions(agent.Options{Dry: true}))

		dagAgent.RunSuccess(t)

		curStatus := dagAgent.Status()
		require.Equal(t, scheduler.StatusSuccess, curStatus.Status)

		// Check if the status is not saved
		dag.AssertHistoryCount(t, 0)
	})
}

func TestAgent_Retry(t *testing.T) {
	t.Parallel()

	t.Run("RetryDAG", func(t *testing.T) {
		th := test.Setup(t)
		// retry.yaml has a DAG that fails
		dag := th.LoadDAGFile(t, "retry.yaml")
		dagAgent := dag.Agent()

		dagAgent.RunError(t)

		// Modify the DAG to make it successful
		status := dagAgent.Status()
		for i := range status.Nodes {
			status.Nodes[i].Step.CmdArgsSys = "true"
		}

		// Retry the DAG and check if it is successful
		dagAgent = dag.Agent(test.WithAgentOptions(agent.Options{
			RetryTarget: &status,
		}))
		dagAgent.RunSuccess(t)

		for _, node := range dagAgent.Status().Nodes {
			if node.Status != scheduler.NodeStatusSuccess &&
				node.Status != scheduler.NodeStatusSkipped {
				t.Errorf("node %q is not successful: %s", node.Step.Name, node.Status)
			}
		}
	})
}

func TestAgent_HandleHTTP(t *testing.T) {
	t.Parallel()
	t.Run("HTTP_Valid", func(t *testing.T) {
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.LoadDAGFile(t, "handle_http_valid.yaml")
		dagAgent := dag.Agent()
		ctx := th.Context
		go func() {
			dagAgent.RunCancel(t)
		}()

		// Wait for the DAG to start
		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		// Get the status of the DAG
		var mockResponseWriter = mockResponseWriter{}
		dagAgent.HandleHTTP(ctx)(&mockResponseWriter, &http.Request{
			Method: "GET", URL: &url.URL{Path: "/status"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)

		// Check if the status is returned correctly
		status, err := model.StatusFromJSON(mockResponseWriter.body)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, status.Status)

		// Stop the DAG
		dagAgent.Abort()
		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
	t.Run("HTTP_InvalidRequest", func(t *testing.T) {
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.LoadDAGFile(t, "handle_http_invalid.yaml")
		dagAgent := dag.Agent()

		go func() {
			dagAgent.RunCancel(t)
		}()

		// Wait for the DAG to start
		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		var mockResponseWriter = mockResponseWriter{}

		// Request with an invalid path
		dagAgent.HandleHTTP(th.Context)(&mockResponseWriter, &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/invalid-path"},
		})
		require.Equal(t, http.StatusNotFound, mockResponseWriter.status)

		// Stop the DAG
		dagAgent.Abort()
		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
	t.Run("HTTP_HandleCancel", func(t *testing.T) {
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.LoadDAGFile(t, "handle_http_cancel.yaml")
		dagAgent := dag.Agent()

		done := make(chan struct{})
		go func() {
			dagAgent.RunCancel(t)
			close(done)
		}()

		// Wait for the DAG to start
		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		// Cancel the DAG
		var mockResponseWriter = mockResponseWriter{}
		dagAgent.HandleHTTP(th.Context)(&mockResponseWriter, &http.Request{
			Method: "POST",
			URL:    &url.URL{Path: "/stop"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)
		require.Equal(t, "OK", mockResponseWriter.body)

		// Wait for the DAG to stop
		<-done
		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
}

// Assert that mockResponseWriter implements http.ResponseWriter
var _ http.ResponseWriter = (*mockResponseWriter)(nil)

type mockResponseWriter struct {
	status int
	body   string
	header *http.Header
}

func (h *mockResponseWriter) Header() http.Header {
	if h.header == nil {
		h.header = &http.Header{}
	}
	return *h.header
}

func (h *mockResponseWriter) Write(body []byte) (int, error) {
	h.body = string(body)
	return len([]byte(h.body)), nil
}

func (h *mockResponseWriter) WriteHeader(statusCode int) {
	h.status = statusCode
}
