// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec032_agent_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// TestAgentBasicDecisionLoopSucceeds proves the core decision loop actually
// runs end to end: the model selects a declared step's tool, observes its
// outcome, then settles the one open task with set_task_status, and the run
// concludes succeeded.
func TestAgentBasicDecisionLoopSucceeds(t *testing.T) {
	t.Parallel()

	llm := newFakeLLM(t, []scriptedTurn{
		{tool: "do_work", args: "{}"},
		{tool: "set_task_status", args: setTaskStatusArgs("finish", "completed", "do_work ran")},
	})

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv(llm.env(), "start", "runtime_basic.yaml")
	result.ExpectExitCode(0)
	dagu.ExpectFileContent("work.out", "run\n")
}

// TestAgentToolErrorsDoNotFailRun proves that a set_task_status call naming
// an unknown task is reported back as a tool error without failing the run,
// per "Naming an unknown task ... is reported back as a tool error and the
// loop continues; none of these fail the run."
func TestAgentToolErrorsDoNotFailRun(t *testing.T) {
	t.Parallel()

	llm := newFakeLLM(t, []scriptedTurn{
		{tool: "set_task_status", args: setTaskStatusArgs("no_such_task", "completed", "oops")},
		{tool: "do_work", args: "{}"},
		{tool: "set_task_status", args: setTaskStatusArgs("finish", "completed", "do_work ran")},
	})

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv(llm.env(), "start", "runtime_basic.yaml")
	result.ExpectExitCode(0)
	dagu.ExpectFileContent("work.out", "run\n")
}

// TestAgentRepetitionLimitRefusesSixthAttempt proves "A single action may run
// at most 5 times per DAG run; beyond that, the request is refused as a tool
// error and the agent must choose differently." The model selects the same
// action 6 times in a row; only 5 real executions must land, the 6th must be
// refused without failing the run, and the run must still be able to
// complete afterward.
func TestAgentRepetitionLimitRefusesSixthAttempt(t *testing.T) {
	t.Parallel()

	turns := make([]scriptedTurn, 0, 7)
	for range 6 {
		turns = append(turns, scriptedTurn{tool: "do_work", args: "{}"})
	}
	turns = append(turns, scriptedTurn{tool: "set_task_status", args: setTaskStatusArgs("finish", "completed", "enough attempts")})
	llm := newFakeLLM(t, turns)

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv(llm.env(), "start", "runtime_basic.yaml")
	result.ExpectExitCode(0)
	// Exactly 5 real executions land; the 6th selection is refused as a tool
	// error rather than actually re-running the step a 6th time.
	dagu.ExpectFileContent("work.out", "run\nrun\nrun\nrun\nrun\n")
}

// TestAgentStallingFailsRun proves "A second consecutive reply without a
// tool call fails the run." The first no-tool-call reply is only a reminder;
// the second, consecutive one must fail the run outright, and the action
// must never have run.
func TestAgentStallingFailsRun(t *testing.T) {
	t.Parallel()

	llm := newFakeLLM(t, []scriptedTurn{
		{content: "Thinking about it..."},
		{content: "Still thinking..."},
	})

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv(llm.env(), "start", "runtime_basic.yaml")
	result.ExpectNonZeroExitCode()
	dagu.ExpectNoFile("work.out")
}

// TestAgentPartiallySucceededWhenActionLeftFailed proves terminal-status
// derivation: "no task open and none failed, at least one action left
// failed -> partially succeeded." The agent runs an action that always
// fails, then settles the only task as completed anyway.
func TestAgentPartiallySucceededWhenActionLeftFailed(t *testing.T) {
	t.Parallel()

	llm := newFakeLLM(t, []scriptedTurn{
		{tool: "risky_action", args: "{}"},
		{tool: "set_task_status", args: setTaskStatusArgs("finish", "completed", "accepted despite failure")},
	})

	dagu := harness.NewRunner(t)
	env := append(llm.env(), "DAGU_HOME="+filepath.Join(t.TempDir(), "dagu"))
	const runID = "spec032-partial"
	result := dagu.RunWithEnv(env, "start", "--run-id="+runID, "runtime_partial_failure.yaml")
	result.ExpectExitCode(0)

	status := dagu.RunWithEnv(env, "status", "--run-id="+runID, "runtime_partial_failure.yaml")
	status.ExpectExitCode(0)
	require.Contains(t, status.Stdout(), "Partially Succeeded")
}

// TestAgentAskUserSuspendsAndResumes proves the hardest-to-verify-by-
// inspection behavior in the spec: an ask_user action opens the agent's own
// synthesized human task (id "ask_user"), the run releases its process and
// reports waiting, completing that human task re-queues the same DAG run,
// and the agent resumes with its conversation and goal progress intact,
// receiving the answer as the next turn's observation.
func TestAgentAskUserSuspendsAndResumes(t *testing.T) {
	llm := newFakeLLM(t, []scriptedTurn{
		{tool: "ask_user", args: askUserArgs("Proceed with the work?")},
		{tool: "do_work", args: "{}"},
		{tool: "set_task_status", args: setTaskStatusArgs("finish", "completed", "user approved and work ran")},
	})

	dagu := harness.NewRunner(t)
	schedulerPort := harness.FreePort(t)
	env := append(llm.env(),
		"DAGU_HOME="+filepath.Join(t.TempDir(), "dagu"),
		"DAGU_SCHEDULER_PORT="+strconv.Itoa(schedulerPort),
	)
	const runID = "spec032-ask-user"

	scheduler := dagu.StartWithEnv(env, "scheduler", "--dags="+dagu.ProjectPath("."))
	select {
	case <-scheduler.Done():
		t.Fatalf("scheduler exited during startup: %s", scheduler.FailureOutput())
	case <-time.After(100 * time.Millisecond):
	}

	start := dagu.RunWithEnv(env, "start", "--run-id="+runID, "runtime_ask_user.yaml")
	start.ExpectExitCode(0)

	waitingStatus := waitForAgentStatus(t, dagu, env, runID, "runtime_ask_user.yaml", "Waiting")
	require.Contains(t, waitingStatus.Stdout(), "ask_user")
	require.Contains(t, waitingStatus.Stdout(), "Proceed with the work?")

	complete := dagu.RunWithEnv(env, "human-task", "complete",
		"--run-id="+runID, "--step=ask_user", "--input=answer=yes", "spec032_runtime_ask_user")
	complete.ExpectExitCode(0)

	waitForAgentStatus(t, dagu, env, runID, "runtime_ask_user.yaml", "Succeeded")
	waitForAgentFileContent(t, dagu.ProjectPath("work.out"), "run\n")
}

func waitForAgentStatus(
	t *testing.T,
	dagu *harness.Runner,
	env []string,
	runID, file, status string,
) *harness.Result {
	t.Helper()

	deadline := time.Now().Add(harness.WaitTimeout(t))
	var result *harness.Result
	for time.Now().Before(deadline) {
		result = dagu.RunWithEnv(env, "status", "--run-id="+runID, file)
		if result.ExitCode() == 0 && strings.Contains(result.Stdout(), status) {
			return result
		}
		time.Sleep(50 * time.Millisecond)
	}
	if result == nil {
		t.Fatal("status command was not run")
	}
	t.Fatalf("DAG-run %s did not reach %s\nstdout:\n%s\nstderr:\n%s", runID, status, result.Stdout(), result.Stderr())
	return nil
}

func waitForAgentFileContent(t *testing.T, path, expected string) {
	t.Helper()

	deadline := time.Now().Add(harness.WaitTimeout(t))
	for time.Now().Before(deadline) {
		actual, err := os.ReadFile(path) // #nosec G304 -- the caller supplies a test-project path.
		if err == nil && string(actual) == expected {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("file %s never reached content %q", path, expected)
}
