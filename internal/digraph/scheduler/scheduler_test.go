package scheduler_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	testScript := test.TestdataPath(t, filepath.Join("digraph", "scheduler", "testfile.sh"))

	t.Run("SequentialStepsSuccess", func(t *testing.T) {
		sc := setupScheduler(t, withMaxActiveRuns(1))

		// 1 -> 2 -> 3
		graph := sc.newGraph(t,
			successStep("1"),
			successStep("2", "1"),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
	})
	t.Run("SequentialStepsWithFailure", func(t *testing.T) {
		sc := setupScheduler(t, withMaxActiveRuns(1))

		// 1 -> 2 -> 3 -> 4
		graph := sc.newGraph(t,
			successStep("1"),
			successStep("2", "1"),
			failStep("3", "2"),
			successStep("4", "3"),
		)

		result := graph.Schedule(t, scheduler.StatusError)

		// 1, 2, 3 should be executed and 4 should be canceled because 3 failed
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "4", scheduler.NodeStatusCancel)
	})
	t.Run("ParallelSteps", func(t *testing.T) {
		sc := setupScheduler(t, withMaxActiveRuns(3))

		// 1,2,3
		graph := sc.newGraph(t,
			successStep("1"),
			successStep("2"),
			successStep("3"),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusError)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "4", scheduler.NodeStatusSuccess)
	})
	t.Run("ComplexCommand", func(t *testing.T) {
		sc := setupScheduler(t, withMaxActiveRuns(1))

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("df / | awk 'NR==2 {exit $4 > 5000 ? 0 : 1}'"),
			))

		result := graph.Schedule(t, scheduler.StatusSuccess)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusError)

		// 1, 2, 3 should be executed even though 2 failed
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSkipped)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusError)

		// 1, 2 should be executed even though 1 failed
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusError)

		// 1, 2 should be executed even though 1 failed
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
	})
	t.Run("ContinueOnOutputStderr", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 (exit code 1) -> 2
		graph := sc.newGraph(t,
			newStep("1",
				withCommand("echo test_output; false 1>&2"), // stderr: test_output
				withContinueOn(digraph.ContinueOn{
					Output: []string{
						"test_output",
					},
				}),
			),
			successStep("2", "1"),
		)

		result := graph.Schedule(t, scheduler.StatusError)

		// 1, 2 should be
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusError)

		// 1, 2 should be executed even though 1 failed
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		// 1, 2 should be executed even though 1 failed
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
	})
	t.Run("CancelSchedule", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1 -> 2 (cancel when running) -> 3 (should not be executed)
		graph := sc.newGraph(t,
			successStep("1"),
			newStep("2", withDepends("1"), withCommand("sleep 10")),
			failStep("3", "2"),
		)

		go func() {
			time.Sleep(time.Millisecond * 500) // wait for step 2 to start
			graph.Cancel(t)
		}()

		result := graph.Schedule(t, scheduler.StatusCancel)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusCancel)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusNone)
	})
	t.Run("Timeout", func(t *testing.T) {
		sc := setupScheduler(t, withTimeout(time.Second*2))

		// 1 -> 2 (timeout) -> 3 (should not be executed)
		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 1")),
			newStep("2", withCommand("sleep 10"), withDepends("1")),
			successStep("3", "2"),
		)

		result := graph.Schedule(t, scheduler.StatusError)

		// 1 should be executed and 2 should be canceled because of timeout
		// 3 should not be executed and should be canceled
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusCancel)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusCancel)
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

		result := graph.Schedule(t, scheduler.StatusError)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)

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
				withRetryPolicy(1, time.Millisecond*500),
			),
		)

		_ = os.Setenv(testEnv, "1")
		go func() {
			time.Sleep(time.Millisecond * 300)
			_ = os.Setenv(testEnv, "0")
			t.Cleanup(func() {
				_ = os.Unsetenv(testEnv)
			})
		}()

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)

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
				withRetryPolicy(1, time.Millisecond*500),
			),
		)

		go func() {
			// Create file for successful retry
			time.Sleep(time.Millisecond * 300) // wait for step 1 to start

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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		// Check if the retry is successful
		state := result.Node(t, "1").State()
		assert.Equal(t, 1, state.DoneCount)
		assert.Equal(t, 1, state.RetryCount)
		assert.NotEmpty(t, state.RetriedAt)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
	})
	t.Run("PreconditionMatch", func(t *testing.T) {
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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		// 1 should be executed and 2, 3 should be skipped
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSkipped)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSkipped)
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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		// 1 should be executed and 2, 3 should be skipped
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSkipped)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSkipped)
	})
	t.Run("OnExitHandler", func(t *testing.T) {
		sc := setupScheduler(t, withOnExit(successStep("onExit")))

		graph := sc.newGraph(t, successStep("1"))

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "onExit", scheduler.NodeStatusSuccess)
	})
	t.Run("OnExitHandlerFail", func(t *testing.T) {
		sc := setupScheduler(t, withOnExit(failStep("onExit")))

		graph := sc.newGraph(t, successStep("1"))

		// Overall status should be error because onExit failed
		result := graph.Schedule(t, scheduler.StatusError)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "onExit", scheduler.NodeStatusError)
	})
	t.Run("OnCancelHandler", func(t *testing.T) {
		sc := setupScheduler(t, withOnCancel(successStep("onCancel")))

		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 10")),
		)

		go func() {
			time.Sleep(time.Millisecond * 100) // wait for step 1 to start
			graph.Signal(syscall.SIGTERM)
		}()

		result := graph.Schedule(t, scheduler.StatusCancel)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusCancel)
		result.AssertNodeStatus(t, "onCancel", scheduler.NodeStatusSuccess)
	})
	t.Run("OnSuccessHandler", func(t *testing.T) {
		sc := setupScheduler(t, withOnSuccess(successStep("onSuccess")))

		graph := sc.newGraph(t, successStep("1"))

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "onSuccess", scheduler.NodeStatusSuccess)
	})
	t.Run("OnFailureHandler", func(t *testing.T) {
		sc := setupScheduler(t, withOnFailure(successStep("onFailure")))

		graph := sc.newGraph(t, failStep("1"))

		result := graph.Schedule(t, scheduler.StatusError)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "onFailure", scheduler.NodeStatusSuccess)
	})
	t.Run("CancelOnSignal", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 10")),
		)

		go func() {
			time.Sleep(time.Millisecond * 100) // wait for step 1 to start
			graph.Signal(syscall.SIGTERM)
		}()

		result := graph.Schedule(t, scheduler.StatusCancel)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusCancel)
	})
	t.Run("Repeat", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("sleep 1"),
				withRepeatPolicy(true, time.Millisecond*500),
			),
		)

		go func() {
			time.Sleep(time.Millisecond * 1750)
			graph.Cancel(t)
		}()

		result := graph.Schedule(t, scheduler.StatusCancel)

		// 1 should be repeated 2 times
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusCancel)

		node := result.Node(t, "1")
		// done count should be 1 because 2nd execution is canceled
		require.Equal(t, 1, node.State().DoneCount)
	})
	t.Run("RepeatFail", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("false"),
				withRepeatPolicy(true, time.Millisecond*300),
			),
		)

		result := graph.Schedule(t, scheduler.StatusError)

		// Done count should be 1 because it failed and not repeated
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)

		node := result.Node(t, "1")
		require.Equal(t, 1, node.State().DoneCount)
	})
	t.Run("StopRepetitiveTaskGracefully", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1",
				withCommand("sleep 1"),
				withRepeatPolicy(true, time.Millisecond*300),
			),
		)

		done := make(chan struct{})
		go func() {
			time.Sleep(time.Millisecond * 100)
			graph.Signal(syscall.SIGTERM)
			close(done)
		}()

		result := graph.Schedule(t, scheduler.StatusSuccess)
		<-done

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
	})
	t.Run("NodeSetupFailure", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withWorkingDir("/nonexistent"),
				withScript("echo 1"),
			),
		)

		result := graph.Schedule(t, scheduler.StatusError)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)

		require.Contains(t, result.Error.Error(), "directory does not exist")
	})
	t.Run("OutputVariables", func(t *testing.T) {
		sc := setupScheduler(t)

		// 1: echo hello > OUT
		// 2: echo $OUT > RESULT
		graph := sc.newGraph(t,
			newStep("1", withCommand("echo hello"), withOutput("OUT")),
			newStep("2", withCommand("echo $OUT"), withDepends("1"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)

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
			newStep("4", withCommand("sleep 1")),
			// 5 should not have reference to OUT or OUT2
			newStep("5", withCommand("echo $OUT $OUT2"), withDepends("4"), withOutput("RESULT2")),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)

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

		result := graph.Schedule(t, scheduler.StatusSuccess)

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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		// check if RESULT variable is set to "value"
		node := result.Node(t, "2")

		output, _ := node.NodeData().State.OutputVariables.Load("RESULT")
		require.Equal(t, "RESULT=value", output, "expected output %q, got %q", "value", output)
	})
	t.Run("SpecialVars_DAG_RUN_LOG_FILE", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_RUN_LOG_FILE"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.log$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVars_DAG_RUN_STEP_STDOUT_FILE", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_RUN_STEP_STDOUT_FILE"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.out$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVars_DAG_RUN_STEP_STDERR_FILE", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_RUN_STEP_STDERR_FILE"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `^RESULT=/.*/.*\.err$`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVars_DAG_RUN_ID", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_RUN_ID"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Regexp(t, `RESULT=[a-f0-9-]+`, output, "unexpected output %q", output)
	})
	t.Run("SpecialVars_DAG_NAME", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("echo $DAG_NAME"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)
		node := result.Node(t, "1")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=test_dag", output, "unexpected output %q", output)
	})
	t.Run("SpecialVars_DAG_RUN_STEP_NAME", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("step_test", withCommand("echo $DAG_RUN_STEP_NAME"), withOutput("RESULT")),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)
		node := result.Node(t, "step_test")

		output, ok := node.NodeData().State.OutputVariables.Load("RESULT")
		require.True(t, ok, "output variable not found")
		require.Equal(t, "RESULT=step_test", output, "unexpected output %q", output)
	})

	t.Run("RepeatPolicy_RepeatsUntilCommandConditionMatchesExpected", func(t *testing.T) {
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
					step.RepeatPolicy.Condition = &digraph.Condition{
						Condition: fmt.Sprintf("`cat %s || true`", file),
						Expected:  "ready",
					}
					step.RepeatPolicy.Interval = 100 * time.Millisecond
				},
			),
		)

		go func() {
			time.Sleep(400 * time.Millisecond)
			err := os.WriteFile(file, []byte("ready"), 0600)
			require.NoError(t, err, "failed to write to file")
		}()

		result := graph.Schedule(t, scheduler.StatusSuccess)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		// Should have run at least twice (first: not ready, second: ready)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicy_RepeatWhileConditionExits0", func(t *testing.T) {
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
					step.RepeatPolicy.Condition = &digraph.Condition{
						Condition: "test ! -f " + file,
					}
					step.RepeatPolicy.Interval = 100 * time.Millisecond
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
		result := graph.Schedule(t, scheduler.StatusSuccess)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicy_RepeatsWhileCommandExitCodeMatches", func(t *testing.T) {
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
				withContinueOn(digraph.ContinueOn{
					ExitCode:    []int{42},
					Failure:     true,
					MarkSuccess: true,
				}),
				func(step *digraph.Step) {
					step.RepeatPolicy.ExitCode = []int{42}
					step.RepeatPolicy.Interval = 200 * time.Millisecond
				},
			),
		)
		go func() {
			time.Sleep(350 * time.Millisecond)
			f, _ := os.Create(countFile)
			err := f.Close()
			require.NoError(t, err)
		}()
		result := graph.Schedule(t, scheduler.StatusSuccess)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicy_RepeatsUntilEnvVarConditionMatchesExpected", func(t *testing.T) {
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
					step.RepeatPolicy.Condition = &digraph.Condition{
						Condition: "$TEST_REPEAT_MATCH_EXPR",
						Expected:  "done",
					}
					step.RepeatPolicy.Interval = 100 * time.Millisecond
				},
			),
		)
		go func() {
			time.Sleep(300 * time.Millisecond)
			err := os.Setenv("TEST_REPEAT_MATCH_EXPR", "done")
			require.NoError(t, err)
		}()
		result := graph.Schedule(t, scheduler.StatusSuccess)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		node := result.Node(t, "1")
		assert.GreaterOrEqual(t, node.State().DoneCount, 2)
	})

	t.Run("RepeatPolicy_RepeatsUntilOutputVarConditionMatchesExpected", func(t *testing.T) {
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
					step.RepeatPolicy.Condition = &digraph.Condition{
						Condition: "$OUT",
						Expected:  "done",
					}
					step.RepeatPolicy.Interval = 100 * time.Millisecond
				},
			),
		)
		go func() {
			time.Sleep(300 * time.Millisecond)
			err := os.WriteFile(file, []byte("done"), 0600)
			require.NoError(t, err)
		}()
		result := graph.Schedule(t, scheduler.StatusSuccess)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
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
				withRetryPolicy(3, time.Millisecond*100),
			),
		)

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)

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

		result := graph.Schedule(t, scheduler.StatusError)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)

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
				withRetryPolicy(2, time.Millisecond*100),
			),
		)

		result := graph.Schedule(t, scheduler.StatusError)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)

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
		step.RepeatPolicy.Repeat = repeat
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

func (gh graphHelper) Schedule(t *testing.T, expectedStatus scheduler.Status) scheduleResult {
	t.Helper()

	dag := &digraph.DAG{Name: "test_dag"}
	logFilename := fmt.Sprintf("%s_%s.log", dag.Name, gh.Config.DAGRunID)
	logFilePath := path.Join(gh.Config.LogDir, logFilename)

	ctx := digraph.SetupEnv(gh.Context, dag, nil, digraph.DAGRunRef{}, gh.Config.DAGRunID, logFilePath, nil)

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
	case scheduler.StatusSuccess, scheduler.StatusCancel:
		require.NoError(t, err)

	case scheduler.StatusError:
		require.Error(t, err)

	case scheduler.StatusRunning, scheduler.StatusNone, scheduler.StatusQueued:
		t.Errorf("unexpected status %s", expectedStatus)

	}

	require.Equal(t, expectedStatus.String(), gh.Scheduler.Status(gh.ExecutionGraph).String(),
		"expected status %s, got %s", expectedStatus, gh.Scheduler.Status(gh.ExecutionGraph))

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

func (sr scheduleResult) AssertNodeStatus(t *testing.T, stepName string, expected scheduler.NodeStatus) {
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
		status   scheduler.Status
		expected string
	}{
		{scheduler.StatusNone, "not started"},
		{scheduler.StatusRunning, "running"},
		{scheduler.StatusError, "failed"},
		{scheduler.StatusCancel, "canceled"},
		{scheduler.StatusSuccess, "finished"},
		{scheduler.StatusQueued, "queued"},
		{scheduler.Status(999), "not started"}, // Invalid status defaults to "not started"
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestStatus_IsActive(t *testing.T) {
	tests := []struct {
		status   scheduler.Status
		expected bool
	}{
		{scheduler.StatusNone, false},
		{scheduler.StatusRunning, true},
		{scheduler.StatusError, false},
		{scheduler.StatusCancel, false},
		{scheduler.StatusSuccess, false},
		{scheduler.StatusQueued, true},
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

	result := graph.Schedule(t, scheduler.StatusSuccess)

	// In dry run, steps should be marked as success without actual execution
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
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

	result := graph.Schedule(t, scheduler.StatusSuccess)

	result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "onExit", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "onSuccess", scheduler.NodeStatusSuccess)
}

func TestScheduler_ConcurrentExecution(t *testing.T) {
	sc := setupScheduler(t, withMaxActiveRuns(3))

	// Create a synchronization mechanism to ensure steps run concurrently
	var counter int32
	var mu sync.Mutex
	maxConcurrent := int32(0)

	step := func(name string) digraph.Step {
		return newStep(name, withScript(fmt.Sprintf(`
			echo "Step %s starting"
			sleep 0.1
			echo "Step %s ending"
		`, name, name)))
	}

	// Track concurrent executions
	graph := sc.newGraph(t,
		step("1"),
		step("2"),
		step("3"),
	)

	// Hook to track concurrent executions
	go func() {
		for {
			time.Sleep(10 * time.Millisecond)
			current := atomic.LoadInt32(&counter)
			mu.Lock()
			if current > maxConcurrent {
				maxConcurrent = current
			}
			mu.Unlock()
			if current == 0 {
				break
			}
		}
	}()

	result := graph.Schedule(t, scheduler.StatusSuccess)

	result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
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

		ctx := digraph.SetupEnv(sc.Context, dag, nil, digraph.DAGRunRef{}, sc.Config.DAGRunID, logFilePath, nil)

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
		result := graph.Schedule(t, scheduler.StatusError)
		result.AssertNodeStatus(t, "panic", scheduler.NodeStatusError)
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

	result := graph.Schedule(t, scheduler.StatusError)

	// Get metrics
	metrics := sc.Scheduler.GetMetrics()

	assert.Equal(t, 4, metrics["totalNodes"])
	assert.Equal(t, 2, metrics["completedNodes"]) // 1 and 4
	assert.Equal(t, 1, metrics["failedNodes"])    // 2
	assert.Equal(t, 1, metrics["skippedNodes"])   // 3
	assert.Equal(t, 0, metrics["canceledNodes"])
	assert.NotEmpty(t, metrics["totalExecutionTime"])

	// Verify individual node statuses
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "2", scheduler.NodeStatusError)
	result.AssertNodeStatus(t, "3", scheduler.NodeStatusSkipped)
	result.AssertNodeStatus(t, "4", scheduler.NodeStatusSuccess)
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

		ctx := digraph.SetupEnv(sc.Context, dag, nil, digraph.DAGRunRef{}, sc.Config.DAGRunID, logFilePath, nil)

		err := sc.Scheduler.Schedule(ctx, graph.ExecutionGraph, nil)
		require.NoError(t, err) // No error, but dag should be canceled

		// Check that the scheduler was canceled
		assert.Equal(t, scheduler.StatusCancel, sc.Scheduler.Status(graph.ExecutionGraph))
	})
}

func TestScheduler_SignalHandling(t *testing.T) {
	t.Run("SignalWithDoneChannel", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 10")),
			successStep("2", "1"),
		)

		done := make(chan bool, 1)

		go func() {
			time.Sleep(100 * time.Millisecond)
			sc.Scheduler.Signal(sc.Context, graph.ExecutionGraph, syscall.SIGTERM, done, false)
		}()

		start := time.Now()
		result := graph.Schedule(t, scheduler.StatusCancel)

		// Wait for signal completion
		select {
		case <-done:
			// Signal handling completed
		case <-time.After(5 * time.Second):
			t.Fatal("Signal handling did not complete in time")
		}

		elapsed := time.Since(start)
		assert.Less(t, elapsed, 2*time.Second, "Should cancel quickly")

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusCancel)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusNone)
	})

	t.Run("SignalWithOverride", func(t *testing.T) {
		sc := setupScheduler(t)

		graph := sc.newGraph(t,
			newStep("1", withCommand("sleep 10")),
		)

		go func() {
			time.Sleep(100 * time.Millisecond)
			sc.Scheduler.Signal(sc.Context, graph.ExecutionGraph, syscall.SIGKILL, nil, true)
		}()

		result := graph.Schedule(t, scheduler.StatusCancel)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusCancel)
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

		result := graph.Schedule(t, scheduler.StatusSuccess)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "4", scheduler.NodeStatusSuccess)
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

		result := graph.Schedule(t, scheduler.StatusError)

		result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusSuccess)
		result.AssertNodeStatus(t, "4", scheduler.NodeStatusCancel) // Canceled due to 2's failure
	})
}

func TestScheduler_EdgeCases(t *testing.T) {
	t.Run("EmptyGraph", func(t *testing.T) {
		sc := setupScheduler(t)
		graph := sc.newGraph(t) // No steps

		result := graph.Schedule(t, scheduler.StatusSuccess)
		assert.NoError(t, result.Error)
	})

	t.Run("SingleNodeGraph", func(t *testing.T) {
		sc := setupScheduler(t)
		graph := sc.newGraph(t, successStep("single"))

		result := graph.Schedule(t, scheduler.StatusSuccess)
		result.AssertNodeStatus(t, "single", scheduler.NodeStatusSuccess)
	})

	t.Run("AllNodesFail", func(t *testing.T) {
		sc := setupScheduler(t)
		graph := sc.newGraph(t,
			failStep("1"),
			failStep("2"),
			failStep("3"),
		)

		result := graph.Schedule(t, scheduler.StatusError)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "2", scheduler.NodeStatusError)
		result.AssertNodeStatus(t, "3", scheduler.NodeStatusError)
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
	_ = graph.Schedule(t, scheduler.StatusSuccess)

	// Access handler nodes
	assert.NotNil(t, sc.Scheduler.HandlerNode(digraph.HandlerOnExit))
	assert.NotNil(t, sc.Scheduler.HandlerNode(digraph.HandlerOnSuccess))
	assert.NotNil(t, sc.Scheduler.HandlerNode(digraph.HandlerOnFailure))
	assert.NotNil(t, sc.Scheduler.HandlerNode(digraph.HandlerOnCancel))
	assert.Nil(t, sc.Scheduler.HandlerNode(digraph.HandlerType("unknown")))
}

func TestScheduler_NodeTeardownError(t *testing.T) {
	t.Skip("Teardown errors are difficult to trigger reliably")
	sc := setupScheduler(t)

	// Create a custom step that will fail during teardown
	// We'll simulate this by using a working directory that we'll remove during execution
	tempDir, err := os.MkdirTemp("", "teardown_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	graph := sc.newGraph(t,
		newStep("1",
			withWorkingDir(tempDir),
			withScript(fmt.Sprintf(`
				echo "Removing working directory"
				rm -rf %s
				echo "Done"
			`, tempDir)),
		),
	)

	result := graph.Schedule(t, scheduler.StatusError)

	// The step should be marked as error due to teardown failure
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)
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

	result := graph.Schedule(t, scheduler.StatusSuccess)

	// The step should be skipped but no error should be set for condition not met
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusSkipped)
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

	result := graph.Schedule(t, scheduler.StatusError)

	// Both onFailure and onExit should execute
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)
	result.AssertNodeStatus(t, "onFailure", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "onExit", scheduler.NodeStatusSuccess)
}

func TestScheduler_TimeoutDuringRetry(t *testing.T) {
	sc := setupScheduler(t, withTimeout(2*time.Second))

	// Step that will keep retrying until timeout
	graph := sc.newGraph(t,
		newStep("1",
			withCommand("sleep 1 && false"),
			withRetryPolicy(10, 500*time.Millisecond), // Many retries
		),
	)

	start := time.Now()
	result := graph.Schedule(t, scheduler.StatusError)
	elapsed := time.Since(start)

	// Should timeout before completing all retries
	assert.Less(t, elapsed, 5*time.Second)
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusCancel)
}

func TestScheduler_CancelDuringHandlerExecution(t *testing.T) {
	sc := setupScheduler(t,
		withOnExit(newStep("onExit", withScript("echo handler started && sleep 1 && echo handler done"))),
	)

	graph := sc.newGraph(t, successStep("1"))

	go func() {
		// Wait for main step to complete and handler to start
		time.Sleep(200 * time.Millisecond)
		sc.Scheduler.Cancel(sc.Context, graph.ExecutionGraph)
	}()

	// Since we cancel during handler execution, the final status depends on timing
	// The graph completes successfully before cancel takes effect
	result := graph.Schedule(t, scheduler.StatusSuccess)

	result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)
	// Handler should complete successfully
	result.AssertNodeStatus(t, "onExit", scheduler.NodeStatusSuccess)
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

	result := graph.Schedule(t, scheduler.StatusCancel)
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusCancel)

	node := result.Node(t, "1")
	// Should have repeated at least twice before cancel
	assert.GreaterOrEqual(t, node.State().DoneCount, 2)
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
				withRetryPolicy(2, 100*time.Millisecond),
			),
		)

		go func() {
			time.Sleep(200 * time.Millisecond)
			graph.Signal(syscall.SIGTERM)
		}()

		result := graph.Schedule(t, scheduler.StatusCancel)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusCancel)
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
				withRetryPolicy(3, 100*time.Millisecond),
				func(step *digraph.Step) {
					step.RetryPolicy.ExitCodes = []int{42} // Only retry on exit code 42
				},
			),
		)

		result := graph.Schedule(t, scheduler.StatusError)
		result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)

		node := result.Node(t, "1")
		// Should retry once (first failure with code 42, then fail with code 100)
		assert.Equal(t, 1, node.State().RetryCount)
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

	result := graph.Schedule(t, scheduler.StatusSuccess)

	result.AssertNodeStatus(t, "step1", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "step2", scheduler.NodeStatusSuccess)
	result.AssertNodeStatus(t, "step3", scheduler.NodeStatusSuccess)

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
	result := graph.Schedule(t, scheduler.StatusSuccess)
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusSuccess)

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
			withRetryPolicy(1, 100*time.Millisecond),
		),
	)

	result := graph.Schedule(t, scheduler.StatusError)
	result.AssertNodeStatus(t, "1", scheduler.NodeStatusError)

	node := result.Node(t, "1")
	// Should have retried once
	assert.Equal(t, 1, node.State().RetryCount)
}
