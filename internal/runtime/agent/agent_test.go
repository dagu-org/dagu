// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/cmdutil"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/runtime/agent"
	"github.com/dagucloud/dagu/internal/test"

	"github.com/stretchr/testify/require"
)

func agentCommandEntry(command string) core.CommandEntry {
	cmd, args, err := cmdutil.SplitCommand(command)
	if err != nil {
		panic(fmt.Errorf("failed to parse command %q: %w", command, err))
	}
	return core.CommandEntry{
		Command:     cmd,
		Args:        args,
		CmdWithArgs: command,
	}
}

func setAllAgentStepCommands(dag *core.DAG, command string) {
	entry := agentCommandEntry(command)
	for i := range dag.Steps {
		dag.Steps[i].Commands = []core.CommandEntry{entry}
	}
}

func TestAgent_Run(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Parallel()
	}

	t.Run("RunDAG", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, fmt.Sprintf(`steps:
  - %q
`, test.PortableSleepCommand(time.Second)))
		dagAgent := dag.Agent()

		dag.AssertLatestStatus(t, core.NotStarted)

		runDone := make(chan error, 1)
		go func() {
			runDone <- dagAgent.Run(th.Context)
		}()

		runTimeout := 10 * time.Second
		if runtime.GOOS == "windows" {
			runTimeout = 90 * time.Second
		}

		select {
		case err := <-runDone:
			require.NoError(t, err)
		case <-time.After(runTimeout):
			t.Fatalf("timed out waiting for DAG run to finish after %s", runTimeout)
		}

		dag.AssertLatestStatus(t, core.Succeeded)
	})
	t.Run("DeleteOldHistory", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, fmt.Sprintf(`steps:
  - %q
`, test.PortableSleepCommand(time.Second)))
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
		releaseFile := filepath.Join(t.TempDir(), "release")
		dag := th.DAG(t, fmt.Sprintf(`steps:
  - name: wait-until-released
    command: %q
`, test.PortableWaitForFileScript(releaseFile, 50*time.Millisecond)))
		dagAgent := dag.Agent(test.WithDAGRunID("test-dag-run"))
		done := make(chan struct{})

		go func() {
			// Run the DAG in the background so that it is running
			dagAgent.RunSuccess(t)
			close(done)
		}()

		require.Eventually(t, func() bool {
			status, err := th.DAGRunMgr.GetCurrentStatus(context.Background(), dag.DAG, "test-dag-run")
			if err != nil || status == nil || status.Status != core.Running {
				return false
			}
			return th.DAGRunMgr.IsRunning(context.Background(), dag.DAG, "test-dag-run")
		}, 2*time.Second, 50*time.Millisecond, "DAG should be running")

		require.NoError(t, os.WriteFile(releaseFile, []byte("ok"), 0600))

		// Wait for the DAG to finish
		<-done
	})
	t.Run("PreConditionNotMet", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, fmt.Sprintf(`steps:
  - %q
  - %q
`, test.PortableSuccessCommand(), test.PortableSuccessCommand()))

		// Set a precondition that always fails
		dag.Preconditions = []*core.Condition{
			{Condition: test.PortableCommandSubstitution(test.PortableOutputCommand("1")), Expected: "0"},
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
		errDAG := th.DAG(t, fmt.Sprintf(`steps:
  - %q
`, test.PortableFailureCommand()))
		dagAgent := errDAG.Agent()
		dagAgent.RunError(t)

		// Check if the status is saved correctly
		require.Equal(t, core.Failed, dagAgent.Status(th.Context).Status)
	})
	t.Run("InitFailurePersistsFinishedAt", func(t *testing.T) {
		th := test.Setup(t)
		blockingFile := filepath.Join(t.TempDir(), "not-a-dir")
		require.NoError(t, os.WriteFile(blockingFile, []byte("x"), 0600))

		dag := th.DAG(t, fmt.Sprintf(`working_dir: %q
steps:
  - "echo hi"
`, blockingFile+string(os.PathSeparator)+"subdir"))
		dagAgent := dag.Agent()

		err := dagAgent.Run(th.Context)
		require.ErrorContains(t, err, "failed to create working directory")

		latest, readErr := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, readErr)
		require.Equal(t, core.Failed, latest.Status)
		require.NotEmpty(t, latest.FinishedAt)
	})
	t.Run("FailureHandlerRunsInline", func(t *testing.T) {
		th := test.Setup(t)
		marker := filepath.Join(t.TempDir(), "failure-marker")
		dag := th.DAG(t, fmt.Sprintf(`handler_on:
  failure:
    command: %q
steps:
  - %q
`, test.PortableWriteFileCommand(marker, "failed"), test.PortableFailureCommand()))
		dagAgent := dag.Agent()
		dagAgent.RunError(t)

		status := dagAgent.Status(th.Context)
		require.Equal(t, core.Failed, status.Status)
		require.NotNil(t, status.OnFailure)
		require.Equal(t, core.NodeSucceeded, status.OnFailure.Status)
		require.NotEmpty(t, status.StartedAt)
		require.NotEmpty(t, status.FinishedAt)

		data, err := os.ReadFile(marker)
		require.NoError(t, err)
		require.Equal(t, "failed", string(data))
	})
	t.Run("FinishWithTimeout", func(t *testing.T) {
		th := test.Setup(t)
		timeoutDAG := th.DAG(t, fmt.Sprintf(`timeout_sec: 2
steps:
  - %q
  - %q
`, test.PortableSleepCommand(time.Second), test.PortableSleepCommand(2*time.Second)))
		dagAgent := timeoutDAG.Agent()
		dagAgent.RunError(t)

		// Check if the status is saved correctly
		require.Equal(t, core.Failed, dagAgent.Status(th.Context).Status)
	})
	t.Run("ReceiveSignal", func(t *testing.T) {
		th := test.Setup(t)
		releaseFile := filepath.Join(t.TempDir(), "release")
		t.Cleanup(func() {
			_ = os.WriteFile(releaseFile, []byte("ok"), 0600)
		})
		dag := th.DAG(t, fmt.Sprintf(`steps:
  - %q
`, test.PortableWaitForFileScript(releaseFile, 50*time.Millisecond)))
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

		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("timed out waiting for DAG cancellation")
		}

		// wait for the DAG to be canceled
		dag.AssertLatestStatus(t, core.Aborted)
	})
	t.Run("ExitHandler", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, fmt.Sprintf(`handler_on:
  exit:
    command: %q
steps:
  - %q
  - %q
`, test.PortableSuccessCommand(), test.PortableSuccessCommand(), test.PortableSuccessCommand()))
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

func TestAgent_WorkingDirExpansion(t *testing.T) {
	t.Run("WorkingDirWithEnvVar", func(t *testing.T) {
		th := test.Setup(t)
		// Set up a temp directory and env var
		tempDir := t.TempDir()
		t.Setenv("TEST_WORK_DIR", tempDir)

		// Create DAG with WorkingDir using env var
		dag := th.DAG(t, `working_dir: $TEST_WORK_DIR
steps:
  - name: check-pwd
    command: `+test.PortablePwdCommand()+`
`)
		dagAgent := dag.Agent()
		dagAgent.RunSuccess(t)

		// Verify the DAG ran successfully
		dagRunStatus := dagAgent.Status(th.Context)
		require.Equal(t, core.Succeeded.String(), dagRunStatus.Status.String())
	})

	t.Run("WorkingDirWithDAGEnvVar", func(t *testing.T) {
		th := test.Setup(t)
		tempDir := t.TempDir()
		origWD, err := os.Getwd()
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.Chdir(origWD)
		})

		// Create DAG with WorkingDir using DAG-defined env var
		dag := th.DAG(t, `env:
  - CUSTOM_DIR=`+tempDir+`
working_dir: $CUSTOM_DIR
steps:
  - name: check-pwd
    command: `+test.PortablePwdCommand()+`
`)
		dagAgent := dag.Agent()
		dagAgent.RunSuccess(t)

		// Verify the DAG ran successfully
		dagRunStatus := dagAgent.Status(th.Context)
		require.Equal(t, core.Succeeded.String(), dagRunStatus.Status.String())
	})

	t.Run("WorkingDirWithTildeExpansion", func(t *testing.T) {
		th := test.Setup(t)

		// Create DAG with WorkingDir using tilde
		dag := th.DAG(t, `working_dir: ~
steps:
  - name: check-pwd
    command: `+test.PortablePwdCommand()+`
`)
		dagAgent := dag.Agent()
		dagAgent.RunSuccess(t)

		// Verify the DAG ran successfully
		dagRunStatus := dagAgent.Status(th.Context)
		require.Equal(t, core.Succeeded.String(), dagRunStatus.Status.String())
	})
}

func TestAgent_DryRun(t *testing.T) {
	t.Run("DryRun", func(t *testing.T) {
		th := test.Setup(t)

		dag := th.DAG(t, fmt.Sprintf(`steps:
  - %q
`, test.PortableSuccessCommand()))
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
		dag := th.DAG(t, fmt.Sprintf(`type: graph
steps:
  - name: "1"
    command: %q
  - name: "2"
    command: %q
    continue_on:
      failure: true
    depends: ["1"]
  - name: "3"
    command: %q
    depends: ["2"]
  - name: "4"
    command: %q
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
    continue_on:
      skipped: true
  - name: "5"
    command: %q
    depends: ["4"]
  - name: "6"
    command: %q
    depends: ["5"]
  - name: "7"
    command: %q
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
    depends: ["6"]
    continue_on:
      skipped: true
  - name: "8"
    command: %q
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
  - name: "9"
    command: %q
`, test.PortableSuccessCommand(), test.PortableFailureCommand(), test.PortableSuccessCommand(), test.PortableSuccessCommand(), test.PortableFailureCommand(), test.PortableSuccessCommand(), test.PortableSuccessCommand(), test.PortableSuccessCommand(), test.PortableFailureCommand()))
		dagAgent := dag.Agent()

		dagAgent.RunError(t)
		require.Equal(t, 0, dagAgent.Status(th.Context).AutoRetryCount)

		// Modify the DAG to make it successful
		dagRunStatus := dagAgent.Status(th.Context)
		setAllAgentStepCommands(dag.DAG, test.PortableSuccessCommand())

		// Retry the DAG and check if it is successful
		dagAgent = dag.Agent(test.WithAgentOptions(agent.Options{
			RetryTarget: &dagRunStatus,
		}))
		dagAgent.RunSuccess(t)
		require.Equal(t, 0, dagAgent.Status(th.Context).AutoRetryCount)

		for _, node := range dagAgent.Status(th.Context).Nodes {
			if node.Status != core.NodeSucceeded &&
				node.Status != core.NodeSkipped {
				t.Errorf("node %q is not successful: %s", node.Step.Name, node.Status)
			}
		}
	})

	t.Run("StepRetry", func(t *testing.T) {
		th := test.Setup(t)
		dag := th.DAG(t, fmt.Sprintf(`type: graph
steps:
  - name: "1"
    command: %q
  - name: "2"
    command: %q
    continue_on:
      failure: true
    depends: ["1"]
  - name: "3"
    command: %q
    depends: ["2"]
  - name: "4"
    command: %q
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
    continue_on:
      skipped: true
  - name: "5"
    command: %q
    depends: ["4"]
  - name: "6"
    command: %q
    depends: ["5"]
  - name: "7"
    command: %q
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
    depends: ["6"]
    continue_on:
      skipped: true
  - name: "8"
    command: %q
    preconditions:
      - condition: "`+"`"+`echo 0`+"`"+`"
        expected: "1"
  - name: "9"
    command: %q
`, test.PortableSuccessCommand(), test.PortableFailureCommand(), test.PortableSuccessCommand(), test.PortableSuccessCommand(), test.PortableFailureCommand(), test.PortableSuccessCommand(), test.PortableSuccessCommand(), test.PortableSuccessCommand(), test.PortableFailureCommand()))
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
		setAllAgentStepCommands(dag.DAG, test.PortableSuccessCommand())

		// Wait until the current time (RFC3339, second precision) differs
		// from the previous FinishedAt timestamps so that retried steps
		// are guaranteed to have a newer formatted timestamp.
		prev := time.Now().Format(time.RFC3339)
		for time.Now().Format(time.RFC3339) == prev {
			time.Sleep(10 * time.Millisecond)
		}

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
		dag := th.DAG(t, fmt.Sprintf(`steps:
  - %q
`, test.PortableSleepCommand(10*time.Second)))
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

		dag.AssertLatestStatus(t, core.Aborted)
	})
	t.Run("HTTPInvalidRequest", func(t *testing.T) {
		t.Parallel()
		th := test.Setup(t)

		// Start a long-running DAG
		dag := th.DAG(t, fmt.Sprintf(`steps:
  - %q
`, test.PortableSleepCommand(10*time.Second)))
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
		dag := th.DAG(t, fmt.Sprintf(`steps:
  - %q
`, test.PortableSleepCommand(10*time.Second)))
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
			dag: `type: graph
steps:
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
			dag: fmt.Sprintf(`steps:
  - name: step1
    command: %q`, test.PortableSuccessCommand()),
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

func TestAgent_SubDAGRunVisibleWhileRunning(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)
	releaseFile := filepath.Join(t.TempDir(), "release-child")
	t.Cleanup(func() {
		_ = os.WriteFile(releaseFile, []byte("done"), 0600)
	})

	// Hold the child open until the test explicitly releases it so Windows can
	// observe persisted SubRuns without racing the child completion.
	th.CreateDAGFile(t, th.Config.Paths.DAGsDir, "child-slow", fmt.Appendf(nil, `
steps:
  - name: slow-step
    command: %q
`, test.PortableWaitForFileScript(releaseFile, 100*time.Millisecond)))

	// The preceding step must run long enough for the one-shot 100ms status timer
	// to fire (and exhaust itself) BEFORE run-child starts. This replicates the
	// production scenario where the bug manifests.
	parent := th.DAG(t, fmt.Sprintf(`
type: graph
steps:
  - name: pre-step
    command: %q
  - name: run-child
    call: child-slow
    depends:
      - pre-step
`, test.PortableSleepCommand(time.Second)))

	a := parent.Agent()
	runErr := make(chan error, 1)
	go func() {
		runErr <- a.Run(parent.Context)
	}()

	// SubRuns must be visible in the *stored* status BEFORE the child DAG completes.
	// We use ListRecentStatus which reads from the status.jsonl file on disk, not from
	// the live socket, so it accurately reflects what the API handler would return.
	// Before the fix, this would never become true because SetSubRuns() was called
	// after the progressCh notification, so the children field was never written
	// to status.jsonl while the subdag was running.
	require.Eventually(t, func() bool {
		statuses := th.DAGRunMgr.ListRecentStatus(th.Context, parent.Name, 1)
		if len(statuses) == 0 || statuses[0].Status != core.Running {
			return false
		}
		for _, node := range statuses[0].Nodes {
			if node.Step.Name == "run-child" && node.Status == core.NodeRunning {
				return len(node.SubRuns) > 0
			}
		}
		return false
	}, subDAGVisibleTimeout(), 100*time.Millisecond,
		"SubRuns must be present in stored status while subDAG step is running")

	require.NoError(t, os.WriteFile(releaseFile, []byte("done"), 0600))
	require.NoError(t, <-runErr)
}

func subDAGVisibleTimeout() time.Duration {
	if runtime.GOOS == "windows" {
		return 90 * time.Second
	}
	return 10 * time.Second
}
