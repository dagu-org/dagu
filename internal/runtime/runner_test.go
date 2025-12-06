package runtime_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner(t *testing.T) {
	testScript := test.TestdataPath(t, filepath.Join("runtime", "runner", "testfile.sh"))

	t.Run("SequentialStepsSuccess", func(t *testing.T) {
		t.Parallel()
		r := setupRunner(t, withMaxActiveRuns(1))

		// 1 -> 2 -> 3
		plan := r.newPlan(t,
			successStep("1"),
			successStep("2", "1"),
			successStep("3", "2"),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
	})
	t.Run("SequentialStepsWithFailure", func(t *testing.T) {
		t.Parallel()
		r := setupRunner(t, withMaxActiveRuns(1))

		// 1 -> 2 -> 3 -> 4
		plan := r.newPlan(t,
			successStep("1"),
			successStep("2", "1"),
			failStep("3", "2"),
			successStep("4", "3"),
		)

		result := plan.assertRun(t, core.Failed)

		// 1, 2, 3 should be executed and 4 should be canceled because 3 failed
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
		result.assertNodeStatus(t, "3", core.NodeFailed)
		result.assertNodeStatus(t, "4", core.NodeAborted)
	})
	t.Run("ParallelSteps", func(t *testing.T) {
		t.Parallel()
		r := setupRunner(t, withMaxActiveRuns(3))

		// 1,2,3
		plan := r.newPlan(t,
			successStep("1"),
			successStep("2"),
			successStep("3"),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
	})
	t.Run("ParallelStepsWithFailure", func(t *testing.T) {
		r := setupRunner(t)

		// 1 -> 3 -> 4, 2 (fail)
		plan := r.newPlan(t,
			successStep("1"),
			failStep("2"),
			successStep("3", "1"),
			successStep("4", "3"),
		)

		result := plan.assertRun(t, core.Failed)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeFailed)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
		result.assertNodeStatus(t, "4", core.NodeSucceeded)
	})
	t.Run("ComplexCommand", func(t *testing.T) {
		t.Parallel()
		r := setupRunner(t, withMaxActiveRuns(1))

		plan := r.newPlan(t,
			newStep("1",
				withCommand("df / | awk 'NR==2 {exit $4 > 5000 ? 0 : 1}'"),
			))

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
	})
	t.Run("ContinueOnFailure", func(t *testing.T) {
		r := setupRunner(t)

		// 1 -> 2 (fail) -> 3
		plan := r.newPlan(t,
			successStep("1"),
			newStep("2",
				withDepends("1"),
				withCommand("false"),
				withContinueOn(core.ContinueOn{
					Failure: true,
				}),
			),
			successStep("3", "2"),
		)

		result := plan.assertRun(t, core.PartiallySucceeded)

		// 1, 2, 3 should be executed even though 2 failed
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeFailed)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
	})
	t.Run("ContinueOnSkip", func(t *testing.T) {
		r := setupRunner(t)

		// 1 -> 2 (skip) -> 3
		plan := r.newPlan(t,
			successStep("1"),
			newStep("2",
				withDepends("1"),
				withCommand("false"),
				withPrecondition(&core.Condition{
					Condition: "`echo 1`",
					Expected:  "0",
				}),
				withContinueOn(core.ContinueOn{
					Skipped: true,
				}),
			),
			successStep("3", "2"),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSkipped)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
	})
	t.Run("ContinueOnExitCode", func(t *testing.T) {
		r := setupRunner(t)

		// 1 (exit code 1) -> 2
		plan := r.newPlan(t,
			newStep("1",
				withCommand("false"),
				withContinueOn(core.ContinueOn{
					ExitCode: []int{1},
				}),
			),
			successStep("2", "1"),
		)

		result := plan.assertRun(t, core.PartiallySucceeded)

		// 1, 2 should be executed even though 1 failed
		result.assertNodeStatus(t, "1", core.NodeFailed)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
	})
	t.Run("ContinueOnOutputStdout", func(t *testing.T) {
		r := setupRunner(t)

		// 1 (exit code 1) -> 2
		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo test_output; false"), // stdout: test_output
				withContinueOn(core.ContinueOn{
					Output: []string{
						"test_output",
					},
				}),
			),
			successStep("2", "1"),
		)

		result := plan.assertRun(t, core.PartiallySucceeded)

		// 1, 2 should be executed even though 1 failed
		result.assertNodeStatus(t, "1", core.NodeFailed)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
	})
	t.Run("ContinueOnOutputStderr", func(t *testing.T) {
		r := setupRunner(t)

		// 1 (exit code 1) -> 2
		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo test_output >&2; echo test_output; false"), // write to stderr and stdout
				withContinueOn(core.ContinueOn{
					Output: []string{
						"test_output",
					},
				}),
			),
			successStep("2", "1"),
		)

		result := plan.assertRun(t, core.PartiallySucceeded)

		// Step 1 fails but matches continueOn output, allowing step 2 to run
		result.assertNodeStatus(t, "1", core.NodeFailed)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		stderrData, err := os.ReadFile(node.GetStderr())
		require.NoError(t, err)
		assert.Contains(t, string(stderrData), "test_output")
	})
	t.Run("ContinueOnOutputRegexp", func(t *testing.T) {
		r := setupRunner(t)

		// 1 (exit code 1) -> 2
		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo test_output; false"), // stdout: test_output
				withContinueOn(core.ContinueOn{
					Output: []string{
						"re:^test_[a-z]+$",
					},
				}),
			),
			successStep("2", "1"),
		)

		result := plan.assertRun(t, core.PartiallySucceeded)

		// 1, 2 should be executed even though 1 failed
		result.assertNodeStatus(t, "1", core.NodeFailed)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
	})
	t.Run("ContinueOnMarkSuccess", func(t *testing.T) {
		r := setupRunner(t)

		// 1 (exit code 1) -> 2
		plan := r.newPlan(t,
			newStep("1",
				withCommand("false"),
				withContinueOn(core.ContinueOn{
					ExitCode:    []int{1},
					MarkSuccess: true,
				}),
			),
			successStep("2", "1"),
		)

		result := plan.assertRun(t, core.Succeeded)

		// 1, 2 should be executed even though 1 failed
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
	})
	t.Run("Cancel", func(t *testing.T) {
		r := setupRunner(t)

		// 1 -> 2 (cancel when running) -> 3 (should not be executed)
		plan := r.newPlan(t,
			successStep("1"),
			newStep("2", withDepends("1"), withCommand("sleep 0.5")),
			failStep("3", "2"),
		)

		go func() {
			time.Sleep(time.Millisecond * 200) // wait for step 2 to start
			plan.cancel(t)
		}()

		result := plan.assertRun(t, core.Aborted)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeAborted)
		result.assertNodeStatus(t, "3", core.NodeNotStarted)
	})
	t.Run("Timeout", func(t *testing.T) {
		r := setupRunner(t, withTimeout(time.Millisecond*500))

		// 1 -> 2 (timeout) -> 3 (should not be executed)
		plan := r.newPlan(t,
			newStep("1", withCommand("sleep 0.1")),
			newStep("2", withCommand("sleep 0.5"), withDepends("1")),
			successStep("3", "2"),
		)

		result := plan.assertRun(t, core.Failed)

		// 1 should be executed and 2 should be canceled because of timeout
		// 3 should not be executed and should be canceled
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeAborted)
		result.assertNodeStatus(t, "3", core.NodeAborted)
	})
	t.Run("RetryPolicyFail", func(t *testing.T) {
		const file = "flag_test_retry_fail"

		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1",
				withCommand(fmt.Sprintf("%s %s", testScript, file)),
				withRetryPolicy(2, 0),
			),
		)

		result := plan.assertRun(t, core.Failed)

		result.assertNodeStatus(t, "1", core.NodeFailed)

		node := result.nodeByName(t, "1")
		require.Equal(t, 2, node.State().RetryCount) // 2 retry
	})
	t.Run("RetryWithScript", func(t *testing.T) {
		r := setupRunner(t)
		tmpDir := t.TempDir()
		testFile := path.Join(tmpDir, "testfile.txt")

		plan := r.newPlan(t,
			newStep("1",
				withScript(`
					if [ ! -f "`+testFile+`" ]; then
						touch `+testFile+`
						exit 1
					fi
					exit 0
				`),
				withRetryPolicy(1, time.Millisecond*50),
			),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		require.Equal(t, 1, node.State().DoneCount)  // 1 successful execution
		require.Equal(t, 1, node.State().RetryCount) // 1 retry
	})
	t.Run("RetryPolicySuccess", func(t *testing.T) {
		file := filepath.Join(
			os.TempDir(), fmt.Sprintf("flag_test_retry_success_%s", uuid.Must(uuid.NewV7()).String()),
		)

		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1",
				withCommand(fmt.Sprintf("%s %s", testScript, file)),
				withRetryPolicy(3, time.Millisecond*50),
			),
		)

		go func() {
			// Create file for successful retry
			time.Sleep(time.Millisecond * 60) // wait for step 1 to start

			// Create file during the retry interval
			f, err := os.Create(file)
			require.NoError(t, err)
			defer func() {
				_ = f.Close()
			}()

			t.Cleanup(func() {
				_ = os.Remove(file)
			})
		}()

		result := plan.assertRun(t, core.Succeeded)

		// Check if the retry is successful
		state := result.nodeByName(t, "1").State()
		assert.Equal(t, 1, state.DoneCount)
		assert.Greater(t, state.RetryCount, 0)
		assert.NotEmpty(t, state.RetriedAt)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
	})
	t.Run("PreconditionMatch", func(t *testing.T) {
		t.Parallel()
		r := setupRunner(t)

		// 1 -> 2 (precondition match) -> 3
		plan := r.newPlan(t,
			successStep("1"),
			newStep("2", withCommand("echo 2"),
				withPrecondition(&core.Condition{
					Condition: "`echo 1`",
					Expected:  "1",
				}),
			),
			successStep("3", "2"),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
	})
	t.Run("PreconditionNotMatch", func(t *testing.T) {
		r := setupRunner(t)

		// 1 -> 2 (precondition not match) -> 3
		plan := r.newPlan(t,
			successStep("1"),
			newStep("2", withCommand("echo 2"),
				withPrecondition(&core.Condition{
					Condition: "`echo 1`",
					Expected:  "0",
				})),
			successStep("3", "2"),
		)

		result := plan.assertRun(t, core.Succeeded)

		// 1 should be executed and 2, 3 should be skipped
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSkipped)
		result.assertNodeStatus(t, "3", core.NodeSkipped)
	})
	t.Run("PreconditionWithCommandMet", func(t *testing.T) {
		r := setupRunner(t)

		// 1 -> 2 (precondition not match) -> 3
		plan := r.newPlan(t,
			successStep("1"),
			newStep("2", withCommand("echo 2"),
				withPrecondition(&core.Condition{
					Condition: "true",
				})),
			successStep("3", "2"),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
	})
	t.Run("PreconditionWithCommandNotMet", func(t *testing.T) {
		r := setupRunner(t)

		// 1 -> 2 (precondition not match) -> 3
		plan := r.newPlan(t,
			successStep("1"),
			newStep("2", withCommand("echo 2"),
				withPrecondition(&core.Condition{
					Condition: "false",
				})),
			successStep("3", "2"),
		)

		result := plan.assertRun(t, core.Succeeded)

		// 1 should be executed and 2, 3 should be skipped
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSkipped)
		result.assertNodeStatus(t, "3", core.NodeSkipped)
	})
	t.Run("OnExitHandler", func(t *testing.T) {
		r := setupRunner(t, withOnExit(successStep("onExit")))

		plan := r.newPlan(t, successStep("1"))

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "onExit", core.NodeSucceeded)
	})
	t.Run("OnExitHandlerFail", func(t *testing.T) {
		r := setupRunner(t, withOnExit(failStep("onExit")))

		plan := r.newPlan(t, successStep("1"))

		// Overall status should be error because onExit failed
		result := plan.assertRun(t, core.Failed)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "onExit", core.NodeFailed)
	})
	t.Run("OnCancelHandler", func(t *testing.T) {
		r := setupRunner(t, withOnCancel(successStep("onCancel")))

		plan := r.newPlan(t,
			newStep("1", withCommand("sleep 0.5")),
		)

		go func() {
			time.Sleep(time.Millisecond * 30) // wait for step 1 to start
			plan.signal(syscall.SIGTERM)
		}()

		result := plan.assertRun(t, core.Aborted)

		result.assertNodeStatus(t, "1", core.NodeAborted)
		result.assertNodeStatus(t, "onCancel", core.NodeSucceeded)
	})
	t.Run("OnSuccessHandler", func(t *testing.T) {
		r := setupRunner(t, withOnSuccess(successStep("onSuccess")))

		plan := r.newPlan(t, successStep("1"))

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "onSuccess", core.NodeSucceeded)
	})
	t.Run("OnFailureHandler", func(t *testing.T) {
		r := setupRunner(t, withOnFailure(successStep("onFailure")))

		plan := r.newPlan(t, failStep("1"))

		result := plan.assertRun(t, core.Failed)

		result.assertNodeStatus(t, "1", core.NodeFailed)
		result.assertNodeStatus(t, "onFailure", core.NodeSucceeded)
	})
	t.Run("CancelOnSignal", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withCommand("sleep 0.5")),
		)

		go func() {
			time.Sleep(time.Millisecond * 30) // wait for step 1 to start
			plan.signal(syscall.SIGTERM)
		}()

		result := plan.assertRun(t, core.Aborted)

		result.assertNodeStatus(t, "1", core.NodeAborted)
	})
	t.Run("Repeat", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1",
				withCommand("sleep 0.1"),
				withRepeatPolicy(true, time.Millisecond*100),
			),
		)

		go func() {
			time.Sleep(time.Millisecond * 250)
			plan.cancel(t)
		}()

		result := plan.assertRun(t, core.Aborted)

		// 1 should be repeated 2 times
		result.assertNodeStatus(t, "1", core.NodeAborted)

		node := result.nodeByName(t, "1")
		// done count should be 1 because 2nd execution is canceled
		require.Equal(t, 1, node.State().DoneCount)
	})
	t.Run("RepeatFail", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1",
				withCommand("false"),
				withRepeatPolicy(true, time.Millisecond*50),
			),
		)

		result := plan.assertRun(t, core.Failed)

		// Done count should be 1 because it failed and not repeated
		result.assertNodeStatus(t, "1", core.NodeFailed)

		node := result.nodeByName(t, "1")
		require.Equal(t, 1, node.State().DoneCount)
	})
	t.Run("StopRepetitiveTaskGracefully", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1",
				withCommand("sleep 0.1"),
				withRepeatPolicy(true, time.Millisecond*50),
			),
		)

		done := make(chan struct{})
		go func() {
			time.Sleep(time.Millisecond * 100)
			plan.signal(syscall.SIGTERM)
			close(done)
		}()

		result := plan.assertRun(t, core.Succeeded)
		<-done

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
	})
	t.Run("WorkingDirNoExist", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withWorkingDir("/nonexistent"),
				withScript("echo 1"),
			),
		)

		result := plan.assertRun(t, core.Failed)

		result.assertNodeStatus(t, "1", core.NodeFailed)

		require.Contains(t, result.Error.Error(), "no such file or directory")
	})
	t.Run("OutputVariables", func(t *testing.T) {
		t.Parallel()
		r := setupRunner(t)

		// 1: echo hello > OUT
		// 2: echo $OUT > RESULT
		plan := r.newPlan(t,
			newStep("1", withCommand("echo hello"), withOutput("OUT")),
			newStep("2", withCommand("echo $OUT"), withDepends("1"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)

		node := result.nodeByName(t, "2")

		// check if RESULT variable is set to "hello"
		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=hello", output, "expected output %q, got %q", "hello", output)
	})
	t.Run("OutputInheritance", func(t *testing.T) {
		r := setupRunner(t)

		// 1: echo hello > OUT
		// 2: echo world > OUT2 (depends on 1)
		// 3: echo $OUT $OUT2 > RESULT (depends on 2)
		// RESULT should be "hello world"
		plan := r.newPlan(t,
			newStep("1", withCommand("echo hello"), withOutput("OUT")),
			newStep("2", withCommand("echo world"), withOutput("OUT2"), withDepends("1")),
			newStep("3", withCommand("echo $OUT $OUT2"), withDepends("2"), withOutput("RESULT")),
			newStep("4", withCommand("sleep 0.1")),
			// 5 should not have reference to OUT or OUT2
			newStep("5", withCommand("echo $OUT $OUT2"), withDepends("4"), withOutput("RESULT2")),
		)

		result := plan.assertRun(t, core.Succeeded)

		node := result.nodeByName(t, "3")
		output, _ := node.NodeData().State.OutputVariables.Load("RESULT")
		require.Equal(t, "RESULT=hello world", output, "expected output %q, got %q", "hello world", output)

		node2 := result.nodeByName(t, "5")
		output2, _ := node2.NodeData().State.OutputVariables.Load("RESULT2")
		require.Equal(t, "RESULT2=", output2, "expected output %q, got %q", "", output)
	})
	t.Run("OutputJSONReference", func(t *testing.T) {
		r := setupRunner(t)

		jsonData := `{"key": "value"}`
		plan := r.newPlan(t,
			newStep("1", withCommand(fmt.Sprintf("echo '%s'", jsonData)), withOutput("OUT")),
			newStep("2", withCommand("echo ${OUT.key}"), withDepends("1"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)

		// check if RESULT variable is set to "value"
		node := result.nodeByName(t, "2")

		output, _ := node.NodeData().State.OutputVariables.Load("RESULT")
		require.Equal(t, "RESULT=value", output, "expected output %q, got %q", "value", output)
	})
	t.Run("HandlingJSONWithSpecialChars", func(t *testing.T) {
		r := setupRunner(t)

		jsonData := `{\n\t"key": "value"\n}`
		plan := r.newPlan(t,
			newStep("1", withCommand(fmt.Sprintf("echo '%s'", jsonData)), withOutput("OUT")),
			newStep("2", withCommand("echo '${OUT.key}'"), withDepends("1"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)

		// check if RESULT variable is set to "value"
		node := result.nodeByName(t, "2")

		output, _ := node.NodeData().State.OutputVariables.Load("RESULT")
		require.Equal(t, "RESULT=value", output, "expected output %q, got %q", "value", output)
	})
	t.Run("SpecialVarsDAGRUNLOGFILE", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withCommand("echo $DAG_RUN_LOG_FILE"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)
		node := result.nodeByName(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.log$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGRUNSTEPSTDOUTFILE", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withCommand("echo $DAG_RUN_STEP_STDOUT_FILE"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)
		node := result.nodeByName(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.out$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGRUNSTEPSTDERRFILE", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withCommand("echo $DAG_RUN_STEP_STDERR_FILE"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)
		node := result.nodeByName(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.err$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGRUNID", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withCommand("echo $DAG_RUN_ID"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)
		node := result.nodeByName(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `RESULT=[a-f0-9-]+`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGNAME", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withCommand("echo $DAG_NAME"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)
		node := result.nodeByName(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=test_dag", output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGRUNSTEPNAME", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("step_test", withCommand("echo $DAG_RUN_STEP_NAME"), withOutput("RESULT")),
		)

		result := plan.assertRun(t, core.Succeeded)
		node := result.nodeByName(t, "step_test")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=step_test", output, "unexpected output %q", output)
	})

	t.Run("DAGRunStatusNotAvailableToMainSteps", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1",
				withScript("if [ -z \"$DAG_RUN_STATUS\" ]; then echo unset; else echo set; fi"),
				withOutput("RESULT"),
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		node := result.nodeByName(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=unset", output, "unexpected output %q", output)
	})

	t.Run("RepeatPolicyRepeatsUntilCommandConditionMatchesExpected", func(t *testing.T) {
		r := setupRunner(t)

		// This step will repeat until the file contains 'ready'
		file := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_test_%s.txt", uuid.Must(uuid.NewV7()).String()))
		err := os.Remove(file)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		defer func() {
			err := os.Remove(file)
			if err != nil && !os.IsNotExist(err) {
				require.NoError(t, err)
			}
		}()
		plan := r.newPlan(t,
			newStep("1",
				withCommand(fmt.Sprintf("cat %s || true", file)),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: fmt.Sprintf("`cat %s || true`", file),
						Expected:  "ready",
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		go func() {
			time.Sleep(400 * time.Millisecond)
			err := os.WriteFile(file, []byte("ready"), 0600)
			require.NoError(t, err, "failed to write to file")
		}()

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		// Should have run at least twice (first: not ready, second: ready)
		node := result.nodeByName(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyRepeatWhileConditionExits0", func(t *testing.T) {
		r := setupRunner(t)
		// This step will repeat until the file exists
		file := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_exit0_%s", uuid.Must(uuid.NewV7()).String()))
		err := os.Remove(file)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		defer func() {
			err := os.Remove(file)
			if err != nil && !os.IsNotExist(err) {
				require.NoError(t, err)
			}
		}()
		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo hello"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeWhile
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: "test ! -f " + file,
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)
		// Create file 100 ms after step runs
		go func() {
			time.Sleep(200 * time.Millisecond)
			f, _ := os.Create(file)
			err := f.Close()
			require.NoError(t, err)
		}()
		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		node := result.nodeByName(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyRepeatsWhileCommandExitCodeMatches", func(t *testing.T) {
		r := setupRunner(t)
		// This step will repeat until exit code is not 42.
		countFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_exitcode_%s", uuid.Must(uuid.NewV7()).String()))
		err := os.Remove(countFile)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		defer func() {
			err := os.Remove(countFile)
			if err != nil && !os.IsNotExist(err) {
				require.NoError(t, err)
			}
		}()
		// Script: fail with exit 42 until file exists, then exit 0
		script := fmt.Sprintf(`if [ ! -f %[1]s ]; then exit 42; else exit 0; fi`, countFile)
		plan := r.newPlan(t,
			newStep("1",
				withScript(script),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeWhile
					step.RepeatPolicy.ExitCode = []int{42}
					step.RepeatPolicy.Interval = 50 * time.Millisecond
				},
			),
		)
		go func() {
			time.Sleep(350 * time.Millisecond)
			f, _ := os.Create(countFile)
			err := f.Close()
			require.NoError(t, err)
		}()
		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		node := result.nodeByName(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyRepeatsUntilEnvVarConditionMatchesExpected", func(t *testing.T) {
		r := setupRunner(t)
		// This step will repeat until the environment variable TEST_REPEAT_MATCH_EXPR equals 'done'
		err := os.Setenv("TEST_REPEAT_MATCH_EXPR", "notyet")
		require.NoError(t, err)
		t.Cleanup(func() {
			err := os.Unsetenv("TEST_REPEAT_MATCH_EXPR")
			require.NoError(t, err)
		})
		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo $TEST_REPEAT_MATCH_EXPR"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: "$TEST_REPEAT_MATCH_EXPR",
						Expected:  "done",
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)
		go func() {
			time.Sleep(300 * time.Millisecond)
			err := os.Setenv("TEST_REPEAT_MATCH_EXPR", "done")
			require.NoError(t, err)
		}()
		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		node := result.nodeByName(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyRepeatsUntilOutputVarConditionMatchesExpected", func(t *testing.T) {
		r := setupRunner(t)
		file := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_outputvar_%s", uuid.Must(uuid.NewV7()).String()))
		err := os.Remove(file)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		t.Cleanup(func() { err := os.Remove(file); require.NoError(t, err) })
		// Write initial value
		err = os.WriteFile(file, []byte("notyet"), 0600)
		require.NoError(t, err)
		plan := r.newPlan(t,
			newStep("1",
				withCommand(fmt.Sprintf("cat %s", file)),
				withOutput("OUT"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: "$OUT",
						Expected:  "done",
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)
		go func() {
			time.Sleep(300 * time.Millisecond)
			err := os.WriteFile(file, []byte("done"), 0600)
			require.NoError(t, err)
		}()
		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		node := result.nodeByName(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})
	t.Run("RetryPolicyWithOutputCapture", func(t *testing.T) {
		r := setupRunner(t)

		// Create a counter file for tracking retry attempts
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("retry_output_%s.txt", uuid.Must(uuid.NewV7()).String()))
		defer func() {
			_ = os.Remove(counterFile)
		}()

		// Step that outputs different values on each retry and fails until 3rd attempt
		plan := r.newPlan(t,
			newStep("1",
				withScript(fmt.Sprintf(`
					COUNTER_FILE="%s"
					if [ ! -f "$COUNTER_FILE" ]; then
						echo "1" > "$COUNTER_FILE"
						echo "output_attempt_1"
						exit 1
					else
						COUNT=$(cat "$COUNTER_FILE")
						if [ "$COUNT" -eq "1" ]; then
							echo "2" > "$COUNTER_FILE"
							echo "output_attempt_2"
							exit 1
						elif [ "$COUNT" -eq "2" ]; then
							echo "3" > "$COUNTER_FILE"
							echo "output_attempt_3_success"
							exit 0
						fi
					fi
				`, counterFile)),
				withOutput("RESULT"),
				withRetryPolicy(3, time.Millisecond*20),
			),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		require.Equal(t, 1, node.State().DoneCount)  // 1 successful execution
		require.Equal(t, 2, node.State().RetryCount) // 2 retries

		// Verify that output contains only the final successful attempt's output
		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=output_attempt_3_success", output, "expected final output, got %q", output)
	})
	t.Run("FailedStepWithOutputCapture", func(t *testing.T) {
		r := setupRunner(t)

		// Step that outputs data but fails
		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo 'error_output'; exit 1"),
				withOutput("ERROR_MSG"),
			),
		)

		result := plan.assertRun(t, core.Failed)

		result.assertNodeStatus(t, "1", core.NodeFailed)

		node := result.nodeByName(t, "1")

		// Verify that output is captured even on failure
		output, ok := node.NodeData().State.OutputVariables.Load("ERROR_MSG")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "ERROR_MSG=error_output", output, "expected output %q, got %q", "error_output", output)
	})
	t.Run("RetryPolicySubDAGRunWithOutputCapture", func(t *testing.T) {
		r := setupRunner(t)

		// Create a counter file for tracking retry attempts
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("retry_sub_output_%s.txt", uuid.Must(uuid.NewV7()).String()))
		defer func() {
			_ = os.Remove(counterFile)
		}()

		// Step that outputs different values on each retry and always fails
		plan := r.newPlan(t,
			newStep("1",
				withScript(fmt.Sprintf(`
					COUNTER_FILE="%s"
					if [ ! -f "$COUNTER_FILE" ]; then
						echo "1" > "$COUNTER_FILE"
						echo "output_attempt_1"
						exit 1
					else
						COUNT=$(cat "$COUNTER_FILE")
						if [ "$COUNT" -eq "1" ]; then
							echo "2" > "$COUNTER_FILE"
							echo "output_attempt_2"
							exit 1
						elif [ "$COUNT" -eq "2" ]; then
							echo "3" > "$COUNTER_FILE"
							echo "output_attempt_3"
							exit 1
						fi
					fi
				`, counterFile)),
				withOutput("RESULT"),
				withRetryPolicy(2, time.Millisecond*20),
			),
		)

		result := plan.assertRun(t, core.Failed)

		result.assertNodeStatus(t, "1", core.NodeFailed)

		node := result.nodeByName(t, "1")
		require.Equal(t, 1, node.State().DoneCount)  // 1 execution (failed)
		require.Equal(t, 2, node.State().RetryCount) // 2 retries

		// Verify that output contains the final retry attempt's output
		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=output_attempt_3", output, "expected final output, got %q", output)
	})
}

// Step-level timeout tests
func TestRunner_StepLevelTimeout(t *testing.T) {
	t.Run("SingleStepTimeoutFailsStep", func(t *testing.T) {
		r := setupRunner(t, withTimeout(2*time.Second)) // large DAG timeout to ensure step-level fires first
		plan := r.newPlan(t,
			newStep("timeout_step",
				withCommand("sleep 0.2"), // longer than step timeout
				withStepTimeout(100*time.Millisecond),
			),
			successStep("after", "timeout_step"),
		)

		start := time.Now()
		result := plan.assertRun(t, core.Failed)
		elapsed := time.Since(start)

		// Step should be aborted quickly (< 2s DAG timeout)
		assert.Less(t, elapsed, 1500*time.Millisecond)
		result.assertNodeStatus(t, "timeout_step", core.NodeFailed)
		// Downstream dependency is aborted since runner cancels remaining steps after failure
		result.assertNodeStatus(t, "after", core.NodeAborted)

		node := result.nodeByName(t, "timeout_step")
		// Exit code should be 124 (standard timeout) and error message should mention timeout
		assert.Equal(t, 124, node.State().ExitCode)
		require.NotNil(t, node.State().Error)
		assert.Contains(t, node.State().Error.Error(), "step timed out")
	})

	t.Run("TimeoutPreemptsRetriesAndMarksFailed", func(t *testing.T) {
		r := setupRunner(t)
		plan := r.newPlan(t,
			newStep("retry_timeout",
				withCommand("sleep 0.15 && false"),
				withRetryPolicy(5, 50*time.Millisecond), // would retry many times if not timed out
				withStepTimeout(100*time.Millisecond),   // shorter than sleep
			),
		)

		result := plan.assertRun(t, core.Failed)
		result.assertNodeStatus(t, "retry_timeout", core.NodeFailed)
		node := result.nodeByName(t, "retry_timeout")
		// Should not have retried because first attempt exceeded timeout
		assert.Equal(t, 0, node.State().RetryCount)
		assert.Equal(t, 124, node.State().ExitCode)
	})

	t.Run("ParallelStepsTimeoutFailIndividually", func(t *testing.T) {
		r := setupRunner(t, withMaxActiveRuns(3))
		plan := r.newPlan(t,
			newStep("p1", withCommand("sleep 0.2"), withStepTimeout(80*time.Millisecond)),
			newStep("p2", withCommand("sleep 0.2"), withStepTimeout(80*time.Millisecond)),
			newStep("p3", withCommand("sleep 0.2"), withStepTimeout(80*time.Millisecond)),
		)

		result := plan.assertRun(t, core.Failed)
		result.assertNodeStatus(t, "p1", core.NodeFailed)
		result.assertNodeStatus(t, "p2", core.NodeFailed)
		result.assertNodeStatus(t, "p3", core.NodeFailed)
	})

	t.Run("StepLevelTimeoutOverridesLongDAGTimeoutAndFails", func(t *testing.T) {
		r := setupRunner(t, withTimeout(5*time.Second))
		plan := r.newPlan(t,
			newStep("short_timeout", withCommand("sleep 0.3"), withStepTimeout(120*time.Millisecond)),
		)
		result := plan.assertRun(t, core.Failed)
		result.assertNodeStatus(t, "short_timeout", core.NodeFailed)
		node := result.nodeByName(t, "short_timeout")
		assert.Equal(t, 124, node.State().ExitCode)
	})
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   core.Status
		expected string
	}{
		{core.NotStarted, "not_started"},
		{core.Running, "running"},
		{core.Failed, "failed"},
		{core.Aborted, "aborted"},
		{core.Succeeded, "succeeded"},
		{core.Queued, "queued"},
		{core.Status(999), "unknown"}, // Invalid status defaults to "unknown"
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestStatus_IsActive(t *testing.T) {
	tests := []struct {
		status   core.Status
		expected bool
	}{
		{core.NotStarted, false},
		{core.Running, true},
		{core.Failed, false},
		{core.Aborted, false},
		{core.Succeeded, false},
		{core.Queued, true},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsActive())
		})
	}
}

func TestRunner_DryRun(t *testing.T) {
	r := setupRunner(t, func(cfg *runtime.Config) {
		cfg.Dry = true
	})

	plan := r.newPlan(t,
		successStep("1"),
		successStep("2", "1"),
		successStep("3", "2"),
	)

	result := plan.assertRun(t, core.Succeeded)

	// In dry run, steps should be marked as success without actual execution
	result.assertNodeStatus(t, "1", core.NodeSucceeded)
	result.assertNodeStatus(t, "2", core.NodeSucceeded)
	result.assertNodeStatus(t, "3", core.NodeSucceeded)
}

func TestRunner_DryRunWithHandlers(t *testing.T) {
	r := setupRunner(t,
		func(cfg *runtime.Config) {
			cfg.Dry = true
		},
		withOnExit(successStep("onExit")),
		withOnSuccess(successStep("onSuccess")),
	)

	plan := r.newPlan(t, successStep("1"))

	result := plan.assertRun(t, core.Succeeded)

	result.assertNodeStatus(t, "1", core.NodeSucceeded)
	result.assertNodeStatus(t, "onExit", core.NodeSucceeded)
	result.assertNodeStatus(t, "onSuccess", core.NodeSucceeded)
}

func TestRunner_ConcurrentExecution(t *testing.T) {
	steps := func() []core.Step {
		return []core.Step{
			newStep("1", withScript("sleep 0.3")),
			newStep("2", withScript("sleep 0.3")),
			newStep("3", withScript("sleep 0.3")),
		}
	}

	sequential := setupRunner(t, withMaxActiveRuns(1))
	planSequential := sequential.newPlan(t, steps()...)
	startSequential := time.Now()
	resultSequential := planSequential.assertRun(t, core.Succeeded)
	elapsedSequential := time.Since(startSequential)
	resultSequential.assertNodeStatus(t, "1", core.NodeSucceeded)
	resultSequential.assertNodeStatus(t, "2", core.NodeSucceeded)
	resultSequential.assertNodeStatus(t, "3", core.NodeSucceeded)

	concurrent := setupRunner(t, withMaxActiveRuns(3))
	planConcurrent := concurrent.newPlan(t, steps()...)
	startConcurrent := time.Now()
	resultConcurrent := planConcurrent.assertRun(t, core.Succeeded)
	elapsedConcurrent := time.Since(startConcurrent)
	resultConcurrent.assertNodeStatus(t, "1", core.NodeSucceeded)
	resultConcurrent.assertNodeStatus(t, "2", core.NodeSucceeded)
	resultConcurrent.assertNodeStatus(t, "3", core.NodeSucceeded)

	assert.Greater(t, elapsedSequential, elapsedConcurrent)
	assert.Greater(t, elapsedSequential-elapsedConcurrent, 200*time.Millisecond)
}

func TestRunner_ErrorHandling(t *testing.T) {
	t.Run("SetupError", func(t *testing.T) {
		// Create a runner with invalid log directory
		invalidLogDir := "/nonexistent/path/that/should/not/exist"
		r := setupRunner(t, func(cfg *runtime.Config) {
			cfg.LogDir = invalidLogDir
		})

		plan := r.newPlan(t, successStep("1"))

		// Should fail during setup
		dag := &core.DAG{Name: "test_dag"}
		logFilename := fmt.Sprintf("%s_%s.log", dag.Name, r.cfg.DAGRunID)
		logFilePath := filepath.Join(r.cfg.LogDir, logFilename)

		ctx := execution.NewContext(plan.Context, dag, r.cfg.DAGRunID, logFilePath)

		err := r.runner.Run(ctx, plan.Plan, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create log directory")
	})

	t.Run("PanicRecovery", func(t *testing.T) {
		r := setupRunner(t)

		// Create a step that will panic
		panicStep := newStep("panic", withScript(`
			echo "About to panic"
			# Simulate a panic by killing the process with an invalid signal
			kill -99 $$
		`))

		plan := r.newPlan(t, panicStep)

		// The runner should recover from the panic and mark the step as error
		result := plan.assertRun(t, core.Failed)
		result.assertNodeStatus(t, "panic", core.NodeFailed)
	})
}

func TestRunner_Metrics(t *testing.T) {
	r := setupRunner(t)

	plan := r.newPlan(t,
		successStep("1"),
		failStep("2"),
		newStep("3", withPrecondition(&core.Condition{
			Condition: "false",
		})),
		successStep("4", "1"),
	)

	result := plan.assertRun(t, core.Failed)

	// Get metrics
	metrics := r.runner.GetMetrics()

	assert.Equal(t, 4, metrics["totalNodes"])
	assert.Equal(t, 2, metrics["completedNodes"]) // 1 and 4
	assert.Equal(t, 1, metrics["failedNodes"])    // 2
	assert.Equal(t, 1, metrics["skippedNodes"])   // 3
	assert.Equal(t, 0, metrics["canceledNodes"])
	assert.NotEmpty(t, metrics["totalExecutionTime"])

	// Verify individual node statuses
	result.assertNodeStatus(t, "1", core.NodeSucceeded)
	result.assertNodeStatus(t, "2", core.NodeFailed)
	result.assertNodeStatus(t, "3", core.NodeSkipped)
	result.assertNodeStatus(t, "4", core.NodeSucceeded)
}

func TestRunner_DAGPreconditions(t *testing.T) {
	t.Run("DAGPreconditionNotMet", func(t *testing.T) {
		r := setupRunner(t)

		// Create DAG with precondition that will fail
		dag := &core.DAG{
			Name: "test_dag",
			Preconditions: []*core.Condition{
				{
					Condition: "false", // This will fail
				},
			},
		}

		plan := r.newPlan(t, successStep("1"))

		// Custom schedule with DAG preconditions
		logFilename := fmt.Sprintf("%s_%s.log", dag.Name, r.cfg.DAGRunID)
		logFilePath := filepath.Join(r.cfg.LogDir, logFilename)

		ctx := execution.NewContext(plan.Context, dag, r.cfg.DAGRunID, logFilePath)

		err := r.runner.Run(ctx, plan.Plan, nil)
		require.NoError(t, err) // No error, but dag should be canceled

		// Check that the runner was canceled
		assert.Equal(t, core.Aborted, r.runner.Status(ctx, plan.Plan))
	})
}

func TestRunner_SignalHandling(t *testing.T) {
	t.Run("SignalWithDoneChannel", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withCommand("sleep 0.5")),
			successStep("2", "1"),
		)

		done := make(chan bool, 1)

		go func() {
			time.Sleep(100 * time.Millisecond)
			r.runner.Signal(r.Context, plan.Plan, syscall.SIGTERM, done, false)
		}()

		start := time.Now()
		result := plan.assertRun(t, core.Aborted)

		// Wait for signal completion
		select {
		case <-done:
			// Signal handling completed
		case <-time.After(1 * time.Second):
			t.Fatal("Signal handling did not complete in time")
		}

		elapsed := time.Since(start)
		assert.Less(t, elapsed, 2*time.Second, "Should cancel quickly")

		result.assertNodeStatus(t, "1", core.NodeAborted)
		result.assertNodeStatus(t, "2", core.NodeNotStarted)
	})

	t.Run("SignalWithOverride", func(t *testing.T) {
		r := setupRunner(t)

		plan := r.newPlan(t,
			newStep("1", withCommand("sleep 0.5")),
		)

		go func() {
			time.Sleep(100 * time.Millisecond)
			r.runner.Signal(r.Context, plan.Plan, syscall.SIGKILL, nil, true)
		}()

		result := plan.assertRun(t, core.Aborted)
		result.assertNodeStatus(t, "1", core.NodeAborted)
	})
}

func TestRunner_ComplexDependencyChains(t *testing.T) {
	t.Run("DiamondDependency", func(t *testing.T) {
		r := setupRunner(t)

		// Create diamond dependency: 1 -> 2,3 -> 4
		plan := r.newPlan(t,
			successStep("1"),
			successStep("2", "1"),
			successStep("3", "1"),
			successStep("4", "2", "3"),
		)

		result := plan.assertRun(t, core.Succeeded)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeSucceeded)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
		result.assertNodeStatus(t, "4", core.NodeSucceeded)
	})

	t.Run("ComplexFailurePropagation", func(t *testing.T) {
		r := setupRunner(t)

		// 1 -> 2 (fail) -> 4
		//   -> 3 -------->
		plan := r.newPlan(t,
			successStep("1"),
			failStep("2", "1"),
			successStep("3", "1"),
			successStep("4", "2", "3"),
		)

		result := plan.assertRun(t, core.Failed)

		result.assertNodeStatus(t, "1", core.NodeSucceeded)
		result.assertNodeStatus(t, "2", core.NodeFailed)
		result.assertNodeStatus(t, "3", core.NodeSucceeded)
		result.assertNodeStatus(t, "4", core.NodeAborted) // Canceled due to 2's failure
	})
}

func TestRunner_EdgeCases(t *testing.T) {
	t.Run("EmptyPlan", func(t *testing.T) {
		r := setupRunner(t)
		plan := r.newPlan(t) // No steps

		result := plan.assertRun(t, core.Succeeded)
		assert.NoError(t, result.Error)
	})

	t.Run("SingleNodePlan", func(t *testing.T) {
		r := setupRunner(t)
		plan := r.newPlan(t, successStep("single"))

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "single", core.NodeSucceeded)
	})

	t.Run("AllNodesFail", func(t *testing.T) {
		r := setupRunner(t)
		plan := r.newPlan(t,
			failStep("1"),
			failStep("2"),
			failStep("3"),
		)

		result := plan.assertRun(t, core.Failed)
		result.assertNodeStatus(t, "1", core.NodeFailed)
		result.assertNodeStatus(t, "2", core.NodeFailed)
		result.assertNodeStatus(t, "3", core.NodeFailed)
	})
}

func TestRunner_HandlerNodeAccess(t *testing.T) {
	exitStep := successStep("onExit")
	successHandlerStep := successStep("onSuccess")
	failureStep := successStep("onFailure")
	cancelStep := successStep("onCancel")

	r := setupRunner(t,
		withOnExit(exitStep),
		withOnSuccess(successHandlerStep),
		withOnFailure(failureStep),
		withOnCancel(cancelStep),
	)

	// Run a simple plan to trigger setup
	plan := r.newPlan(t, successStep("1"))
	_ = plan.assertRun(t, core.Succeeded)

	// Access handler nodes
	assert.NotNil(t, r.runner.HandlerNode(core.HandlerOnExit))
	assert.NotNil(t, r.runner.HandlerNode(core.HandlerOnSuccess))
	assert.NotNil(t, r.runner.HandlerNode(core.HandlerOnFailure))
	assert.NotNil(t, r.runner.HandlerNode(core.HandlerOnCancel))
	assert.Nil(t, r.runner.HandlerNode(core.HandlerType("unknown")))
}

func TestRunner_PreconditionWithError(t *testing.T) {
	r := setupRunner(t)

	// Create a step with a precondition that will error (not just return false)
	plan := r.newPlan(t,
		newStep("1",
			withPrecondition(&core.Condition{
				Condition: "exit 2", // Exit with non-zero code
			}),
			withCommand("echo should_not_run"),
		),
	)

	result := plan.assertRun(t, core.Succeeded)

	// The step should be skipped but no error should be set for condition not met
	result.assertNodeStatus(t, "1", core.NodeSkipped)
	// Conditions that exit with non-zero are just "not met", not errors
}

func TestRunner_MultipleHandlerExecution(t *testing.T) {
	recordHandler := func(name string) core.Step {
		return newStep(name, withScript(fmt.Sprintf(`echo "Handler %s executed"`, name)))
	}

	r := setupRunner(t,
		withOnExit(recordHandler("onExit")),
		withOnFailure(recordHandler("onFailure")),
	)

	plan := r.newPlan(t, failStep("1"))

	result := plan.assertRun(t, core.Failed)

	// Both onFailure and onExit should execute
	result.assertNodeStatus(t, "1", core.NodeFailed)
	result.assertNodeStatus(t, "onFailure", core.NodeSucceeded)
	result.assertNodeStatus(t, "onExit", core.NodeSucceeded)
}

func TestRunner_TimeoutDuringRetry(t *testing.T) {
	r := setupRunner(t, withTimeout(500*time.Millisecond))

	// Step that will keep retrying until timeout
	plan := r.newPlan(t,
		newStep("1",
			withCommand("sleep 0.1 && false"),
			withRetryPolicy(10, 50*time.Millisecond), // Many retries
		),
	)

	start := time.Now()
	result := plan.assertRun(t, core.Failed)
	elapsed := time.Since(start)

	// Should timeout before completing all retries
	assert.Less(t, elapsed, 5*time.Second)
	result.assertNodeStatus(t, "1", core.NodeAborted)
}

func TestRunner_CancelDuringHandlerExecution(t *testing.T) {
	r := setupRunner(t,
		withOnExit(newStep("onExit", withScript("echo handler started && sleep 0.1 && echo handler done"))),
	)

	plan := r.newPlan(t, successStep("1"))

	go func() {
		// Wait for main step to complete and handler to start
		time.Sleep(200 * time.Millisecond)
		r.runner.Cancel(plan.Plan)
	}()

	// Since we cancel during handler execution, the final status depends on timing
	// The plan completes successfully before cancel takes effect
	result := plan.assertRun(t, core.Succeeded)

	result.assertNodeStatus(t, "1", core.NodeSucceeded)
	// Handler should complete successfully
	result.assertNodeStatus(t, "onExit", core.NodeSucceeded)
}

func TestRunner_RepeatPolicyWithCancel(t *testing.T) {
	r := setupRunner(t)

	plan := r.newPlan(t,
		newStep("1",
			withCommand("echo repeat"),
			withRepeatPolicy(true, 100*time.Millisecond),
		),
	)

	go func() {
		time.Sleep(350 * time.Millisecond)
		r.runner.Cancel(plan.Plan)
	}()

	result := plan.assertRun(t, core.Aborted)
	result.assertNodeStatus(t, "1", core.NodeAborted)

	node := result.nodeByName(t, "1")
	// Should have repeated at least twice before cancel
	assert.GreaterOrEqual(t, node.State().DoneCount, 2)
}

func TestRunner_RepeatPolicyWithLimit(t *testing.T) {
	r := setupRunner(t)

	// Test repeat with limit
	plan := r.newPlan(t,
		newStep("1",
			withCommand("echo repeat"),
			withRepeatPolicy(true, 100*time.Millisecond),
			func(step *core.Step) {
				step.RepeatPolicy.Limit = 3
			},
		),
	)

	result := plan.assertRun(t, core.Succeeded)
	result.assertNodeStatus(t, "1", core.NodeSucceeded)

	node := result.nodeByName(t, "1")
	// Should have executed exactly 3 times (initial + 2 repeats)
	assert.Equal(t, 3, node.State().DoneCount)
}

func TestRunner_RepeatPolicyWithLimitAndCondition(t *testing.T) {
	r := setupRunner(t)

	counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_limit_%s", uuid.Must(uuid.NewV7()).String()))
	defer func() { _ = os.Remove(counterFile) }()

	// Test repeat with limit and condition
	plan := r.newPlan(t,
		newStep("1",
			withScript(fmt.Sprintf(`
				COUNT=0
				if [ -f "%s" ]; then
					COUNT=$(cat "%s")
				fi
				COUNT=$((COUNT + 1))
				echo "$COUNT" > "%s"
				echo "PENDING"
			`, counterFile, counterFile, counterFile)),
			func(step *core.Step) {
				step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
				step.RepeatPolicy.Limit = 5
				step.RepeatPolicy.Condition = &core.Condition{
					Condition: "`cat " + counterFile + "`",
					Expected:  "10", // Would repeat forever but limit stops at 5
				}
			},
		),
	)

	result := plan.assertRun(t, core.Succeeded)
	result.assertNodeStatus(t, "1", core.NodeSucceeded)

	node := result.nodeByName(t, "1")
	// Should have executed exactly 5 times due to limit
	assert.Equal(t, 5, node.State().DoneCount)

	// Verify counter file shows 5
	content, err := os.ReadFile(counterFile)
	assert.NoError(t, err)
	assert.Equal(t, "5\n", string(content))
}

func TestRunner_ComplexRetryScenarios(t *testing.T) {
	t.Run("RetryWithSignalTermination", func(t *testing.T) {
		r := setupRunner(t)

		// Create a script that will be terminated by signal
		plan := r.newPlan(t,
			newStep("1",
				withScript(`
					trap 'exit 143' TERM
					sleep 10
				`),
				withRetryPolicy(2, 20*time.Millisecond),
			),
		)

		go func() {
			time.Sleep(200 * time.Millisecond)
			plan.signal(syscall.SIGTERM)
		}()

		result := plan.assertRun(t, core.Aborted)
		result.assertNodeStatus(t, "1", core.NodeAborted)
	})

	t.Run("RetryWithSpecificExitCodes", func(t *testing.T) {
		r := setupRunner(t)

		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("retry_codes_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		// Step that returns different exit codes
		plan := r.newPlan(t,
			newStep("1",
				withScript(fmt.Sprintf(`
					if [ ! -f "%s" ]; then
						echo "1" > "%s"
						exit 42  # Should retry
					else
						COUNT=$(cat "%s")
						if [ "$COUNT" -eq "1" ]; then
							echo "2" > "%s"
							exit 100  # Should not retry
						fi
					fi
				`, counterFile, counterFile, counterFile, counterFile)),
				withRetryPolicy(3, 20*time.Millisecond),
				func(step *core.Step) {
					step.RetryPolicy.ExitCodes = []int{42} // Only retry on exit code 42
				},
			),
		)

		result := plan.assertRun(t, core.Failed)
		result.assertNodeStatus(t, "1", core.NodeFailed)

		node := result.nodeByName(t, "1")
		// Should retry once (first failure with code 42, then fail with code 100)
		assert.Equal(t, 1, node.State().RetryCount)
	})

	// Test cases for behaviors when neither condition nor exitCode are present
	t.Run("RepeatPolicyBooleanTrueRepeatsWhileStepSucceeds", func(t *testing.T) {
		r := setupRunner(t)

		// Test repeat: true (boolean mode) - should repeat while step succeeds (no condition/exitCode)
		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo boolean true mode"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeWhile // This is what repeat: true becomes
					step.RepeatPolicy.Interval = 20 * time.Millisecond
					step.RepeatPolicy.Limit = 3 // Limit to prevent infinite loop
					// No condition, no exitCode - should repeat while step succeeds
				},
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have executed exactly 3 times (limit reached, step always succeeds)
		assert.Equal(t, 3, node.State().DoneCount)
	})

	t.Run("RepeatPolicyBooleanTrueWithFailureStopsOnFailure", func(t *testing.T) {
		r := setupRunner(t)

		// Test repeat: true (boolean mode) with step that eventually fails
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_bool_fail_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		plan := r.newPlan(t,
			newStep("1",
				withScript(fmt.Sprintf(`
					COUNT=0
					if [ -f "%s" ]; then
						COUNT=$(cat "%s")
					fi
					COUNT=$((COUNT + 1))
					echo "$COUNT" > "%s"
					if [ "$COUNT" -le 2 ]; then
						exit 0
					else
						exit 1
					fi
				`, counterFile, counterFile, counterFile)),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeWhile // Boolean true mode
					step.RepeatPolicy.Interval = 20 * time.Millisecond
					// No condition, no exitCode - should stop when step fails
				},
			),
		)

		result := plan.assertRun(t, core.Failed)
		result.assertNodeStatus(t, "1", core.NodeFailed)

		node := result.nodeByName(t, "1")
		// Should have executed exactly 3 times (2 successes, then 1 failure stops it)
		assert.Equal(t, 3, node.State().DoneCount)
	})

	t.Run("RepeatPolicyUntilModeWithoutConditionRepeatsOnFailure", func(t *testing.T) {
		r := setupRunner(t)

		// Test explicit until mode without condition/exitCode (repeats until step succeeds)
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_until_none_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		plan := r.newPlan(t,
			newStep("1",
				withScript(fmt.Sprintf(`
					COUNT=0
					if [ -f "%s" ]; then
						COUNT=$(cat "%s")
					fi
					COUNT=$((COUNT + 1))
					echo "$COUNT" > "%s"
					if [ "$COUNT" -le 2 ]; then
						exit 1
					else
						exit 0
					fi
				`, counterFile, counterFile, counterFile)),
				withContinueOn(core.ContinueOn{
					ExitCode:    []int{1},
					Failure:     true,
					MarkSuccess: true,
				}),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.Interval = 20 * time.Millisecond
					// No condition, no exitCode - should repeat until step succeeds
				},
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have executed exactly 3 times (fails twice, then succeeds)
		assert.Equal(t, 3, node.State().DoneCount)
	})

	t.Run("RepeatPolicyWhileWithConditionRepeatsWhileConditionSucceeds", func(t *testing.T) {
		r := setupRunner(t)

		// Test explicit while mode with condition
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_while_cond_%s", uuid.Must(uuid.NewV7()).String()))
		err := os.Remove(counterFile)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		defer func() {
			err := os.Remove(counterFile)
			if err != nil && !os.IsNotExist(err) {
				require.NoError(t, err)
			}
		}()
		plan := r.newPlan(t,
			newStep("1",
				withCommand("cat "+counterFile+" || echo notfound"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeWhile
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: fmt.Sprintf("test ! -f %s", counterFile),
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		go func() {
			time.Sleep(time.Millisecond * 50)
			f, _ := os.Create(counterFile)
			_ = f.Close()
		}()

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have run at least twice (first: file not found, second: file created)
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyWhileWithConditionAndExpectedRepeatsWhileMatches", func(t *testing.T) {
		r := setupRunner(t)

		// Test explicit while mode with condition and expected value
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_while_exp_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		// Write initial value
		err := os.WriteFile(counterFile, []byte("continue"), 0600)
		require.NoError(t, err)

		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo while with expected"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeWhile
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: fmt.Sprintf("`cat %s`", counterFile),
						Expected:  "continue",
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		go func() {
			time.Sleep(time.Millisecond * 50)
			err := os.WriteFile(counterFile, []byte("stop"), 0600)
			require.NoError(t, err)
		}()

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have executed at least 2 times (while expected matches)
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyUntilWithConditionRepeatsUntilConditionSucceeds", func(t *testing.T) {
		r := setupRunner(t)

		// Test explicit until mode with condition (no expected)
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_until_cond_%s", uuid.Must(uuid.NewV7()).String()))
		err := os.Remove(counterFile)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		defer func() {
			err := os.Remove(counterFile)
			if err != nil && !os.IsNotExist(err) {
				require.NoError(t, err)
			}
		}()
		plan := r.newPlan(t,
			newStep("1",
				withCommand("cat "+counterFile+" || echo notfound"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: fmt.Sprintf("test -f %s", counterFile),
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		go func() {
			time.Sleep(time.Millisecond * 100)
			f, _ := os.Create(counterFile)
			_ = f.Close()
		}()

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have run at least twice (first: file not found, second: file created)
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyUntilWithConditionAndExpectedRepeatsUntilMatches", func(t *testing.T) {
		r := setupRunner(t)

		// Test explicit until mode with condition and expected value
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_until_exp_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		// Write initial value
		err := os.WriteFile(counterFile, []byte("waiting"), 0600)
		require.NoError(t, err)

		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo until with expected"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: fmt.Sprintf("`cat %s`", counterFile),
						Expected:  "ready",
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		go func() {
			time.Sleep(time.Millisecond * 50)
			err := os.WriteFile(counterFile, []byte("ready"), 0600)
			require.NoError(t, err)
		}()

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have executed at least 2 times (until expected matches)
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyUntilWithExitCodeRepeatsUntilExitCodeMatches", func(t *testing.T) {
		r := setupRunner(t)

		// Test explicit until mode with exit codes
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_until_exit_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		plan := r.newPlan(t,
			newStep("1",
				withScript(fmt.Sprintf(`
					COUNT=0
					if [ -f "%s" ]; then
						COUNT=$(cat "%s")
					fi
					COUNT=$((COUNT + 1))
					echo "$COUNT" > "%s"
					if [ "$COUNT" -le 2 ]; then
						exit 1
					else
						exit 42
					fi
				`, counterFile, counterFile, counterFile)),
				withContinueOn(core.ContinueOn{
					ExitCode:    []int{1},
					Failure:     true,
					MarkSuccess: true,
				}),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.ExitCode = []int{42}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		go func() {
			time.Sleep(time.Millisecond * 100)
			f, _ := os.Create(counterFile)
			_ = f.Close()
		}()

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have executed at least 3 times (until exit code 42)
		assert.GreaterOrEqual(t, node.State().DoneCount, 3)
	})

	t.Run("RepeatPolicyLimit", func(t *testing.T) {
		r := setupRunner(t)
		plan := r.newPlan(t,
			newStep("1",
				withCommand("echo limit"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: "false", // Will never be true
					}
					step.RepeatPolicy.Limit = 3
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have executed exactly 3 times (limit reached)
		assert.Equal(t, 3, node.State().DoneCount)
	})

	t.Run("RepeatPolicyOutputVariablesReloadedBeforeConditionEval", func(t *testing.T) {
		r := setupRunner(t)

		// Test that output variables are reloaded before evaluating repeat condition
		// Use a file-based counter to track iterations properly
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_output_var_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		plan := r.newPlan(t,
			newStep("1",
				withScript(fmt.Sprintf(`
					# Read counter from file or start at 0
					if [ -f "%s" ]; then
						COUNT=$(cat "%s")
					else
						COUNT=0
					fi
					COUNT=$((COUNT + 1))
					echo "$COUNT" > "%s"
					echo "$COUNT"
				`, counterFile, counterFile, counterFile)),
				withOutput("COUNTER"),
				func(step *core.Step) {
					step.RepeatPolicy.RepeatMode = core.RepeatModeUntil
					step.RepeatPolicy.Condition = &core.Condition{
						Condition: "$COUNTER",
						Expected:  "3",
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "1", core.NodeSucceeded)

		node := result.nodeByName(t, "1")
		// Should have executed exactly 3 times (until COUNTER equals 3)
		assert.Equal(t, 3, node.State().DoneCount)

		// Verify final output variable value
		output, ok := node.NodeData().State.OutputVariables.Load("COUNTER")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "COUNTER=3", output)
	})
}

func TestRunner_StepIDVariableExpansion(t *testing.T) {
	r := setupRunner(t)

	// Test step ID usage in environment setup
	plan := r.newPlan(t,
		newStep("step1",
			withCommand("echo output1"),
			withOutput("OUT1"),
			func(step *core.Step) {
				step.ID = "s1"
			},
		),
		newStep("step2",
			withCommand("echo output2"),
			withOutput("OUT2"),
			func(step *core.Step) {
				step.ID = "s2"
			},
			withDepends("step1"),
		),
		newStep("step3",
			// This should have access to both step1 and step2 outputs via IDs
			withCommand("echo $OUT1 $OUT2"),
			withOutput("COMBINED"),
			withDepends("step2"),
		),
	)

	result := plan.assertRun(t, core.Succeeded)

	result.assertNodeStatus(t, "step1", core.NodeSucceeded)
	result.assertNodeStatus(t, "step2", core.NodeSucceeded)
	result.assertNodeStatus(t, "step3", core.NodeSucceeded)

	node := result.nodeByName(t, "step3")
	output, ok := node.NodeData().State.OutputVariables.Load("COMBINED")
	require.True(t, ok)
	assert.Equal(t, "COMBINED=output1 output2", output)
}

func TestRunner_UnexpectedFinalStatus(t *testing.T) {
	// This is a bit tricky to test as it requires the runner to be in an
	// unexpected state at the end. We'll simulate this by creating a custom
	// scenario that might trigger this edge case.
	r := setupRunner(t)

	// Create a plan with a step that might leave the runner in an unexpected state
	plan := r.newPlan(t,
		newStep("1", withCommand("echo test")),
	)

	// Schedule normally
	result := plan.assertRun(t, core.Succeeded)
	result.assertNodeStatus(t, "1", core.NodeSucceeded)

	// The warning log about unexpected final status would be logged internally
	// but we can't easily test for it without mock logging
}

func TestRunner_RetryPolicyDefaults(t *testing.T) {
	r := setupRunner(t)

	// Test retry with unhandled error type (not exec.ExitError)
	plan := r.newPlan(t,
		newStep("1",
			withScript(`
				# This will cause a different type of error
				echo "Test error" >&2
				exit 1
			`),
			withRetryPolicy(1, 20*time.Millisecond),
		),
	)

	result := plan.assertRun(t, core.Failed)
	result.assertNodeStatus(t, "1", core.NodeFailed)

	node := result.nodeByName(t, "1")
	// Should have retried once
	assert.Equal(t, 1, node.State().RetryCount)
}

func TestRunner_StepRetryExecution(t *testing.T) {
	t.Run("RetrySuccessfulStep", func(t *testing.T) {
		r := setupRunner(t)

		// A -> B -> C, all successful
		dag := &core.DAG{
			Steps: []core.Step{
				{Name: "A", Command: "echo A"},
				{Name: "B", Command: "echo B", Depends: []string{"A"}},
				{Name: "C", Command: "echo C", Depends: []string{"B"}},
			},
		}

		// Initial run - all successful
		plan := r.newPlan(t,
			successStep("A"),
			successStep("B", "A"),
			successStep("C", "B"),
		)
		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "A", core.NodeSucceeded)
		result.assertNodeStatus(t, "B", core.NodeSucceeded)
		result.assertNodeStatus(t, "C", core.NodeSucceeded)

		// Create nodes with their current states
		nodes := []*runtime.Node{
			runtime.NodeWithData(runtime.NodeData{
				Step:  dag.Steps[0],
				State: runtime.NodeState{Status: core.NodeSucceeded},
			}),
			runtime.NodeWithData(runtime.NodeData{
				Step:  dag.Steps[1],
				State: runtime.NodeState{Status: core.NodeSucceeded},
			}),
			runtime.NodeWithData(runtime.NodeData{
				Step:  dag.Steps[2],
				State: runtime.NodeState{Status: core.NodeSucceeded},
			}),
		}

		// Retry step B
		retryPlan, err := runtime.CreateStepRetryPlan(dag, nodes, "B")
		require.NoError(t, err)

		// Schedule the retry
		retryResult := planHelper{testHelper: r, Plan: retryPlan}.assertRun(t, core.Succeeded)

		// A and C should remain unchanged, only B should be re-executed
		retryResult.assertNodeStatus(t, "A", core.NodeSucceeded)
		retryResult.assertNodeStatus(t, "B", core.NodeSucceeded)
		retryResult.assertNodeStatus(t, "C", core.NodeSucceeded)
	})
}

// TestRunner_StepIDAccess tests that step ID variables are expanded correctly
func TestRunner_StepIDAccess(t *testing.T) {
	t.Run("StepReferenceInCommand", func(t *testing.T) {
		r := setupRunner(t)

		// Create a DAG where step2 references step1's output
		plan := r.newPlan(t,
			newStep("step1",
				withID("first"),
				withCommand("echo 'output from step1'"),
				withOutput("STEP1_RESULT=success"),
			),
			newStep("step2",
				withID("second"),
				withDepends("step1"),
				withCommand("echo 'Step 1 stdout: ${first.stdout}'"),
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "step1", core.NodeSucceeded)
		result.assertNodeStatus(t, "step2", core.NodeSucceeded)

		// Step2 should have access to step1's stdout path
		node2 := result.nodeByName(t, "step2")
		stdoutFile := node2.GetStdout()
		stdoutContent, err := os.ReadFile(stdoutFile)
		require.NoError(t, err)
		assert.Contains(t, string(stdoutContent), "Step 1 stdout:")
	})
	t.Run("StepWithoutID", func(t *testing.T) {
		r := setupRunner(t)

		// Create a DAG where some steps don't have IDs
		plan := r.newPlan(t,
			newStep("step1",
				// No ID
				withCommand("echo 'no id'"),
			),
			newStep("step2",
				withID("with_id"),
				withCommand("echo 'has id'"),
				withOutput("VAR=value"),
			),
			newStep("step3",
				withID("third"),
				withDepends("step1", "step2"),
				// Can reference step2's stdout file path
				withCommand("echo 'Can reference: ${with_id.stdout}'"),
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "step1", core.NodeSucceeded)
		result.assertNodeStatus(t, "step2", core.NodeSucceeded)
		result.assertNodeStatus(t, "step3", core.NodeSucceeded)

		node3 := result.nodeByName(t, "step3")
		stdoutFile := node3.GetStdout()
		stdoutContent, err := os.ReadFile(stdoutFile)
		require.NoError(t, err)
		// Should contain the path to step2's stdout file
		assert.Contains(t, string(stdoutContent), "Can reference:")
		assert.Contains(t, string(stdoutContent), ".out")
	})

	t.Run("StepExitCodeReference", func(t *testing.T) {
		r := setupRunner(t)

		// Create a step that checks another step's exit code
		plan := r.newPlan(t,
			newStep("check",
				withID("checker"),
				withCommand("exit 42"),
				withContinueOn(core.ContinueOn{
					ExitCode: []int{42},
				}),
			),
			newStep("verify",
				withID("verifier"),
				withDepends("check"),
				withCommand("echo 'Checker exit code: ${checker.exit_code}'"),
			),
		)

		result := plan.assertRun(t, core.PartiallySucceeded)
		result.assertNodeStatus(t, "check", core.NodeFailed)
		result.assertNodeStatus(t, "verify", core.NodeSucceeded)

		nodeVerify := result.nodeByName(t, "verify")
		stdoutFile := nodeVerify.GetStdout()
		stdoutContent, err := os.ReadFile(stdoutFile)
		require.NoError(t, err)
		assert.Contains(t, string(stdoutContent), "Checker exit code: 42")
	})
}

// TestRunner_EventHandlerStepIDAccess tests that step ID references work in event handlers
func TestRunner_EventHandlerStepIDAccess(t *testing.T) {
	t.Run("OnSuccessHandlerWithStepReferences", func(t *testing.T) {
		r := setupRunner(t,
			withOnSuccess(core.Step{
				Name:    "success_handler",
				ID:      "on_success",
				Command: "echo 'Main output: ${main.stdout}, Worker result: ${worker.exit_code}'",
			}),
		)

		plan := r.newPlan(t,
			newStep("main_step",
				withID("main"),
				withCommand("echo 'Main processing done'"),
			),
			newStep("worker_step",
				withID("worker"),
				withCommand("echo 'Worker processing done' && exit 0"),
				withDepends("main_step"),
			),
		)

		result := plan.assertRun(t, core.Succeeded)

		// All steps should succeed
		result.assertNodeStatus(t, "main_step", core.NodeSucceeded)
		result.assertNodeStatus(t, "worker_step", core.NodeSucceeded)

		// The handler should have executed
		result.assertNodeStatus(t, "success_handler", core.NodeSucceeded)

		// Get the handler node
		handlerNode := result.nodeByName(t, "success_handler")
		require.NotNil(t, handlerNode, "Success handler should have executed")

		// Check handler output contains references to main steps
		handlerOutput, err := os.ReadFile(handlerNode.GetStdout())
		require.NoError(t, err)
		output := string(handlerOutput)
		assert.Contains(t, output, "Main output:")
		assert.Contains(t, output, ".out") // Should contain stdout file path
		assert.Contains(t, output, "Worker result: 0")
	})

	t.Run("OnFailureHandlerWithStepReferences", func(t *testing.T) {
		r := setupRunner(t,
			withOnFailure(core.Step{
				Name:    "failure_handler",
				ID:      "on_fail",
				Command: "echo 'Failed step stderr: ${failing.stderr}, exit code: ${failing.exit_code}'",
			}),
		)

		plan := r.newPlan(t,
			newStep("setup",
				withID("setup_step"),
				withCommand("echo 'Setup complete'"),
			),
			newStep("failing_step",
				withID("failing"),
				withCommand("echo 'Error occurred' >&2 && exit 1"),
				withDepends("setup"),
			),
		)

		result := plan.assertRun(t, core.Failed)

		// Check step statuses
		result.assertNodeStatus(t, "setup", core.NodeSucceeded)
		result.assertNodeStatus(t, "failing_step", core.NodeFailed)

		// The failure handler should have executed
		result.assertNodeStatus(t, "failure_handler", core.NodeSucceeded)

		// Get the handler node
		handlerNode := result.nodeByName(t, "failure_handler")
		require.NotNil(t, handlerNode, "Failure handler should have executed")

		// Check handler output
		handlerOutput, err := os.ReadFile(handlerNode.GetStdout())
		require.NoError(t, err)
		output := string(handlerOutput)
		assert.Contains(t, output, "Failed step stderr:")
		assert.Contains(t, output, ".err") // Should contain stderr file path
		assert.Contains(t, output, "exit code: 1")
	})

	t.Run("OnExitHandlerWithMultipleStepReferences", func(t *testing.T) {
		r := setupRunner(t,
			withOnExit(core.Step{
				Name:    "exit_handler",
				ID:      "on_exit",
				Command: "echo 'Step1: ${step1.stdout}, Step2: ${step2.exit_code}, Step3: ${step3.stderr}'",
			}),
		)

		plan := r.newPlan(t,
			newStep("first",
				withID("step1"),
				withCommand("echo 'First step output'"),
			),
			newStep("second",
				withID("step2"),
				withCommand("exit 0"),
				withDepends("first"),
			),
			newStep("third",
				withID("step3"),
				withCommand("echo 'Warning message' >&2"),
				withDepends("second"),
			),
		)

		result := plan.assertRun(t, core.Succeeded)

		// All main steps should succeed
		result.assertNodeStatus(t, "first", core.NodeSucceeded)
		result.assertNodeStatus(t, "second", core.NodeSucceeded)
		result.assertNodeStatus(t, "third", core.NodeSucceeded)

		// The exit handler should have executed
		result.assertNodeStatus(t, "exit_handler", core.NodeSucceeded)

		// Get the handler node
		handlerNode := result.nodeByName(t, "exit_handler")
		require.NotNil(t, handlerNode, "Exit handler should have executed")

		// Check handler output contains all step references
		handlerOutput, err := os.ReadFile(handlerNode.GetStdout())
		require.NoError(t, err)
		output := string(handlerOutput)
		assert.Contains(t, output, "Step1:")
		assert.Contains(t, output, ".out") // step1 stdout path
		assert.Contains(t, output, "Step2: 0")
		assert.Contains(t, output, "Step3:")
		assert.Contains(t, output, ".err") // step3 stderr path
	})

	t.Run("HandlerWithoutIDCannotBeReferenced", func(t *testing.T) {
		r := setupRunner(t,
			withOnExit(core.Step{
				Name: "exit_handler_no_id",
				// No ID field set
				Command: "echo 'Handler executed'",
			}),
		)

		plan := r.newPlan(t,
			newStep("main",
				withID("main_step"),
				withCommand("echo 'Main step'"),
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "main", core.NodeSucceeded)

		// Handler should execute
		result.assertNodeStatus(t, "exit_handler_no_id", core.NodeSucceeded)

		// Get the handler node to verify it has no ID
		handlerNode := result.nodeByName(t, "exit_handler_no_id")
		assert.Empty(t, handlerNode.Step().ID, "Handler should have no ID")
	})

	t.Run("HandlersCanOnlyReferenceMainSteps", func(t *testing.T) {
		// Test that handlers can reference main steps but not other handlers
		// This is because handlers execute after all main steps are complete
		r := setupRunner(t,
			withOnSuccess(core.Step{
				Name:    "first_handler",
				ID:      "handler1",
				Command: "echo 'SUCCESS: Main returned ${main.exit_code}'",
			}),
			withOnExit(core.Step{
				Name:    "final_handler",
				ID:      "handler2",
				Command: "echo 'FINAL: Main step output at ${main.stdout}, trying handler ref: ${handler1.stdout}'",
			}),
		)

		plan := r.newPlan(t,
			newStep("main",
				withID("main"),
				withCommand("echo 'Processing' && exit 0"),
			),
		)

		result := plan.assertRun(t, core.Succeeded)
		result.assertNodeStatus(t, "main", core.NodeSucceeded)

		// Both handlers should have executed
		result.assertNodeStatus(t, "first_handler", core.NodeSucceeded)
		result.assertNodeStatus(t, "final_handler", core.NodeSucceeded)

		// Get the handler nodes
		successHandler := result.nodeByName(t, "first_handler")
		exitHandler := result.nodeByName(t, "final_handler")

		// Check success handler output
		successOutput, err := os.ReadFile(successHandler.GetStdout())
		require.NoError(t, err)
		assert.Contains(t, string(successOutput), "SUCCESS: Main returned 0")

		// Check exit handler output - it can reference main steps but not other handlers
		exitOutput, err := os.ReadFile(exitHandler.GetStdout())
		require.NoError(t, err)
		output := string(exitOutput)
		assert.Contains(t, output, "FINAL: Main step output at")
		assert.Contains(t, output, ".out") // Should contain main's stdout path
		// Handler reference should not be resolved
		assert.Contains(t, output, "${handler1.stdout}") // Should remain unresolved
	})
}

func TestRunner_DAGRunStatusHandlerEnv(t *testing.T) {
	r := setupRunner(t,
		withOnExit(core.Step{
			Name:    "exit_handler",
			Command: "echo status=${DAG_RUN_STATUS}",
		}),
	)

	plan := r.newPlan(t, successStep("main"))
	result := plan.assertRun(t, core.Succeeded)

	handlerNode := result.nodeByName(t, "exit_handler")
	handlerOutput, err := os.ReadFile(handlerNode.GetStdout())
	require.NoError(t, err)

	assert.Equal(t, "status=succeeded", strings.TrimSpace(string(handlerOutput)))
}

func TestRunnerPartialSuccess(t *testing.T) {
	t.Run("NodeStatusPartialSuccess", func(t *testing.T) {
		r := setupRunner(t)

		// Create a plan where:
		// - step1 succeeds
		// - step2 fails but has continueOn.failure = true
		// - step3 depends on step2 and succeeds
		plan := r.newPlan(t,
			successStep("step1"),
			newStep("step2",
				withDepends("step1"),
				withCommand("false"), // This will fail
				withContinueOn(core.ContinueOn{
					Failure: true,
				}),
			),
			successStep("step3", "step2"),
		)

		// The overall DAG should complete with partial success
		result := plan.assertRun(t, core.PartiallySucceeded)

		// Verify individual node statuses
		result.assertNodeStatus(t, "step1", core.NodeSucceeded)
		result.assertNodeStatus(t, "step2", core.NodeFailed)
		result.assertNodeStatus(t, "step3", core.NodeSucceeded)
	})

	t.Run("NodeStatusPartialSuccessWithMarkSuccess", func(t *testing.T) {
		r := setupRunner(t)

		// Create a plan where:
		// - step1 succeeds
		// - step2 fails but has continueOn.failure = true and markSuccess = true
		// - step3 depends on step2 and succeeds
		plan := r.newPlan(t,
			successStep("step1"),
			newStep("step2",
				withDepends("step1"),
				withCommand("false"), // This will fail
				withContinueOn(core.ContinueOn{
					Failure:     true,
					MarkSuccess: true,
				}),
			),
			successStep("step3", "step2"),
		)

		// When markSuccess is true, the overall DAG should complete with success
		result := plan.assertRun(t, core.Succeeded)

		// Verify individual node statuses
		result.assertNodeStatus(t, "step1", core.NodeSucceeded)
		result.assertNodeStatus(t, "step2", core.NodeSucceeded) // Marked as success
		result.assertNodeStatus(t, "step3", core.NodeSucceeded)
	})

	t.Run("MultipleFailuresWithContinueOn", func(t *testing.T) {
		r := setupRunner(t)

		// Create a plan where multiple steps fail but have continueOn
		plan := r.newPlan(t,
			newStep("step1",
				withCommand("false"),
				withContinueOn(core.ContinueOn{
					Failure: true,
				}),
			),
			newStep("step2",
				withDepends("step1"),
				withCommand("false"),
				withContinueOn(core.ContinueOn{
					Failure: true,
				}),
			),
			successStep("step3", "step2"),
		)

		// The overall DAG should complete with partial success
		result := plan.assertRun(t, core.PartiallySucceeded)

		// Verify individual node statuses
		result.assertNodeStatus(t, "step1", core.NodeFailed)
		result.assertNodeStatus(t, "step2", core.NodeFailed)
		result.assertNodeStatus(t, "step3", core.NodeSucceeded)
	})

	t.Run("NoSuccessfulStepsWithContinueOn", func(t *testing.T) {
		r := setupRunner(t)

		// Create a plan where all steps fail but have continueOn
		// This should still be an error, not partial success,
		// because partial success requires at least one successful step
		plan := r.newPlan(t,
			newStep("step1",
				withCommand("false"),
				withContinueOn(core.ContinueOn{
					Failure: true,
				}),
			),
			newStep("step2",
				withDepends("step1"),
				withCommand("false"),
				withContinueOn(core.ContinueOn{
					Failure: true,
				}),
			),
		)

		// The overall DAG should complete with error since no steps succeeded
		result := plan.assertRun(t, core.Failed)

		// Verify individual node statuses
		result.assertNodeStatus(t, "step1", core.NodeFailed)
		result.assertNodeStatus(t, "step2", core.NodeFailed)
	})

	t.Run("FailureWithoutContinueOn", func(t *testing.T) {
		r := setupRunner(t)

		// Create a plan where a step fails without continueOn
		// This should result in an error status, not partial success
		plan := r.newPlan(t,
			successStep("step1"),
			failStep("step2", "step1"),    // This will fail without continueOn
			successStep("step3", "step1"), // This depends on step1, not step2
		)

		// The overall DAG should complete with error
		result := plan.assertRun(t, core.Failed)

		// Verify individual node statuses
		result.assertNodeStatus(t, "step1", core.NodeSucceeded)
		result.assertNodeStatus(t, "step2", core.NodeFailed)
		result.assertNodeStatus(t, "step3", core.NodeSucceeded)
	})
}

func TestRunner_DeadlockDetection(t *testing.T) {
	t.Parallel()

	steps := []core.Step{
		{Name: "a"},
		{Name: "b", Depends: []string{"a"}},
	}

	plan, err := runtime.NewPlan(steps...)
	require.NoError(t, err)

	// Corrupt dependencies to create an unschedulable plan (self-dependency).
	for _, n := range plan.Nodes() {
		plan.DependencyMap[n.ID()] = []int{n.ID()}
	}

	cfg := &runtime.Config{
		LogDir:   t.TempDir(),
		DAGRunID: uuid.NewString(),
	}
	r := runtime.New(cfg)
	dag := &core.DAG{Name: "deadlock_dag"}
	logFile := filepath.Join(cfg.LogDir, dag.Name+".log")
	ctx := execution.NewContext(context.Background(), dag, cfg.DAGRunID, logFile)

	progressCh := make(chan *runtime.Node, 2)
	err = r.Run(ctx, plan, progressCh)

	require.ErrorIs(t, err, runtime.ErrDeadlockDetected)
	require.Equal(t, core.Failed, r.Status(ctx, plan))
}

func TestNewEnvWithStepInfo(t *testing.T) {
	t.Parallel()

	plan, err := runtime.NewPlan(
		core.Step{ID: "s1", Name: "s1"},
		core.Step{ID: "s2", Name: "s2"},
		core.Step{Name: "no-id"},
	)
	require.NoError(t, err)

	env := runtime.NewPlanEnv(context.Background(), core.Step{Name: "current"}, plan)

	require.Len(t, env.StepMap, 2)
	require.Contains(t, env.StepMap, "s1")
	require.Contains(t, env.StepMap, "s2")
	require.Equal(t, "0", env.StepMap["s1"].ExitCode)
	require.Equal(t, "0", env.StepMap["s2"].ExitCode)
	require.NotContains(t, env.StepMap, "no-id")
}
