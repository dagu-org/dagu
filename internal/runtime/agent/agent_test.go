package agent_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/dagu-org/dagu/internal/test"

	"github.com/stretchr/testify/require"
)

func TestAgent_Run(t *testing.T) {
	t.Parallel()

	t.Run("RunDAG", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - "sleep 1"
`)
		dagAgent := dag.Agent()

		dag.AssertLatestStatus(t, core.NotStarted)

		go func() {
			dagAgent.RunSuccess(t)
		}()

		dag.AssertLatestStatus(t, core.Succeeded)
	})
	t.Run("DeleteOldHistory", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - "sleep 1"
`)
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
		dag := th.DAG(t, `steps:
  - "sleep 1"
`)
		dagAgent := dag.Agent(test.WithDAGRunID("test-dag-run"))
		done := make(chan struct{})

		go func() {
			// Run the DAG in the background so that it is running
			dagAgent.RunSuccess(t)
			close(done)
		}()

		dag.AssertCurrentStatus(t, core.Running)

		isRunning := th.DAGRunMgr.IsRunning(context.Background(), dag.DAG, "test-dag-run")
		require.True(t, isRunning, "DAG should be running")

		// Wait for the DAG to finish
		<-done
	})
	t.Run("PreConditionNotMet", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - "true"
  - "true"
`)

		// Set a precondition that always fails
		dag.Preconditions = []*core.Condition{
			{Condition: "`echo 1`", Expected: "0"},
		}

		dagAgent := dag.Agent()
		dagAgent.RunCancel(t)

		// Check if all nodes are not executed
		dagRunStatus := dagAgent.Status(th.Context)
		require.Equal(t, core.Aborted.String(), dagRunStatus.Status.String())
		require.Equal(t, core.NodeNotStarted.String(), dagRunStatus.Nodes[0].Status.String())
		require.Equal(t, core.NodeNotStarted.String(), dagRunStatus.Nodes[1].Status.String())
	})
	t.Run("FinishWithError", func(t *testing.T) {
		th := test.Setup(t)
		errDAG := th.DAG(t, `steps:
  - "false"
`)
		dagAgent := errDAG.Agent()
		dagAgent.RunError(t)

		// Check if the status is saved correctly
		require.Equal(t, core.Failed, dagAgent.Status(th.Context).Status)
	})
	t.Run("FinishWithTimeout", func(t *testing.T) {
		th := test.Setup(t)
		timeoutDAG := th.DAG(t, `timeoutSec: 2
steps:
  - "sleep 1"
  - "sleep 2"
`)
		dagAgent := timeoutDAG.Agent()
		dagAgent.RunError(t)

		// Check if the status is saved correctly
		require.Equal(t, core.Failed, dagAgent.Status(th.Context).Status)
	})
	t.Run("ReceiveSignal", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - "sleep 3"
`)
		dagAgent := dag.Agent()
		done := make(chan struct{})

		go func() {
			dagAgent.RunCancel(t)
			close(done)
		}()

		// wait for the DAG to start
		dag.AssertLatestStatus(t, core.Running)

		// send a signal to cancel the DAG
		dagAgent.Abort()

		<-done

		// wait for the DAG to be canceled
		dag.AssertLatestStatus(t, core.Aborted)
	})
	t.Run("ExitHandler", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `handlerOn:
  Exit:
    command: "true"
steps:
  - "true"
  - "true"
`)
		dagAgent := dag.Agent()
		dagAgent.RunSuccess(t)

		// Check if the DAG is executed successfully
		dagRunStatus := dagAgent.Status(th.Context)
		require.Equal(t, core.Succeeded.String(), dagRunStatus.Status.String())
		for _, s := range dagRunStatus.Nodes {
			require.Equal(t, core.NodeSucceeded.String(), s.Status.String())
		}

		// Check if the exit handler is executed
		require.Equal(t, core.NodeSucceeded.String(), dagRunStatus.OnExit.Status.String())
	})
}

func TestAgent_DryRun(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		th := test.Setup(t)

		dag := th.DAG(t, `steps:
  - "true"
`)
		dagAgent := dag.Agent(test.WithAgentOptions(agent.Options{Dry: true}))

		dagAgent.RunSuccess(t)

		curStatus := dagAgent.Status(th.Context)
		require.Equal(t, core.Succeeded, curStatus.Status)

		// Check if the status is not saved
		dag.AssertDAGRunCount(t, 0)
	})
}

func TestAgent_Retry(t *testing.T) {
	t.Parallel()

	t.Run("RetryDAG", func(t *testing.T) {
		th := test.Setup(t)
		// retry DAG that fails
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
  - name: "2"
    command: "false"
    continueOn:
      failure: true
    depends: ["1"]
  - name: "3"
    command: "true"
    depends: ["2"]
  - name: "4"
    command: "true"
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
    continueOn:
      skipped: true
  - name: "5"
    command: "false"
    depends: ["4"]
  - name: "6"
    command: "true"
    depends: ["5"]
  - name: "7"
    command: "true"
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
    depends: ["6"]
    continueOn:
      skipped: true
  - name: "8"
    command: "true"
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
  - name: "9"
    command: "false"
`)
		dagAgent := dag.Agent()

		dagAgent.RunError(t)

		// Modify the DAG to make it successful
		dagRunStatus := dagAgent.Status(th.Context)
		for i := range dag.Steps {
			dag.Steps[i].Commands = []core.CommandEntry{{Command: "true", CmdWithArgs: "true"}}
		}

		// Retry the DAG and check if it is successful
		dagAgent = dag.Agent(test.WithAgentOptions(agent.Options{
			RetryTarget: &dagRunStatus,
		}))
		dagAgent.RunSuccess(t)

		for _, node := range dagAgent.Status(th.Context).Nodes {
			if node.Status != core.NodeSucceeded &&
				node.Status != core.NodeSkipped {
				t.Errorf("node %q is not successful: %s", node.Step.Name, node.Status)
			}
		}
	})

	t.Run("StepRetry", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, `steps:
  - name: "1"
    command: "true"
  - name: "2"
    command: "false"
    continueOn:
      failure: true
    depends: ["1"]
  - name: "3"
    command: "true"
    depends: ["2"]
  - name: "4"
    command: "true"
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
    continueOn:
      skipped: true
  - name: "5"
    command: "false"
    depends: ["4"]
  - name: "6"
    command: "true"
    depends: ["5"]
  - name: "7"
    command: "true"
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
    depends: ["6"]
    continueOn:
      skipped: true
  - name: "8"
    command: "true"
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
  - name: "9"
    command: "false"
`)
		dagAgent := dag.Agent()

		// Run the DAG to get a failed status
		dagAgent.RunError(t)
		dagRunStatus := dagAgent.Status(th.Context)

		// Save FinishedAt for all nodes before retry
		prevFinishedAt := map[string]string{}
		for _, node := range dagRunStatus.Nodes {
			prevFinishedAt[node.Step.Name] = node.FinishedAt
		}

		// Modify the DAG to make all steps successful
		for i := range dag.Steps {
			dag.Steps[i].Commands = []core.CommandEntry{{Command: "true", CmdWithArgs: "true"}}
		}

		// Sleep to ensure timestamps will be different
		time.Sleep(1 * time.Second)

		// Retry from step '5' using StepRetry
		dagAgent = dag.Agent(test.WithAgentOptions(agent.Options{
			RetryTarget: &dagRunStatus,
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
				if node.Status != core.NodeSucceeded && node.Status != core.NodeSkipped {
					t.Errorf("step %q is not successful or skipped after step retry: %s", name, node.Status)
				}
				// FinishedAt should be fresher (more recent) than before, if it was set
				if prev != "" && now != "" && now <= prev {
					t.Errorf("retried step %q FinishedAt not updated: was %v, now %v", name, prev, now)
				}
			} else {
				// Assert that steps with "false" commands are still failed
				if _, isFalseStep := falseSteps[name]; isFalseStep {
					if node.Status != core.NodeFailed {
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

	t.Run("HTTPValid", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.DAG(t, `steps:
  - "sleep 10"
`)
		dagAgent := dag.Agent()
		ctx := th.Context
		go func() {
			dagAgent.RunCancel(t)
		}()

		// Wait for the DAG to start
		dag.AssertLatestStatus(t, core.Running)

		// Get the status of the DAG
		var mockResponseWriter = mockResponseWriter{}
		dagAgent.HandleHTTP(ctx)(&mockResponseWriter, &http.Request{
			Method: "GET", URL: &url.URL{Path: "/status"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)

		// Check if the status is returned correctly
		dagRunStatus, err := exec.StatusFromJSON(mockResponseWriter.body)
		require.NoError(t, err)
		require.Equal(t, core.Running, dagRunStatus.Status)

		// Stop the DAG
		dagAgent.Abort()

		// Give the process a moment to handle the signal and update status
		time.Sleep(100 * time.Millisecond)

		dag.AssertLatestStatus(t, core.Aborted)
	})
	t.Run("HTTPInvalidRequest", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.DAG(t, `steps:
  - "sleep 10"
`)
		dagAgent := dag.Agent()

		go func() {
			dagAgent.RunCancel(t)
		}()

		// Wait for the DAG to start
		dag.AssertLatestStatus(t, core.Running)

		var mockResponseWriter = mockResponseWriter{}

		// Request with an invalid path
		dagAgent.HandleHTTP(th.Context)(&mockResponseWriter, &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/invalid-path"},
		})
		require.Equal(t, http.StatusNotFound, mockResponseWriter.status)

		// Stop the DAG
		dagAgent.Abort()
		dag.AssertLatestStatus(t, core.Aborted)
	})
	t.Run("HTTPHandleCancel", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.DAG(t, `steps:
  - "sleep 10"
`)
		dagAgent := dag.Agent()

		done := make(chan struct{})
		go func() {
			dagAgent.RunCancel(t)
			close(done)
		}()

		// Wait for the DAG to start
		dag.AssertLatestStatus(t, core.Running)

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
		dag.AssertLatestStatus(t, core.Aborted)
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

func TestAgent_OutputCollection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dag      string
		expected map[string]string
	}{
		{
			name: "SimpleOutput",
			dag: `steps:
  - name: step1
    command: echo "hello"
    output: RESULT`,
			expected: map[string]string{"result": "hello"},
		},
		{
			name: "CamelCaseConversion",
			dag: `steps:
  - name: step1
    command: echo "value"
    output: MY_OUTPUT_VAR`,
			expected: map[string]string{"myOutputVar": "value"},
		},
		{
			name: "CustomOutputKey",
			dag: `steps:
  - name: step1
    command: echo "value"
    output:
      name: RESULT
      key: customKey`,
			expected: map[string]string{"customKey": "value"},
		},
		{
			name: "OmitExcludesFromOutputs",
			dag: `steps:
  - name: step1
    command: echo "visible"
    output: VISIBLE
  - name: step2
    command: echo "hidden"
    output:
      name: HIDDEN
      omit: true`,
			expected: map[string]string{"visible": "visible"},
		},
		{
			name: "MultipleSteps",
			dag: `steps:
  - name: step1
    command: echo "one"
    output: OUTPUT_ONE
  - name: step2
    command: echo "two"
    output: OUTPUT_TWO`,
			expected: map[string]string{"outputOne": "one", "outputTwo": "two"},
		},
		{
			name: "LastOneWins",
			dag: `steps:
  - name: step1
    command: echo "first"
    output: RESULT
  - name: step2
    command: echo "second"
    output: RESULT
    depends: [step1]`,
			expected: map[string]string{"result": "second"},
		},
		{
			name: "NoOutputs",
			dag: `steps:
  - name: step1
    command: "true"`,
			expected: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			th := test.Setup(t)
			dag := th.DAG(t, tc.dag)
			dagAgent := dag.Agent()
			dagAgent.RunSuccess(t)

			outputs := dag.ReadOutputs(t)
			for k, v := range tc.expected {
				require.Equal(t, v, outputs[k], "output %s mismatch", k)
			}
			require.Len(t, outputs, len(tc.expected))
		})
	}
}

func TestAgent_OutputSecretMasking(t *testing.T) {
	t.Parallel()
	th := test.Setup(t)

	secretValue := "super-secret-token-xyz123"
	secretFile := th.TempFile(t, "secret.txt", []byte(secretValue))

	dag := th.DAG(t, `
secrets:
  - name: API_TOKEN
    provider: file
    key: `+secretFile+`
steps:
  - name: step1
    command: echo "Token is ${API_TOKEN}"
    output: RESPONSE`)

	dagAgent := dag.Agent()
	dagAgent.RunSuccess(t)

	outputs := dag.ReadOutputs(t)
	require.NotContains(t, outputs["response"], secretValue, "secret should be masked")
	require.Contains(t, outputs["response"], "*******", "masked placeholder expected")
}
