package agent_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

func TestAgent_Run(t *testing.T) {
	t.Parallel()

	t.Run("RunDAG", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, "agent/run.yaml")
		dagAgent := dag.Agent()

		dag.AssertLatestStatus(t, scheduler.StatusNone)

		go func() {
			dagAgent.RunSuccess(t)
		}()

		dag.AssertLatestStatus(t, scheduler.StatusSuccess)
	})
	t.Run("DeleteOldHistory", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, "agent/delete_old_history.yaml")
		dagAgent := dag.Agent()

		// Create a history file by running a DAG
		dagAgent.RunSuccess(t)
		dag.AssertDAGRunCount(t, 1)

		// Set the retention days to 0 (delete all history files except the latest one)
		dag.HistRetentionDays = 0

		// Run the DAG again
		dagAgent = dag.Agent()
		dagAgent.RunSuccess(t)

		// Check if only the latest history file exists
		dag.AssertDAGRunCount(t, 1)
	})
	t.Run("AlreadyRunning", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, "agent/is_running.yaml")
		dagAgent := dag.Agent(test.WithDAGRunID("test-dag-run"))
		done := make(chan struct{})

		go func() {
			// Run the DAG in the background so that it is running
			dagAgent.RunSuccess(t)
			close(done)
		}()

		dag.AssertCurrentStatus(t, scheduler.StatusRunning)

		isRunning := th.DAGRunMgr.IsRunning(context.Background(), dag.DAG, "test-dag-run")
		require.True(t, isRunning, "DAG should be running")

		// Wait for the DAG to finish
		<-done
	})
	t.Run("PreConditionNotMet", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, "agent/multiple_steps.yaml")

		// Set a precondition that always fails
		dag.Preconditions = []*digraph.Condition{
			{Condition: "`echo 1`", Expected: "0"},
		}

		dagAgent := dag.Agent()
		dagAgent.RunCancel(t)

		// Check if all nodes are not executed
		status := dagAgent.Status(th.Context)
		require.Equal(t, scheduler.StatusCancel.String(), status.Status.String())
		require.Equal(t, scheduler.NodeStatusNone.String(), status.Nodes[0].Status.String())
		require.Equal(t, scheduler.NodeStatusNone.String(), status.Nodes[1].Status.String())
	})
	t.Run("FinishWithError", func(t *testing.T) {
		th := test.Setup(t)
		errDAG := th.DAG(t, "agent/error.yaml")
		dagAgent := errDAG.Agent()
		dagAgent.RunError(t)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, dagAgent.Status(th.Context).Status)
	})
	t.Run("FinishWithTimeout", func(t *testing.T) {
		th := test.Setup(t)
		timeoutDAG := th.DAG(t, "agent/timeout.yaml")
		dagAgent := timeoutDAG.Agent()
		dagAgent.RunError(t)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, dagAgent.Status(th.Context).Status)
	})
	t.Run("ReceiveSignal", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, "agent/sleep.yaml")
		dagAgent := dag.Agent()
		done := make(chan struct{})

		go func() {
			dagAgent.RunCancel(t)
			close(done)
		}()

		// wait for the DAG to start
		dag.AssertLatestStatus(t, scheduler.StatusRunning)

		// send a signal to cancel the DAG
		dagAgent.Abort()

		<-done

		// wait for the DAG to be canceled
		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
	t.Run("ExitHandler", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, "agent/on_exit.yaml")
		dagAgent := dag.Agent()
		dagAgent.RunSuccess(t)

		// Check if the DAG is executed successfully
		status := dagAgent.Status(th.Context)
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

		dag := th.DAG(t, "agent/dry.yaml")
		dagAgent := dag.Agent(test.WithAgentOptions(agent.Options{Dry: true}))

		dagAgent.RunSuccess(t)

		curStatus := dagAgent.Status(th.Context)
		require.Equal(t, scheduler.StatusSuccess, curStatus.Status)

		// Check if the status is not saved
		dag.AssertDAGRunCount(t, 0)
	})
}

func TestAgent_Retry(t *testing.T) {
	t.Parallel()

	t.Run("RetryDAG", func(t *testing.T) {
		th := test.Setup(t)
		// retry.yaml has a DAG that fails
		dag := th.DAG(t, "agent/retry.yaml")
		dagAgent := dag.Agent()

		dagAgent.RunError(t)

		// Modify the DAG to make it successful
		status := dagAgent.Status(th.Context)
		for i := range dag.Steps {
			dag.Steps[i].CmdWithArgs = "true"
		}

		// Retry the DAG and check if it is successful
		dagAgent = dag.Agent(test.WithAgentOptions(agent.Options{
			RetryTarget: &status,
		}))
		dagAgent.RunSuccess(t)

		for _, node := range dagAgent.Status(th.Context).Nodes {
			if node.Status != scheduler.NodeStatusSuccess &&
				node.Status != scheduler.NodeStatusSkipped {
				t.Errorf("node %q is not successful: %s", node.Step.Name, node.Status)
			}
		}
	})

	t.Run("StepRetry", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, "agent/retry.yaml")
		dagAgent := dag.Agent()

		// Run the DAG to get a failed status
		dagAgent.RunError(t)
		status := dagAgent.Status(th.Context)

		// Save FinishedAt for all nodes before retry
		prevFinishedAt := map[string]string{}
		for _, node := range status.Nodes {
			prevFinishedAt[node.Step.Name] = node.FinishedAt
		}

		// Modify the DAG to make all steps successful
		for i := range dag.Steps {
			dag.Steps[i].Command = "true"
			dag.Steps[i].CmdWithArgs = "true"
		}

		// Sleep to ensure timestamps will be different
		time.Sleep(1 * time.Second)

		// Retry from step '5' using StepRetry
		dagAgent = dag.Agent(test.WithAgentOptions(agent.Options{
			RetryTarget: &status,
			StepRetry:   "5",
		}))
		err := dagAgent.Run(context.Background())
		require.NoError(t, err)

		// Only node 5 is retried, downstream steps remain untouched
		retried := map[string]struct{}{"5": {}}
		// Node 2 is a false command and should remain failed
		// Downstream nodes (6, 7, 8, 9) should remain in their previous state
		falseSteps := map[string]struct{}{"2": {}}
		// Check that only step '5' is rerun, all other steps remain unchanged
		st := dagAgent.Status(th.Context)

		for _, node := range st.Nodes {
			name := node.Step.Name
			prev := prevFinishedAt[name]
			now := node.FinishedAt

			if _, isRetried := retried[name]; isRetried {
				// Only step '5' should be retried and successful
				if node.Status != scheduler.NodeStatusSuccess && node.Status != scheduler.NodeStatusSkipped {
					t.Errorf("step %q is not successful or skipped after step retry: %s", name, node.Status)
				}
				// FinishedAt should be fresher (more recent) than before, if it was set
				if prev != "" && now != "" && now <= prev {
					t.Errorf("retried step %q FinishedAt not updated: was %v, now %v", name, prev, now)
				}
			} else {
				// Assert that steps with "false" commands are still failed
				if _, isFalseStep := falseSteps[name]; isFalseStep {
					if node.Status != scheduler.NodeStatusError {
						t.Errorf("non-retried step %q (false command) should remain failed after step retry, got: %s", name, node.Status)
					}
				}
				// FinishedAt should be unchanged for all non-retried steps
				if prev != now {
					t.Errorf("non-retried step %q FinishedAt changed after step retry: was %v, now %v", name, prev, now)
				}
			}
		}
	})
}

func TestAgent_HandleHTTP(t *testing.T) {
	t.Parallel()
	t.Run("HTTP_Valid", func(t *testing.T) {
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.DAG(t, "agent/handle_http_valid.yaml")
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
		status, err := models.StatusFromJSON(mockResponseWriter.body)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, status.Status)

		// Stop the DAG
		dagAgent.Abort()
		dag.AssertLatestStatus(t, scheduler.StatusCancel)
	})
	t.Run("HTTP_InvalidRequest", func(t *testing.T) {
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.DAG(t, "agent/handle_http_invalid.yaml")
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
		dag := th.DAG(t, "agent/handle_http_cancel.yaml")
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
