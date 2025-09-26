package scheduler_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	testScript := test.TestdataPath(t, filepath.Join("digraph", "scheduler", "testfile.sh"))

	t.Run("SequentialStepsSuccess", func(t *testing.T) {
		t.Parallel()
		sc := setupScheduler(t, withMaxActiveRuns(1))

		// 1 -> 2 -> 3
		graph := sc.newGraph(t,
			successStep("1"),
			successStep("2", "1"),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
	})
	t.Run("SequentialStepsWithFailure", func(t *testing.T) {
		t.Parallel()
		sc := setupScheduler(t, withMaxActiveRuns(1))

		// 1 -> 2 -> 3 -> 4
		graph := sc.newGraph(t,
			successStep("1"),
			successStep("2", "1"),
			failStep("3", "2"),
			successStep("4", "3"),
		)

		result := graph.Schedule(t, status.Error)

		// 1, 2, 3 should be executed and 4 should be canceled because 3 failed
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
		result.AssertNodeStatus(t, "3", status.NodeError)
		result.AssertNodeStatus(t, "4", status.NodeCancel)
	})
	t.Run("ParallelSteps", func(t *testing.T) {
		t.Parallel()
		sc := setupScheduler(t, withMaxActiveRuns(3))

		// 1,2,3
		graph := sc.newGraph(t,
			successStep("1"),
			successStep("2"),
			successStep("3"),
		)

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
	})
	t.Run("ParallelStepsWithFailure", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 3 -> 4, 2 (fail)
		graph := sc.newGraph(t,
			successStep("1"),
			failStep("2"),
			successStep("3", "1"),
			successStep("4", "3"),
		)

		result := graph.Schedule(t, status.Error)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeError)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
		result.AssertNodeStatus(t, "4", status.NodeSuccess)
	})
	t.Run("ComplexCommand", func(t *testing.T) {
		t.Parallel()
		sc := setupScheduler(t, withMaxActiveRuns(1))

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("df / | awk 'NR==2 {exit $4 > 5000 ? 0 : 1}'"),
			))

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
	})
	t.Run("ContinueOnFailure", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 2 (fail) -> 3
		graph := sc.newGraph(t,
			successStep("1"),
			newStep("2",
				withDepends("1"),
				withCommand("false"),
				withContinueOn(digraph.ContinueOn{
					Failure: true,
				}),
			),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, status.PartialSuccess)

		// 1, 2, 3 should be executed even though 2 failed
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeError)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
	})
	t.Run("ContinueOnSkip", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 2 (skip) -> 3
		graph := sc.newGraph(t,
			successStep("1"),
			newStep("2",
				withDepends("1"),
				withCommand("false"),
				withPrecondition(&digraph.Condition{
					Condition: "`echo 1`",
					Expected:  "0",
				}),
				withContinueOn(digraph.ContinueOn{
					Skipped: true,
				}),
			),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSkipped)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
	})
	t.Run("ContinueOnExitCode", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 (exit code 1) -> 2
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("false"),
				withContinueOn(digraph.ContinueOn{
					ExitCode: []int{1},
				}),
			),
			successStep("2", "1"),
		)

		result := graph.Schedule(t, status.PartialSuccess)

		// 1, 2 should be executed even though 1 failed
		result.AssertNodeStatus(t, "1", status.NodeError)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
	})
	t.Run("ContinueOnOutputStdout", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 (exit code 1) -> 2
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo test_output; false"), // stdout: test_output
				withContinueOn(digraph.ContinueOn{
					Output: []string{
						"test_output",
					},
				}),
			),
			successStep("2", "1"),
		)

		result := graph.Schedule(t, status.PartialSuccess)

		// 1, 2 should be executed even though 1 failed
		result.AssertNodeStatus(t, "1", status.NodeError)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
	})
	t.Run("ContinueOnOutputStderr", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 (exit code 1) -> 2
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo test_output >&2; echo test_output; false"), // write to stderr and stdout
				withContinueOn(digraph.ContinueOn{
					Output: []string{
						"test_output",
					},
				}),
			),
			successStep("2", "1"),
		)

		result := graph.Schedule(t, status.PartialSuccess)

		// Step 1 fails but matches continueOn output, allowing step 2 to run
		result.AssertNodeStatus(t, "1", status.NodeError)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)

		node := result.Node(t, "1")
		stderrData, err := os.ReadFile(node.GetStderr())
		require.NoError(t, err)
		assert.Contains(t, string(stderrData), "test_output")
	})
	t.Run("ContinueOnOutputRegexp", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 (exit code 1) -> 2
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo test_output; false"), // stdout: test_output
				withContinueOn(digraph.ContinueOn{
					Output: []string{
						"re:^test_[a-z]+$",
					},
				}),
			),
			successStep("2", "1"),
		)

		result := graph.Schedule(t, status.PartialSuccess)

		// 1, 2 should be executed even though 1 failed
		result.AssertNodeStatus(t, "1", status.NodeError)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
	})
	t.Run("ContinueOnMarkSuccess", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 (exit code 1) -> 2
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("false"),
				withContinueOn(digraph.ContinueOn{
					ExitCode:    []int{1},
					MarkSuccess: true,
				}),
			),
			successStep("2", "1"),
		)

		result := graph.Schedule(t, status.Success)

		// 1, 2 should be executed even though 1 failed
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
	})
	t.Run("CancelSchedule", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 2 (cancel when running) -> 3 (should not be executed)
		graph := sc.newGraph(t,
			successStep("1"),
			newStep("2", withDepends("1"), withCommand("sleep 0.5")),
			failStep("3", "2"),
		)

		go func() {
			time.Sleep(time.Millisecond * 200) // wait for step 2 to start
			graph.Cancel(t)
		}()

		result := graph.Schedule(t, status.Cancel)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeCancel)
		result.AssertNodeStatus(t, "3", status.NodeNone)
	})
	t.Run("Timeout", func(t *testing.T) {
		sc := setupScheduler(t, withTimeout(time.Millisecond*500))

		// 1 -> 2 (timeout) -> 3 (should not be executed)
		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 0.1")),
			newStep("2", withCommand("sleep 0.5"), withDepends("1")),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, status.Error)

		// 1 should be executed and 2 should be canceled because of timeout
		// 3 should not be executed and should be canceled
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeCancel)
		result.AssertNodeStatus(t, "3", status.NodeCancel)
	})
	t.Run("RetryPolicyFail", func(t *testing.T) {
		const file = "flag_test_retry_fail"

		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand(fmt.Sprintf("%s %s", testScript, file)),
				withRetryPolicy(2, 0),
			),
		)

		result := graph.Schedule(t, status.Error)

		result.AssertNodeStatus(t, "1", status.NodeError)

		node := result.Node(t, "1")
		require.Equal(t, 2, node.State().RetryCount) // 2 retry
	})
	t.Run("RetryWithScript", func(t *testing.T) {
		sc := setupScheduler(t)

		const testEnv = "TEST_RETRY_WITH_SCRIPT"
		graph := sc.newGraph(t,
			newStep("1",
				withScript(`
					if [ "$TEST_RETRY_WITH_SCRIPT" -eq 1 ]; then
						exit 1
					fi
					exit 0
				`),
				withRetryPolicy(1, time.Millisecond*50),
			),
		)

		_ = os.Setenv(testEnv, "1")
		go func() {
			time.Sleep(time.Millisecond * 50)
			_ = os.Setenv(testEnv, "0")
			t.Cleanup(func() {
				_ = os.Unsetenv(testEnv)
			})
		}()

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		require.Equal(t, 1, node.State().DoneCount)  // 1 successful execution
		require.Equal(t, 1, node.State().RetryCount) // 1 retry
	})
	t.Run("RetryPolicySuccess", func(t *testing.T) {
		file := filepath.Join(
			os.TempDir(), fmt.Sprintf("flag_test_retry_success_%s", uuid.Must(uuid.NewV7()).String()),
		)

		sc := setupScheduler(t)

		graph := sc.newGraph(t,
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

		result := graph.Schedule(t, status.Success)

		// Check if the retry is successful
		state := result.Node(t, "1").State()
		assert.Equal(t, 1, state.DoneCount)
		assert.Greater(t, state.RetryCount, 0)
		assert.NotEmpty(t, state.RetriedAt)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
	})
	t.Run("PreconditionMatch", func(t *testing.T) {
		t.Parallel()
		sc := setupScheduler(t)

		// 1 -> 2 (precondition match) -> 3
		graph := sc.newGraph(t,
			successStep("1"),
			newStep("2", withCommand("echo 2"),
				withPrecondition(&digraph.Condition{
					Condition: "`echo 1`",
					Expected:  "1",
				}),
			),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
	})
	t.Run("PreconditionNotMatch", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 2 (precondition not match) -> 3
		graph := sc.newGraph(t,
			successStep("1"),
			newStep("2", withCommand("echo 2"),
				withPrecondition(&digraph.Condition{
					Condition: "`echo 1`",
					Expected:  "0",
				})),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, status.Success)

		// 1 should be executed and 2, 3 should be skipped
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSkipped)
		result.AssertNodeStatus(t, "3", status.NodeSkipped)
	})
	t.Run("PreconditionWithCommandMet", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 2 (precondition not match) -> 3
		graph := sc.newGraph(t,
			successStep("1"),
			newStep("2", withCommand("echo 2"),
				withPrecondition(&digraph.Condition{
					Condition: "true",
				})),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
	})
	t.Run("PreconditionWithCommandNotMet", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 2 (precondition not match) -> 3
		graph := sc.newGraph(t,
			successStep("1"),
			newStep("2", withCommand("echo 2"),
				withPrecondition(&digraph.Condition{
					Condition: "false",
				})),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, status.Success)

		// 1 should be executed and 2, 3 should be skipped
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSkipped)
		result.AssertNodeStatus(t, "3", status.NodeSkipped)
	})
	t.Run("OnExitHandler", func(t *testing.T) {
		sc := setupScheduler(t, withOnExit(successStep("onExit")))

		graph := sc.newGraph(t, successStep("1"))

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "onExit", status.NodeSuccess)
	})
	t.Run("OnExitHandlerFail", func(t *testing.T) {
		sc := setupScheduler(t, withOnExit(failStep("onExit")))

		graph := sc.newGraph(t, successStep("1"))

		// Overall status should be error because onExit failed
		result := graph.Schedule(t, status.Error)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "onExit", status.NodeError)
	})
	t.Run("OnCancelHandler", func(t *testing.T) {
		sc := setupScheduler(t, withOnCancel(successStep("onCancel")))

		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 0.5")),
		)

		go func() {
			time.Sleep(time.Millisecond * 30) // wait for step 1 to start
			graph.Signal(syscall.SIGTERM)
		}()

		result := graph.Schedule(t, status.Cancel)

		result.AssertNodeStatus(t, "1", status.NodeCancel)
		result.AssertNodeStatus(t, "onCancel", status.NodeSuccess)
	})
	t.Run("OnSuccessHandler", func(t *testing.T) {
		sc := setupScheduler(t, withOnSuccess(successStep("onSuccess")))

		graph := sc.newGraph(t, successStep("1"))

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "onSuccess", status.NodeSuccess)
	})
	t.Run("OnFailureHandler", func(t *testing.T) {
		sc := setupScheduler(t, withOnFailure(successStep("onFailure")))

		graph := sc.newGraph(t, failStep("1"))

		result := graph.Schedule(t, status.Error)

		result.AssertNodeStatus(t, "1", status.NodeError)
		result.AssertNodeStatus(t, "onFailure", status.NodeSuccess)
	})
	t.Run("CancelOnSignal", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 0.5")),
		)

		go func() {
			time.Sleep(time.Millisecond * 30) // wait for step 1 to start
			graph.Signal(syscall.SIGTERM)
		}()

		result := graph.Schedule(t, status.Cancel)

		result.AssertNodeStatus(t, "1", status.NodeCancel)
	})
	t.Run("Repeat", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("sleep 0.1"),
				withRepeatPolicy(true, time.Millisecond*100),
			),
		)

		go func() {
			time.Sleep(time.Millisecond * 250)
			graph.Cancel(t)
		}()

		result := graph.Schedule(t, status.Cancel)

		// 1 should be repeated 2 times
		result.AssertNodeStatus(t, "1", status.NodeCancel)

		node := result.Node(t, "1")
		// done count should be 1 because 2nd execution is canceled
		require.Equal(t, 1, node.State().DoneCount)
	})
	t.Run("RepeatFail", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("false"),
				withRepeatPolicy(true, time.Millisecond*50),
			),
		)

		result := graph.Schedule(t, status.Error)

		// Done count should be 1 because it failed and not repeated
		result.AssertNodeStatus(t, "1", status.NodeError)

		node := result.Node(t, "1")
		require.Equal(t, 1, node.State().DoneCount)
	})
	t.Run("StopRepetitiveTaskGracefully", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("sleep 0.1"),
				withRepeatPolicy(true, time.Millisecond*50),
			),
		)

		done := make(chan struct{})
		go func() {
			time.Sleep(time.Millisecond * 100)
			graph.Signal(syscall.SIGTERM)
			close(done)
		}()

		result := graph.Schedule(t, status.Success)
		<-done

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
	})
	t.Run("NodeSetupFailure", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withWorkingDir("/nonexistent"),
				withScript("echo 1"),
			),
		)

		result := graph.Schedule(t, status.Error)

		result.AssertNodeStatus(t, "1", status.NodeError)

		require.Contains(t, result.Error.Error(), "no such file or directory")
	})
	t.Run("OutputVariables", func(t *testing.T) {
		t.Parallel()
		sc := setupScheduler(t)

		// 1: echo hello > OUT
		// 2: echo $OUT > RESULT
		graph := sc.newGraph(t,
			newStep("1", withCommand("echo hello"), withOutput("OUT")),
			newStep("2", withCommand("echo $OUT"), withDepends("1"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)

		node := result.Node(t, "2")

		// check if RESULT variable is set to "hello"
		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=hello", output, "expected output %q, got %q", "hello", output)
	})
	t.Run("OutputInheritance", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1: echo hello > OUT
		// 2: echo world > OUT2 (depends on 1)
		// 3: echo $OUT $OUT2 > RESULT (depends on 2)
		// RESULT should be "hello world"
		graph := sc.newGraph(t,
			newStep("1", withCommand("echo hello"), withOutput("OUT")),
			newStep("2", withCommand("echo world"), withOutput("OUT2"), withDepends("1")),
			newStep("3", withCommand("echo $OUT $OUT2"), withDepends("2"), withOutput("RESULT")),
			newStep("4", withCommand("sleep 0.1")),
			// 5 should not have reference to OUT or OUT2
			newStep("5", withCommand("echo $OUT $OUT2"), withDepends("4"), withOutput("RESULT2")),
		)

		result := graph.Schedule(t, status.Success)

		node := result.Node(t, "3")
		output, _ := node.NodeData().State.OutputVariables.Load("RESULT")
		require.Equal(t, "RESULT=hello world", output, "expected output %q, got %q", "hello world", output)

		node2 := result.Node(t, "5")
		output2, _ := node2.NodeData().State.OutputVariables.Load("RESULT2")
		require.Equal(t, "RESULT2=", output2, "expected output %q, got %q", "", output)
	})
	t.Run("OutputJSONReference", func(t *testing.T) {
		sc := setupScheduler(t)

		jsonData := `{"key": "value"}`
		graph := sc.newGraph(t,
			newStep("1", withCommand(fmt.Sprintf("echo '%s'", jsonData)), withOutput("OUT")),
			newStep("2", withCommand("echo ${OUT.key}"), withDepends("1"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)

		// check if RESULT variable is set to "value"
		node := result.Node(t, "2")

		output, _ := node.NodeData().State.OutputVariables.Load("RESULT")
		require.Equal(t, "RESULT=value", output, "expected output %q, got %q", "value", output)
	})
	t.Run("HandlingJSONWithSpecialChars", func(t *testing.T) {
		sc := setupScheduler(t)

		jsonData := `{\n\t"key": "value"\n}`
		graph := sc.newGraph(t,
			newStep("1", withCommand(fmt.Sprintf("echo '%s'", jsonData)), withOutput("OUT")),
			newStep("2", withCommand("echo '${OUT.key}'"), withDepends("1"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)

		// check if RESULT variable is set to "value"
		node := result.Node(t, "2")

		output, _ := node.NodeData().State.OutputVariables.Load("RESULT")
		require.Equal(t, "RESULT=value", output, "expected output %q, got %q", "value", output)
	})
	t.Run("SpecialVarsDAGRUNLOGFILE", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_RUN_LOG_FILE"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.log$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGRUNSTEPSTDOUTFILE", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_RUN_STEP_STDOUT_FILE"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.out$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGRUNSTEPSTDERRFILE", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_RUN_STEP_STDERR_FILE"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.err$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGRUNID", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_RUN_ID"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `RESULT=[a-f0-9-]+`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGNAME", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_NAME"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=test_dag", output, "unexpected output %q", output)
	})
	t.Run("SpecialVarsDAGRUNSTEPNAME", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("step_test", withCommand("echo $DAG_RUN_STEP_NAME"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, status.Success)
		node := result.Node(t, "step_test")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=step_test", output, "unexpected output %q", output)
	})

	t.Run("RepeatPolicyRepeatsUntilCommandConditionMatchesExpected", func(t *testing.T) {
		sc := setupScheduler(t)

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
		graph := sc.newGraph(t,
			newStep("1",
				withCommand(fmt.Sprintf("cat %s || true", file)),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
					step.RepeatPolicy.Condition = &digraph.Condition{
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

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		// Should have run at least twice (first: not ready, second: ready)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyRepeatWhileConditionExits0", func(t *testing.T) {
		sc := setupScheduler(t)
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
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo hello"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeWhile
					step.RepeatPolicy.Condition = &digraph.Condition{
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
		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyRepeatsWhileCommandExitCodeMatches", func(t *testing.T) {
		sc := setupScheduler(t)
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
		graph := sc.newGraph(t,
			newStep("1",
				withScript(script),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeWhile
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
		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyRepeatsUntilEnvVarConditionMatchesExpected", func(t *testing.T) {
		sc := setupScheduler(t)
		// This step will repeat until the environment variable TEST_REPEAT_MATCH_EXPR equals 'done'
		err := os.Setenv("TEST_REPEAT_MATCH_EXPR", "notyet")
		require.NoError(t, err)
		t.Cleanup(func() {
			err := os.Unsetenv("TEST_REPEAT_MATCH_EXPR")
			require.NoError(t, err)
		})
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo $TEST_REPEAT_MATCH_EXPR"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
					step.RepeatPolicy.Condition = &digraph.Condition{
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
		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyRepeatsUntilOutputVarConditionMatchesExpected", func(t *testing.T) {
		sc := setupScheduler(t)
		file := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_outputvar_%s", uuid.Must(uuid.NewV7()).String()))
		err := os.Remove(file)
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
		t.Cleanup(func() { err := os.Remove(file); require.NoError(t, err) })
		// Write initial value
		err = os.WriteFile(file, []byte("notyet"), 0600)
		require.NoError(t, err)
		graph := sc.newGraph(t,
			newStep("1",
				withCommand(fmt.Sprintf("cat %s", file)),
				withOutput("OUT"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
					step.RepeatPolicy.Condition = &digraph.Condition{
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
		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})
	t.Run("RetryPolicyWithOutputCapture", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a counter file for tracking retry attempts
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("retry_output_%s.txt", uuid.Must(uuid.NewV7()).String()))
		defer func() {
			_ = os.Remove(counterFile)
		}()

		// Step that outputs different values on each retry and fails until 3rd attempt
		graph := sc.newGraph(t,
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

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		require.Equal(t, 1, node.State().DoneCount)  // 1 successful execution
		require.Equal(t, 2, node.State().RetryCount) // 2 retries

		// Verify that output contains only the final successful attempt's output
		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=output_attempt_3_success", output, "expected final output, got %q", output)
	})
	t.Run("FailedStepWithOutputCapture", func(t *testing.T) {
		sc := setupScheduler(t)

		// Step that outputs data but fails
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo 'error_output'; exit 1"),
				withOutput("ERROR_MSG"),
			),
		)

		result := graph.Schedule(t, status.Error)

		result.AssertNodeStatus(t, "1", status.NodeError)

		node := result.Node(t, "1")

		// Verify that output is captured even on failure
		output, ok := node.NodeData().State.OutputVariables.Load("ERROR_MSG")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "ERROR_MSG=error_output", output, "expected output %q, got %q", "error_output", output)
	})
	t.Run("RetryPolicyChildDAGRunWithOutputCapture", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a counter file for tracking retry attempts
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("retry_child_output_%s.txt", uuid.Must(uuid.NewV7()).String()))
		defer func() {
			_ = os.Remove(counterFile)
		}()

		// Step that outputs different values on each retry and always fails
		graph := sc.newGraph(t,
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

		result := graph.Schedule(t, status.Error)

		result.AssertNodeStatus(t, "1", status.NodeError)

		node := result.Node(t, "1")
		require.Equal(t, 1, node.State().DoneCount)  // 1 execution (failed)
		require.Equal(t, 2, node.State().RetryCount) // 2 retries

		// Verify that output contains the final retry attempt's output
		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=output_attempt_3", output, "expected final output, got %q", output)
	})
}

func successStep(name string, depends ...string) digraph.Step {
	return newStep(name, withDepends(depends...), withCommand("true"))
}

func failStep(name string, depends ...string) digraph.Step {
	return newStep(name, withDepends(depends...), withCommand("false"))
}

type stepOption func(*digraph.Step)

func withDepends(depends ...string) stepOption {
	return func(step *digraph.Step) {
		step.Depends = depends
	}
}

func withContinueOn(c digraph.ContinueOn) stepOption {
	return func(step *digraph.Step) {
		step.ContinueOn = c
	}
}

func withRetryPolicy(limit int, interval time.Duration) stepOption {
	return func(step *digraph.Step) {
		step.RetryPolicy.Limit = limit
		step.RetryPolicy.Interval = interval
	}
}

func withRepeatPolicy(repeat bool, interval time.Duration) stepOption {
	return func(step *digraph.Step) {
		if repeat {
			step.RepeatPolicy.RepeatMode = digraph.RepeatModeWhile
		}
		step.RepeatPolicy.Interval = interval
	}
}

func withPrecondition(condition *digraph.Condition) stepOption {
	return func(step *digraph.Step) {
		step.Preconditions = []*digraph.Condition{condition}
	}
}

func withScript(script string) stepOption {
	return func(step *digraph.Step) {
		step.Script = script
	}
}

func withWorkingDir(dir string) stepOption {
	return func(step *digraph.Step) {
		step.Dir = dir
	}
}

func withOutput(output string) stepOption {
	return func(step *digraph.Step) {
		step.Output = output
	}
}

func withCommand(command string) stepOption {
	return func(step *digraph.Step) {
		cmd, args, err := cmdutil.SplitCommand(command)
		if err != nil {
			panic(fmt.Errorf("unexpected: %w", err))
		}
		step.CmdWithArgs = command
		step.Command = cmd
		step.Args = args
	}
}

func withID(id string) stepOption {
	return func(step *digraph.Step) {
		step.ID = id
	}
}

func newStep(name string, opts ...stepOption) digraph.Step {
	step := digraph.Step{Name: name}
	for _, opt := range opts {
		opt(&step)
	}

	return step
}

type testHelper struct {
	test.Helper

	Scheduler *scheduler.Scheduler
	Config    *scheduler.Config
}

type schedulerOption func(*scheduler.Config)

func withTimeout(d time.Duration) schedulerOption {
	return func(cfg *scheduler.Config) {
		cfg.Timeout = d
	}
}

func withMaxActiveRuns(n int) schedulerOption {
	return func(cfg *scheduler.Config) {
		cfg.MaxActiveSteps = n
	}
}

func withOnExit(step digraph.Step) schedulerOption {
	return func(cfg *scheduler.Config) {
		cfg.OnExit = &step
	}
}

func withOnCancel(step digraph.Step) schedulerOption {
	return func(cfg *scheduler.Config) {
		cfg.OnCancel = &step
	}
}

func withOnSuccess(step digraph.Step) schedulerOption {
	return func(cfg *scheduler.Config) {
		cfg.OnSuccess = &step
	}
}

func withOnFailure(step digraph.Step) schedulerOption {
	return func(cfg *scheduler.Config) {
		cfg.OnFailure = &step
	}
}

func setupScheduler(t *testing.T, opts ...schedulerOption) testHelper {
	t.Helper()

	th := test.Setup(t)

	cfg := &scheduler.Config{
		LogDir:   th.Config.Paths.LogDir,
		DAGRunID: uuid.Must(uuid.NewV7()).String(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	sc := scheduler.New(cfg)

	return testHelper{
		Helper:    test.Setup(t),
		Scheduler: sc,
		Config:    cfg,
	}
}

func (th testHelper) newGraph(t *testing.T, steps ...digraph.Step) graphHelper {
	t.Helper()

	graph, err := scheduler.NewExecutionGraph(steps...)
	require.NoError(t, err)

	return graphHelper{
		testHelper:     th,
		ExecutionGraph: graph,
	}
}

type graphHelper struct {
	testHelper
	*scheduler.ExecutionGraph
}

func (gh graphHelper) Schedule(t *testing.T, expectedStatus status.Status) scheduleResult {
	t.Helper()

	dag := &digraph.DAG{Name: "test_dag"}
	logFilename := fmt.Sprintf("%s_%s.log", dag.Name, gh.Config.DAGRunID)
	logFilePath := path.Join(gh.Config.LogDir, logFilename)

	ctx := digraph.SetupEnvForTest(gh.Context, dag, nil, digraph.DAGRunRef{}, gh.Config.DAGRunID, logFilePath, nil)

	var doneNodes []*scheduler.Node
	progressCh := make(chan *scheduler.Node)

	done := make(chan struct{})
	go func() {
		for node := range progressCh {
			doneNodes = append(doneNodes, node)
		}
		done <- struct{}{}
	}()

	err := gh.Scheduler.Schedule(ctx, gh.ExecutionGraph, progressCh)

	close(progressCh)

	switch expectedStatus {
	case status.Success, status.Cancel:
		require.NoError(t, err)

	case status.Error, status.PartialSuccess:
		require.Error(t, err)

	case status.Running, status.None, status.Queued:
		t.Errorf("unexpected status %s", expectedStatus)

	}

	require.Equal(t, expectedStatus.String(), gh.Scheduler.Status(ctx, gh.ExecutionGraph).String(),
		"expected status %s, got %s", expectedStatus, gh.Scheduler.Status(ctx, gh.ExecutionGraph))

	// wait for items of nodeCompletedChan to be processed
	<-done
	close(done)

	return scheduleResult{
		graphHelper: gh,
		Done:        doneNodes,
		Error:       err,
	}
}

func (gh graphHelper) Signal(sig syscall.Signal) {
	gh.Scheduler.Signal(gh.Context, gh.ExecutionGraph, sig, nil, false)
}

func (gh graphHelper) Cancel(t *testing.T) {
	t.Helper()

	gh.Scheduler.Cancel(gh.Context, gh.ExecutionGraph)
}

type scheduleResult struct {
	graphHelper
	Done  []*scheduler.Node
	Error error
}

func (sr scheduleResult) AssertDoneCount(t *testing.T, expected int) {
	t.Helper()

	require.Len(t, sr.Done, expected, "expected %d done nodes, got %d", expected, len(sr.Done))
}

func (sr scheduleResult) AssertNodeStatus(t *testing.T, stepName string, expected status.NodeStatus) {
	t.Helper()

	target := sr.NodeByName(stepName)
	if target == nil {
		if sr.Config.OnExit != nil && sr.Config.OnExit.Name == stepName {
			target = sr.Scheduler.HandlerNode(digraph.HandlerOnExit)
		}
		if sr.Config.OnSuccess != nil && sr.Config.OnSuccess.Name == stepName {
			target = sr.Scheduler.HandlerNode(digraph.HandlerOnSuccess)
		}
		if sr.Config.OnFailure != nil && sr.Config.OnFailure.Name == stepName {
			target = sr.Scheduler.HandlerNode(digraph.HandlerOnFailure)
		}
		if sr.Config.OnCancel != nil && sr.Config.OnCancel.Name == stepName {
			target = sr.Scheduler.HandlerNode(digraph.HandlerOnCancel)
		}
	}

	if target == nil {
		t.Fatalf("step %s not found", stepName)
	}

	require.Equal(t, expected.String(), target.State().Status.String(), "expected status %q, got %q", expected.String(), target.State().Status.String())
}

func (sr scheduleResult) Node(t *testing.T, stepName string) *scheduler.Node {
	t.Helper()

	if node := sr.NodeByName(stepName); node != nil {
		return node
	}

	if sr.Config.OnExit != nil && sr.Config.OnExit.Name == stepName {
		return sr.Scheduler.HandlerNode(digraph.HandlerOnExit)
	}
	if sr.Config.OnSuccess != nil && sr.Config.OnSuccess.Name == stepName {
		return sr.Scheduler.HandlerNode(digraph.HandlerOnSuccess)
	}
	if sr.Config.OnFailure != nil && sr.Config.OnFailure.Name == stepName {
		return sr.Scheduler.HandlerNode(digraph.HandlerOnFailure)
	}
	if sr.Config.OnCancel != nil && sr.Config.OnCancel.Name == stepName {
		return sr.Scheduler.HandlerNode(digraph.HandlerOnCancel)
	}

	t.Fatalf("step %s not found", stepName)
	return nil
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   status.Status
		expected string
	}{
		{status.None, "not started"},
		{status.Running, "running"},
		{status.Error, "failed"},
		{status.Cancel, "cancelled"},
		{status.Success, "finished"},
		{status.Queued, "queued"},
		{status.Status(999), "not started"}, // Invalid status defaults to "not started"
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestStatus_IsActive(t *testing.T) {
	tests := []struct {
		status   status.Status
		expected bool
	}{
		{status.None, false},
		{status.Running, true},
		{status.Error, false},
		{status.Cancel, false},
		{status.Success, false},
		{status.Queued, true},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsActive())
		})
	}
}

func TestScheduler_DryRun(t *testing.T) {
	sc := setupScheduler(t, func(cfg *scheduler.Config) {
		cfg.Dry = true
	})

	graph := sc.newGraph(t,
		successStep("1"),
		successStep("2", "1"),
		successStep("3", "2"),
	)

	result := graph.Schedule(t, status.Success)

	// In dry run, steps should be marked as success without actual execution
	result.AssertNodeStatus(t, "1", status.NodeSuccess)
	result.AssertNodeStatus(t, "2", status.NodeSuccess)
	result.AssertNodeStatus(t, "3", status.NodeSuccess)
}

func TestScheduler_DryRunWithHandlers(t *testing.T) {
	sc := setupScheduler(t,
		func(cfg *scheduler.Config) {
			cfg.Dry = true
		},
		withOnExit(successStep("onExit")),
		withOnSuccess(successStep("onSuccess")),
	)

	graph := sc.newGraph(t, successStep("1"))

	result := graph.Schedule(t, status.Success)

	result.AssertNodeStatus(t, "1", status.NodeSuccess)
	result.AssertNodeStatus(t, "onExit", status.NodeSuccess)
	result.AssertNodeStatus(t, "onSuccess", status.NodeSuccess)
}

func TestScheduler_ConcurrentExecution(t *testing.T) {
	steps := func() []digraph.Step {
		return []digraph.Step{
			newStep("1", withScript("sleep 0.3")),
			newStep("2", withScript("sleep 0.3")),
			newStep("3", withScript("sleep 0.3")),
		}
	}

	sequential := setupScheduler(t, withMaxActiveRuns(1))
	graphSequential := sequential.newGraph(t, steps()...)
	startSequential := time.Now()
	resultSequential := graphSequential.Schedule(t, status.Success)
	elapsedSequential := time.Since(startSequential)
	resultSequential.AssertNodeStatus(t, "1", status.NodeSuccess)
	resultSequential.AssertNodeStatus(t, "2", status.NodeSuccess)
	resultSequential.AssertNodeStatus(t, "3", status.NodeSuccess)

	concurrent := setupScheduler(t, withMaxActiveRuns(3))
	graphConcurrent := concurrent.newGraph(t, steps()...)
	startConcurrent := time.Now()
	resultConcurrent := graphConcurrent.Schedule(t, status.Success)
	elapsedConcurrent := time.Since(startConcurrent)
	resultConcurrent.AssertNodeStatus(t, "1", status.NodeSuccess)
	resultConcurrent.AssertNodeStatus(t, "2", status.NodeSuccess)
	resultConcurrent.AssertNodeStatus(t, "3", status.NodeSuccess)

	assert.Greater(t, elapsedSequential, elapsedConcurrent)
	assert.Greater(t, elapsedSequential-elapsedConcurrent, 200*time.Millisecond)
}

func TestScheduler_ErrorHandling(t *testing.T) {
	t.Run("SetupError", func(t *testing.T) {
		// Create a scheduler with invalid log directory
		invalidLogDir := "/nonexistent/path/that/should/not/exist"
		sc := setupScheduler(t, func(cfg *scheduler.Config) {
			cfg.LogDir = invalidLogDir
		})

		graph := sc.newGraph(t, successStep("1"))

		// Should fail during setup
		dag := &digraph.DAG{Name: "test_dag"}
		logFilename := fmt.Sprintf("%s_%s.log", dag.Name, sc.Config.DAGRunID)
		logFilePath := filepath.Join(sc.Config.LogDir, logFilename)

		ctx := digraph.SetupEnvForTest(sc.Context, dag, nil, digraph.DAGRunRef{}, sc.Config.DAGRunID, logFilePath, nil)

		err := sc.Scheduler.Schedule(ctx, graph.ExecutionGraph, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create log directory")
	})

	t.Run("PanicRecovery", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a step that will panic
		panicStep := newStep("panic", withScript(`
			echo "About to panic"
			# Simulate a panic by killing the process with an invalid signal
			kill -99 $$
		`))

		graph := sc.newGraph(t, panicStep)

		// The scheduler should recover from the panic and mark the step as error
		result := graph.Schedule(t, status.Error)
		result.AssertNodeStatus(t, "panic", status.NodeError)
	})
}

func TestScheduler_Metrics(t *testing.T) {
	sc := setupScheduler(t)

	graph := sc.newGraph(t,
		successStep("1"),
		failStep("2"),
		newStep("3", withPrecondition(&digraph.Condition{
			Condition: "false",
		})),
		successStep("4", "1"),
	)

	result := graph.Schedule(t, status.Error)

	// Get metrics
	metrics := sc.Scheduler.GetMetrics()

	assert.Equal(t, 4, metrics["totalNodes"])
	assert.Equal(t, 2, metrics["completedNodes"]) // 1 and 4
	assert.Equal(t, 1, metrics["failedNodes"])    // 2
	assert.Equal(t, 1, metrics["skippedNodes"])   // 3
	assert.Equal(t, 0, metrics["canceledNodes"])
	assert.NotEmpty(t, metrics["totalExecutionTime"])

	// Verify individual node statuses
	result.AssertNodeStatus(t, "1", status.NodeSuccess)
	result.AssertNodeStatus(t, "2", status.NodeError)
	result.AssertNodeStatus(t, "3", status.NodeSkipped)
	result.AssertNodeStatus(t, "4", status.NodeSuccess)
}

func TestScheduler_DAGPreconditions(t *testing.T) {
	t.Run("DAGPreconditionNotMet", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create DAG with precondition that will fail
		dag := &digraph.DAG{
			Name: "test_dag",
			Preconditions: []*digraph.Condition{
				{
					Condition: "false", // This will fail
				},
			},
		}

		graph := sc.newGraph(t, successStep("1"))

		// Custom schedule with DAG preconditions
		logFilename := fmt.Sprintf("%s_%s.log", dag.Name, sc.Config.DAGRunID)
		logFilePath := filepath.Join(sc.Config.LogDir, logFilename)

		ctx := digraph.SetupEnvForTest(sc.Context, dag, nil, digraph.DAGRunRef{}, sc.Config.DAGRunID, logFilePath, nil)

		err := sc.Scheduler.Schedule(ctx, graph.ExecutionGraph, nil)
		require.NoError(t, err) // No error, but dag should be canceled

		// Check that the scheduler was canceled
		assert.Equal(t, status.Cancel, sc.Scheduler.Status(ctx, graph.ExecutionGraph))
	})
}

func TestScheduler_SignalHandling(t *testing.T) {
	t.Run("SignalWithDoneChannel", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 0.5")),
			successStep("2", "1"),
		)

		done := make(chan bool, 1)

		go func() {
			time.Sleep(100 * time.Millisecond)
			sc.Scheduler.Signal(sc.Context, graph.ExecutionGraph, syscall.SIGTERM, done, false)
		}()

		start := time.Now()
		result := graph.Schedule(t, status.Cancel)

		// Wait for signal completion
		select {
		case <-done:
			// Signal handling completed
		case <-time.After(1 * time.Second):
			t.Fatal("Signal handling did not complete in time")
		}

		elapsed := time.Since(start)
		assert.Less(t, elapsed, 2*time.Second, "Should cancel quickly")

		result.AssertNodeStatus(t, "1", status.NodeCancel)
		result.AssertNodeStatus(t, "2", status.NodeNone)
	})

	t.Run("SignalWithOverride", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 0.5")),
		)

		go func() {
			time.Sleep(100 * time.Millisecond)
			sc.Scheduler.Signal(sc.Context, graph.ExecutionGraph, syscall.SIGKILL, nil, true)
		}()

		result := graph.Schedule(t, status.Cancel)
		result.AssertNodeStatus(t, "1", status.NodeCancel)
	})
}

func TestScheduler_ComplexDependencyChains(t *testing.T) {
	t.Run("DiamondDependency", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create diamond dependency: 1 -> 2,3 -> 4
		graph := sc.newGraph(t,
			successStep("1"),
			successStep("2", "1"),
			successStep("3", "1"),
			successStep("4", "2", "3"),
		)

		result := graph.Schedule(t, status.Success)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeSuccess)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
		result.AssertNodeStatus(t, "4", status.NodeSuccess)
	})

	t.Run("ComplexFailurePropagation", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 2 (fail) -> 4
		//   -> 3 -------->
		graph := sc.newGraph(t,
			successStep("1"),
			failStep("2", "1"),
			successStep("3", "1"),
			successStep("4", "2", "3"),
		)

		result := graph.Schedule(t, status.Error)

		result.AssertNodeStatus(t, "1", status.NodeSuccess)
		result.AssertNodeStatus(t, "2", status.NodeError)
		result.AssertNodeStatus(t, "3", status.NodeSuccess)
		result.AssertNodeStatus(t, "4", status.NodeCancel) // Canceled due to 2's failure
	})
}

func TestScheduler_EdgeCases(t *testing.T) {
	t.Run("EmptyGraph", func(t *testing.T) {
		sc := setupScheduler(t)
		graph := sc.newGraph(t) // No steps

		result := graph.Schedule(t, status.Success)
		assert.NoError(t, result.Error)
	})

	t.Run("SingleNodeGraph", func(t *testing.T) {
		sc := setupScheduler(t)
		graph := sc.newGraph(t, successStep("single"))

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "single", status.NodeSuccess)
	})

	t.Run("AllNodesFail", func(t *testing.T) {
		sc := setupScheduler(t)
		graph := sc.newGraph(t,
			failStep("1"),
			failStep("2"),
			failStep("3"),
		)

		result := graph.Schedule(t, status.Error)
		result.AssertNodeStatus(t, "1", status.NodeError)
		result.AssertNodeStatus(t, "2", status.NodeError)
		result.AssertNodeStatus(t, "3", status.NodeError)
	})
}

func TestScheduler_HandlerNodeAccess(t *testing.T) {
	exitStep := successStep("onExit")
	successHandlerStep := successStep("onSuccess")
	failureStep := successStep("onFailure")
	cancelStep := successStep("onCancel")

	sc := setupScheduler(t,
		withOnExit(exitStep),
		withOnSuccess(successHandlerStep),
		withOnFailure(failureStep),
		withOnCancel(cancelStep),
	)

	// Run a simple graph to trigger setup
	graph := sc.newGraph(t, successStep("1"))
	_ = graph.Schedule(t, status.Success)

	// Access handler nodes
	assert.NotNil(t, sc.Scheduler.HandlerNode(digraph.HandlerOnExit))
	assert.NotNil(t, sc.Scheduler.HandlerNode(digraph.HandlerOnSuccess))
	assert.NotNil(t, sc.Scheduler.HandlerNode(digraph.HandlerOnFailure))
	assert.NotNil(t, sc.Scheduler.HandlerNode(digraph.HandlerOnCancel))
	assert.Nil(t, sc.Scheduler.HandlerNode(digraph.HandlerType("unknown")))
}

func TestScheduler_PreconditionWithError(t *testing.T) {
	sc := setupScheduler(t)

	// Create a step with a precondition that will error (not just return false)
	graph := sc.newGraph(t,
		newStep("1",
			withPrecondition(&digraph.Condition{
				Condition: "exit 2", // Exit with non-zero code
			}),
			withCommand("echo should_not_run"),
		),
	)

	result := graph.Schedule(t, status.Success)

	// The step should be skipped but no error should be set for condition not met
	result.AssertNodeStatus(t, "1", status.NodeSkipped)
	// Conditions that exit with non-zero are just "not met", not errors
}

func TestScheduler_MultipleHandlerExecution(t *testing.T) {
	recordHandler := func(name string) digraph.Step {
		return newStep(name, withScript(fmt.Sprintf(`echo "Handler %s executed"`, name)))
	}

	sc := setupScheduler(t,
		withOnExit(recordHandler("onExit")),
		withOnFailure(recordHandler("onFailure")),
	)

	graph := sc.newGraph(t, failStep("1"))

	result := graph.Schedule(t, status.Error)

	// Both onFailure and onExit should execute
	result.AssertNodeStatus(t, "1", status.NodeError)
	result.AssertNodeStatus(t, "onFailure", status.NodeSuccess)
	result.AssertNodeStatus(t, "onExit", status.NodeSuccess)
}

func TestScheduler_TimeoutDuringRetry(t *testing.T) {
	sc := setupScheduler(t, withTimeout(500*time.Millisecond))

	// Step that will keep retrying until timeout
	graph := sc.newGraph(t,
		newStep("1",
			withCommand("sleep 0.1 && false"),
			withRetryPolicy(10, 50*time.Millisecond), // Many retries
		),
	)

	start := time.Now()
	result := graph.Schedule(t, status.Error)
	elapsed := time.Since(start)

	// Should timeout before completing all retries
	assert.Less(t, elapsed, 5*time.Second)
	result.AssertNodeStatus(t, "1", status.NodeCancel)
}

func TestScheduler_CancelDuringHandlerExecution(t *testing.T) {
	sc := setupScheduler(t,
		withOnExit(newStep("onExit", withScript("echo handler started && sleep 0.1 && echo handler done"))),
	)

	graph := sc.newGraph(t, successStep("1"))

	go func() {
		// Wait for main step to complete and handler to start
		time.Sleep(200 * time.Millisecond)
		sc.Scheduler.Cancel(sc.Context, graph.ExecutionGraph)
	}()

	// Since we cancel during handler execution, the final status depends on timing
	// The graph completes successfully before cancel takes effect
	result := graph.Schedule(t, status.Success)

	result.AssertNodeStatus(t, "1", status.NodeSuccess)
	// Handler should complete successfully
	result.AssertNodeStatus(t, "onExit", status.NodeSuccess)
}

func TestScheduler_RepeatPolicyWithCancel(t *testing.T) {
	sc := setupScheduler(t)

	graph := sc.newGraph(t,
		newStep("1",
			withCommand("echo repeat"),
			withRepeatPolicy(true, 100*time.Millisecond),
		),
	)

	go func() {
		time.Sleep(350 * time.Millisecond)
		sc.Scheduler.Cancel(sc.Context, graph.ExecutionGraph)
	}()

	result := graph.Schedule(t, status.Cancel)
	result.AssertNodeStatus(t, "1", status.NodeCancel)

	node := result.Node(t, "1")
	// Should have repeated at least twice before cancel
	assert.GreaterOrEqual(t, node.State().DoneCount, 2)
}

func TestScheduler_RepeatPolicyWithLimit(t *testing.T) {
	sc := setupScheduler(t)

	// Test repeat with limit
	graph := sc.newGraph(t,
		newStep("1",
			withCommand("echo repeat"),
			withRepeatPolicy(true, 100*time.Millisecond),
			func(step *digraph.Step) {
				step.RepeatPolicy.Limit = 3
			},
		),
	)

	result := graph.Schedule(t, status.Success)
	result.AssertNodeStatus(t, "1", status.NodeSuccess)

	node := result.Node(t, "1")
	// Should have executed exactly 3 times (initial + 2 repeats)
	assert.Equal(t, 3, node.State().DoneCount)
}

func TestScheduler_RepeatPolicyWithLimitAndCondition(t *testing.T) {
	sc := setupScheduler(t)

	counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_limit_%s", uuid.Must(uuid.NewV7()).String()))
	defer func() { _ = os.Remove(counterFile) }()

	// Test repeat with limit and condition
	graph := sc.newGraph(t,
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
			func(step *digraph.Step) {
				step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
				step.RepeatPolicy.Limit = 5
				step.RepeatPolicy.Condition = &digraph.Condition{
					Condition: "`cat " + counterFile + "`",
					Expected:  "10", // Would repeat forever but limit stops at 5
				}
			},
		),
	)

	result := graph.Schedule(t, status.Success)
	result.AssertNodeStatus(t, "1", status.NodeSuccess)

	node := result.Node(t, "1")
	// Should have executed exactly 5 times due to limit
	assert.Equal(t, 5, node.State().DoneCount)

	// Verify counter file shows 5
	content, err := os.ReadFile(counterFile)
	assert.NoError(t, err)
	assert.Equal(t, "5\n", string(content))
}

func TestScheduler_ComplexRetryScenarios(t *testing.T) {
	t.Run("RetryWithSignalTermination", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a script that will be terminated by signal
		graph := sc.newGraph(t,
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
			graph.Signal(syscall.SIGTERM)
		}()

		result := graph.Schedule(t, status.Cancel)
		result.AssertNodeStatus(t, "1", status.NodeCancel)
	})

	t.Run("RetryWithSpecificExitCodes", func(t *testing.T) {
		sc := setupScheduler(t)

		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("retry_codes_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		// Step that returns different exit codes
		graph := sc.newGraph(t,
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
				func(step *digraph.Step) {
					step.RetryPolicy.ExitCodes = []int{42} // Only retry on exit code 42
				},
			),
		)

		result := graph.Schedule(t, status.Error)
		result.AssertNodeStatus(t, "1", status.NodeError)

		node := result.Node(t, "1")
		// Should retry once (first failure with code 42, then fail with code 100)
		assert.Equal(t, 1, node.State().RetryCount)
	})

	// Test cases for behaviors when neither condition nor exitCode are present
	t.Run("RepeatPolicyBooleanTrueRepeatsWhileStepSucceeds", func(t *testing.T) {
		sc := setupScheduler(t)

		// Test repeat: true (boolean mode) - should repeat while step succeeds (no condition/exitCode)
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo boolean true mode"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeWhile // This is what repeat: true becomes
					step.RepeatPolicy.Interval = 20 * time.Millisecond
					step.RepeatPolicy.Limit = 3 // Limit to prevent infinite loop
					// No condition, no exitCode - should repeat while step succeeds
				},
			),
		)

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have executed exactly 3 times (limit reached, step always succeeds)
		assert.Equal(t, 3, node.State().DoneCount)
	})

	t.Run("RepeatPolicyBooleanTrueWithFailureStopsOnFailure", func(t *testing.T) {
		sc := setupScheduler(t)

		// Test repeat: true (boolean mode) with step that eventually fails
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_bool_fail_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		graph := sc.newGraph(t,
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
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeWhile // Boolean true mode
					step.RepeatPolicy.Interval = 20 * time.Millisecond
					// No condition, no exitCode - should stop when step fails
				},
			),
		)

		result := graph.Schedule(t, status.Error)
		result.AssertNodeStatus(t, "1", status.NodeError)

		node := result.Node(t, "1")
		// Should have executed exactly 3 times (2 successes, then 1 failure stops it)
		assert.Equal(t, 3, node.State().DoneCount)
	})

	t.Run("RepeatPolicyUntilModeWithoutConditionRepeatsOnFailure", func(t *testing.T) {
		sc := setupScheduler(t)

		// Test explicit until mode without condition/exitCode (repeats until step succeeds)
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_until_none_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		graph := sc.newGraph(t,
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
				withContinueOn(digraph.ContinueOn{
					ExitCode:    []int{1},
					Failure:     true,
					MarkSuccess: true,
				}),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
					step.RepeatPolicy.Interval = 20 * time.Millisecond
					// No condition, no exitCode - should repeat until step succeeds
				},
			),
		)

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have executed exactly 3 times (fails twice, then succeeds)
		assert.Equal(t, 3, node.State().DoneCount)
	})

	t.Run("RepeatPolicyWhileWithConditionRepeatsWhileConditionSucceeds", func(t *testing.T) {
		sc := setupScheduler(t)

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
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("cat "+counterFile+" || echo notfound"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeWhile
					step.RepeatPolicy.Condition = &digraph.Condition{
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

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have run at least twice (first: file not found, second: file created)
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyWhileWithConditionAndExpectedRepeatsWhileMatches", func(t *testing.T) {
		sc := setupScheduler(t)

		// Test explicit while mode with condition and expected value
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_while_exp_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		// Write initial value
		err := os.WriteFile(counterFile, []byte("continue"), 0600)
		require.NoError(t, err)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo while with expected"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeWhile
					step.RepeatPolicy.Condition = &digraph.Condition{
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

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have executed at least 2 times (while expected matches)
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyUntilWithConditionRepeatsUntilConditionSucceeds", func(t *testing.T) {
		sc := setupScheduler(t)

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
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("cat "+counterFile+" || echo notfound"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
					step.RepeatPolicy.Condition = &digraph.Condition{
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

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have run at least twice (first: file not found, second: file created)
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyUntilWithConditionAndExpectedRepeatsUntilMatches", func(t *testing.T) {
		sc := setupScheduler(t)

		// Test explicit until mode with condition and expected value
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_until_exp_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		// Write initial value
		err := os.WriteFile(counterFile, []byte("waiting"), 0600)
		require.NoError(t, err)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo until with expected"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
					step.RepeatPolicy.Condition = &digraph.Condition{
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

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have executed at least 2 times (until expected matches)
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicyUntilWithExitCodeRepeatsUntilExitCodeMatches", func(t *testing.T) {
		sc := setupScheduler(t)

		// Test explicit until mode with exit codes
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_until_exit_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		graph := sc.newGraph(t,
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
				withContinueOn(digraph.ContinueOn{
					ExitCode:    []int{1},
					Failure:     true,
					MarkSuccess: true,
				}),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
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

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have executed at least 3 times (until exit code 42)
		assert.GreaterOrEqual(t, node.State().DoneCount, 3)
	})

	t.Run("RepeatPolicyLimit", func(t *testing.T) {
		sc := setupScheduler(t)
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo limit"),
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
					step.RepeatPolicy.Condition = &digraph.Condition{
						Condition: "false", // Will never be true
					}
					step.RepeatPolicy.Limit = 3
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have executed exactly 3 times (limit reached)
		assert.Equal(t, 3, node.State().DoneCount)
	})

	t.Run("RepeatPolicyOutputVariablesReloadedBeforeConditionEval", func(t *testing.T) {
		sc := setupScheduler(t)

		// Test that output variables are reloaded before evaluating repeat condition
		// Use a file-based counter to track iterations properly
		counterFile := filepath.Join(os.TempDir(), fmt.Sprintf("repeat_output_var_%s", uuid.Must(uuid.NewV7()).String()))
		defer func() { _ = os.Remove(counterFile) }()

		graph := sc.newGraph(t,
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
				func(step *digraph.Step) {
					step.RepeatPolicy.RepeatMode = digraph.RepeatModeUntil
					step.RepeatPolicy.Condition = &digraph.Condition{
						Condition: "$COUNTER",
						Expected:  "3",
					}
					step.RepeatPolicy.Interval = 20 * time.Millisecond
				},
			),
		)

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "1", status.NodeSuccess)

		node := result.Node(t, "1")
		// Should have executed exactly 3 times (until COUNTER equals 3)
		assert.Equal(t, 3, node.State().DoneCount)

		// Verify final output variable value
		output, ok := node.NodeData().State.OutputVariables.Load("COUNTER")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "COUNTER=3", output)
	})
}

func TestScheduler_StepIDVariableExpansion(t *testing.T) {
	sc := setupScheduler(t)

	// Test step ID usage in environment setup
	graph := sc.newGraph(t,
		newStep("step1",
			withCommand("echo output1"),
			withOutput("OUT1"),
			func(step *digraph.Step) {
				step.ID = "s1"
			},
		),
		newStep("step2",
			withCommand("echo output2"),
			withOutput("OUT2"),
			func(step *digraph.Step) {
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

	result := graph.Schedule(t, status.Success)

	result.AssertNodeStatus(t, "step1", status.NodeSuccess)
	result.AssertNodeStatus(t, "step2", status.NodeSuccess)
	result.AssertNodeStatus(t, "step3", status.NodeSuccess)

	node := result.Node(t, "step3")
	output, ok := node.NodeData().State.OutputVariables.Load("COMBINED")
	require.True(t, ok)
	assert.Equal(t, "COMBINED=output1 output2", output)
}

func TestScheduler_UnexpectedFinalStatus(t *testing.T) {
	// This is a bit tricky to test as it requires the scheduler to be in an
	// unexpected state at the end. We'll simulate this by creating a custom
	// scenario that might trigger this edge case.
	sc := setupScheduler(t)

	// Create a graph with a step that might leave the scheduler in an unexpected state
	graph := sc.newGraph(t,
		newStep("1", withCommand("echo test")),
	)

	// Schedule normally
	result := graph.Schedule(t, status.Success)
	result.AssertNodeStatus(t, "1", status.NodeSuccess)

	// The warning log about unexpected final status would be logged internally
	// but we can't easily test for it without mock logging
}

func TestScheduler_RetryPolicyDefaults(t *testing.T) {
	sc := setupScheduler(t)

	// Test retry with unhandled error type (not exec.ExitError)
	graph := sc.newGraph(t,
		newStep("1",
			withScript(`
				# This will cause a different type of error
				echo "Test error" >&2
				exit 1
			`),
			withRetryPolicy(1, 20*time.Millisecond),
		),
	)

	result := graph.Schedule(t, status.Error)
	result.AssertNodeStatus(t, "1", status.NodeError)

	node := result.Node(t, "1")
	// Should have retried once
	assert.Equal(t, 1, node.State().RetryCount)
}

func TestScheduler_StepRetryExecution(t *testing.T) {
	t.Run("RetrySuccessfulStep", func(t *testing.T) {
		sc := setupScheduler(t)

		// A -> B -> C, all successful
		dag := &digraph.DAG{
			Steps: []digraph.Step{
				{Name: "A", Command: "echo A"},
				{Name: "B", Command: "echo B", Depends: []string{"A"}},
				{Name: "C", Command: "echo C", Depends: []string{"B"}},
			},
		}

		// Initial run - all successful
		graph := sc.newGraph(t,
			successStep("A"),
			successStep("B", "A"),
			successStep("C", "B"),
		)
		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "A", status.NodeSuccess)
		result.AssertNodeStatus(t, "B", status.NodeSuccess)
		result.AssertNodeStatus(t, "C", status.NodeSuccess)

		// Create nodes with their current states
		nodes := []*scheduler.Node{
			scheduler.NodeWithData(scheduler.NodeData{
				Step:  dag.Steps[0],
				State: scheduler.NodeState{Status: status.NodeSuccess},
			}),
			scheduler.NodeWithData(scheduler.NodeData{
				Step:  dag.Steps[1],
				State: scheduler.NodeState{Status: status.NodeSuccess},
			}),
			scheduler.NodeWithData(scheduler.NodeData{
				Step:  dag.Steps[2],
				State: scheduler.NodeState{Status: status.NodeSuccess},
			}),
		}

		// Retry step B
		retryGraph, err := scheduler.CreateStepRetryGraph(context.Background(), dag, nodes, "B")
		require.NoError(t, err)

		// Schedule the retry
		retryResult := graphHelper{testHelper: sc, ExecutionGraph: retryGraph}.Schedule(t, status.Success)

		// A and C should remain unchanged, only B should be re-executed
		retryResult.AssertNodeStatus(t, "A", status.NodeSuccess)
		retryResult.AssertNodeStatus(t, "B", status.NodeSuccess)
		retryResult.AssertNodeStatus(t, "C", status.NodeSuccess)
	})
}

// TestScheduler_StepIDAccess tests that step ID variables are expanded correctly
func TestScheduler_StepIDAccess(t *testing.T) {
	t.Run("StepReferenceInCommand", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a DAG where step2 references step1's output
		graph := sc.newGraph(t,
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

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "step1", status.NodeSuccess)
		result.AssertNodeStatus(t, "step2", status.NodeSuccess)

		// Step2 should have access to step1's stdout path
		node2 := result.Node(t, "step2")
		stdoutFile := node2.GetStdout()
		stdoutContent, err := os.ReadFile(stdoutFile)
		require.NoError(t, err)
		assert.Contains(t, string(stdoutContent), "Step 1 stdout:")
	})
	t.Run("StepWithoutID", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a DAG where some steps don't have IDs
		graph := sc.newGraph(t,
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

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "step1", status.NodeSuccess)
		result.AssertNodeStatus(t, "step2", status.NodeSuccess)
		result.AssertNodeStatus(t, "step3", status.NodeSuccess)

		node3 := result.Node(t, "step3")
		stdoutFile := node3.GetStdout()
		stdoutContent, err := os.ReadFile(stdoutFile)
		require.NoError(t, err)
		// Should contain the path to step2's stdout file
		assert.Contains(t, string(stdoutContent), "Can reference:")
		assert.Contains(t, string(stdoutContent), ".out")
	})

	t.Run("StepExitCodeReference", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a step that checks another step's exit code
		graph := sc.newGraph(t,
			newStep("check",
				withID("checker"),
				withCommand("exit 42"),
				withContinueOn(digraph.ContinueOn{
					ExitCode: []int{42},
				}),
			),
			newStep("verify",
				withID("verifier"),
				withDepends("check"),
				withCommand("echo 'Checker exit code: ${checker.exit_code}'"),
			),
		)

		result := graph.Schedule(t, status.PartialSuccess)
		result.AssertNodeStatus(t, "check", status.NodeError)
		result.AssertNodeStatus(t, "verify", status.NodeSuccess)

		nodeVerify := result.Node(t, "verify")
		stdoutFile := nodeVerify.GetStdout()
		stdoutContent, err := os.ReadFile(stdoutFile)
		require.NoError(t, err)
		assert.Contains(t, string(stdoutContent), "Checker exit code: 42")
	})
}

// TestScheduler_EventHandlerStepIDAccess tests that step ID references work in event handlers
func TestScheduler_EventHandlerStepIDAccess(t *testing.T) {
	t.Run("OnSuccessHandlerWithStepReferences", func(t *testing.T) {
		sc := setupScheduler(t,
			withOnSuccess(digraph.Step{
				Name:    "success_handler",
				ID:      "on_success",
				Command: "echo 'Main output: ${main.stdout}, Worker result: ${worker.exit_code}'",
			}),
		)

		graph := sc.newGraph(t,
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

		result := graph.Schedule(t, status.Success)

		// All steps should succeed
		result.AssertNodeStatus(t, "main_step", status.NodeSuccess)
		result.AssertNodeStatus(t, "worker_step", status.NodeSuccess)

		// The handler should have executed
		result.AssertNodeStatus(t, "success_handler", status.NodeSuccess)

		// Get the handler node
		handlerNode := result.Node(t, "success_handler")
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
		sc := setupScheduler(t,
			withOnFailure(digraph.Step{
				Name:    "failure_handler",
				ID:      "on_fail",
				Command: "echo 'Failed step stderr: ${failing.stderr}, exit code: ${failing.exit_code}'",
			}),
		)

		graph := sc.newGraph(t,
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

		result := graph.Schedule(t, status.Error)

		// Check step statuses
		result.AssertNodeStatus(t, "setup", status.NodeSuccess)
		result.AssertNodeStatus(t, "failing_step", status.NodeError)

		// The failure handler should have executed
		result.AssertNodeStatus(t, "failure_handler", status.NodeSuccess)

		// Get the handler node
		handlerNode := result.Node(t, "failure_handler")
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
		sc := setupScheduler(t,
			withOnExit(digraph.Step{
				Name:    "exit_handler",
				ID:      "on_exit",
				Command: "echo 'Step1: ${step1.stdout}, Step2: ${step2.exit_code}, Step3: ${step3.stderr}'",
			}),
		)

		graph := sc.newGraph(t,
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

		result := graph.Schedule(t, status.Success)

		// All main steps should succeed
		result.AssertNodeStatus(t, "first", status.NodeSuccess)
		result.AssertNodeStatus(t, "second", status.NodeSuccess)
		result.AssertNodeStatus(t, "third", status.NodeSuccess)

		// The exit handler should have executed
		result.AssertNodeStatus(t, "exit_handler", status.NodeSuccess)

		// Get the handler node
		handlerNode := result.Node(t, "exit_handler")
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
		sc := setupScheduler(t,
			withOnExit(digraph.Step{
				Name: "exit_handler_no_id",
				// No ID field set
				Command: "echo 'Handler executed'",
			}),
		)

		graph := sc.newGraph(t,
			newStep("main",
				withID("main_step"),
				withCommand("echo 'Main step'"),
			),
		)

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "main", status.NodeSuccess)

		// Handler should execute
		result.AssertNodeStatus(t, "exit_handler_no_id", status.NodeSuccess)

		// Get the handler node to verify it has no ID
		handlerNode := result.Node(t, "exit_handler_no_id")
		assert.Empty(t, handlerNode.Step().ID, "Handler should have no ID")
	})

	t.Run("HandlersCanOnlyReferenceMainSteps", func(t *testing.T) {
		// Test that handlers can reference main steps but not other handlers
		// This is because handlers execute after all main steps are complete
		sc := setupScheduler(t,
			withOnSuccess(digraph.Step{
				Name:    "first_handler",
				ID:      "handler1",
				Command: "echo 'SUCCESS: Main returned ${main.exit_code}'",
			}),
			withOnExit(digraph.Step{
				Name:    "final_handler",
				ID:      "handler2",
				Command: "echo 'FINAL: Main step output at ${main.stdout}, trying handler ref: ${handler1.stdout}'",
			}),
		)

		graph := sc.newGraph(t,
			newStep("main",
				withID("main"),
				withCommand("echo 'Processing' && exit 0"),
			),
		)

		result := graph.Schedule(t, status.Success)
		result.AssertNodeStatus(t, "main", status.NodeSuccess)

		// Both handlers should have executed
		result.AssertNodeStatus(t, "first_handler", status.NodeSuccess)
		result.AssertNodeStatus(t, "final_handler", status.NodeSuccess)

		// Get the handler nodes
		successHandler := result.Node(t, "first_handler")
		exitHandler := result.Node(t, "final_handler")

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

func TestSchedulerPartialSuccess(t *testing.T) {
	t.Run("NodeStatusPartialSuccess", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a graph where:
		// - step1 succeeds
		// - step2 fails but has continueOn.failure = true
		// - step3 depends on step2 and succeeds
		graph := sc.newGraph(t,
			successStep("step1"),
			newStep("step2",
				withDepends("step1"),
				withCommand("false"), // This will fail
				withContinueOn(digraph.ContinueOn{
					Failure: true,
				}),
			),
			successStep("step3", "step2"),
		)

		// The overall DAG should complete with partial success
		result := graph.Schedule(t, status.PartialSuccess)

		// Verify individual node statuses
		result.AssertNodeStatus(t, "step1", status.NodeSuccess)
		result.AssertNodeStatus(t, "step2", status.NodeError)
		result.AssertNodeStatus(t, "step3", status.NodeSuccess)
	})

	t.Run("NodeStatusPartialSuccessWithMarkSuccess", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a graph where:
		// - step1 succeeds
		// - step2 fails but has continueOn.failure = true and markSuccess = true
		// - step3 depends on step2 and succeeds
		graph := sc.newGraph(t,
			successStep("step1"),
			newStep("step2",
				withDepends("step1"),
				withCommand("false"), // This will fail
				withContinueOn(digraph.ContinueOn{
					Failure:     true,
					MarkSuccess: true,
				}),
			),
			successStep("step3", "step2"),
		)

		// When markSuccess is true, the overall DAG should complete with success
		result := graph.Schedule(t, status.Success)

		// Verify individual node statuses
		result.AssertNodeStatus(t, "step1", status.NodeSuccess)
		result.AssertNodeStatus(t, "step2", status.NodeSuccess) // Marked as success
		result.AssertNodeStatus(t, "step3", status.NodeSuccess)
	})

	t.Run("MultipleFailuresWithContinueOn", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a graph where multiple steps fail but have continueOn
		graph := sc.newGraph(t,
			newStep("step1",
				withCommand("false"),
				withContinueOn(digraph.ContinueOn{
					Failure: true,
				}),
			),
			newStep("step2",
				withDepends("step1"),
				withCommand("false"),
				withContinueOn(digraph.ContinueOn{
					Failure: true,
				}),
			),
			successStep("step3", "step2"),
		)

		// The overall DAG should complete with partial success
		result := graph.Schedule(t, status.PartialSuccess)

		// Verify individual node statuses
		result.AssertNodeStatus(t, "step1", status.NodeError)
		result.AssertNodeStatus(t, "step2", status.NodeError)
		result.AssertNodeStatus(t, "step3", status.NodeSuccess)
	})

	t.Run("NoSuccessfulStepsWithContinueOn", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a graph where all steps fail but have continueOn
		// This should still be an error, not partial success,
		// because partial success requires at least one successful step
		graph := sc.newGraph(t,
			newStep("step1",
				withCommand("false"),
				withContinueOn(digraph.ContinueOn{
					Failure: true,
				}),
			),
			newStep("step2",
				withDepends("step1"),
				withCommand("false"),
				withContinueOn(digraph.ContinueOn{
					Failure: true,
				}),
			),
		)

		// The overall DAG should complete with error since no steps succeeded
		result := graph.Schedule(t, status.Error)

		// Verify individual node statuses
		result.AssertNodeStatus(t, "step1", status.NodeError)
		result.AssertNodeStatus(t, "step2", status.NodeError)
	})

	t.Run("FailureWithoutContinueOn", func(t *testing.T) {
		sc := setupScheduler(t)

		// Create a graph where a step fails without continueOn
		// This should result in an error status, not partial success
		graph := sc.newGraph(t,
			successStep("step1"),
			failStep("step2", "step1"),    // This will fail without continueOn
			successStep("step3", "step1"), // This depends on step1, not step2
		)

		// The overall DAG should complete with error
		result := graph.Schedule(t, status.Error)

		// Verify individual node statuses
		result.AssertNodeStatus(t, "step1", status.NodeSuccess)
		result.AssertNodeStatus(t, "step2", status.NodeError)
		result.AssertNodeStatus(t, "step3", status.NodeSuccess)
	})
}
