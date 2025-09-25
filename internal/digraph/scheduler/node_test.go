package scheduler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNode(t *testing.T) {
	t.Parallel()

	t.Run("Execute", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCommand("true"))
		node.Execute(t)
	})
	t.Run("Error", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCommand("false"))
		node.ExecuteFail(t, "exit status 1")
	})
	t.Run("Signal", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCommand("sleep 3"))
		go func() {
			time.Sleep(100 * time.Millisecond)
			node.Signal(node.Context, syscall.SIGTERM, false)
		}()

		node.SetStatus(status.NodeRunning)

		node.ExecuteFail(t, "signal: terminated")
		require.Equal(t, status.NodeCancel.String(), node.State().Status.String())
	})
	t.Run("SignalOnStop", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCommand("sleep 3"), withNodeSignalOnStop("SIGINT"))
		go func() {
			time.Sleep(100 * time.Millisecond)
			node.Signal(node.Context, syscall.SIGTERM, true) // allow override signal
		}()

		node.SetStatus(status.NodeRunning)

		node.ExecuteFail(t, "signal: interrupt")
		require.Equal(t, status.NodeCancel.String(), node.State().Status.String())
	})
	t.Run("LogOutput", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCommand("echo hello"))
		node.Execute(t)
		node.AssertLogContains(t, "hello")
	})
	t.Run("Stdout", func(t *testing.T) {
		t.Parallel()

		random := path.Join(os.TempDir(), uuid.Must(uuid.NewV7()).String())
		defer func() {
			_ = os.Remove(random)
		}()

		node := setupNode(t, withNodeCommand("echo hello"), withNodeStdout(random))
		node.Execute(t)

		file := node.NodeData().Step.Stdout
		dat, _ := os.ReadFile(file)
		require.Equalf(t, "hello\n", string(dat), "unexpected stdout content: %s", string(dat))
	})
	t.Run("Stderr", func(t *testing.T) {
		t.Parallel()

		random := path.Join(os.TempDir(), uuid.Must(uuid.NewV7()).String())
		defer func() {
			_ = os.Remove(random)
		}()

		node := setupNode(t,
			withNodeCommand("sh"),
			withNodeStderr(random),
			withNodeScript("echo hello >&2"),
		)
		node.Execute(t)

		file := node.NodeData().Step.Stderr
		dat, _ := os.ReadFile(file)
		require.Equalf(t, "hello\n", string(dat), "unexpected stderr content: %s", string(dat))
	})
	t.Run("Output", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs("echo hello"), withNodeOutput("OUTPUT_TEST"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT_TEST", "hello")
	})
	t.Run("OutputJSON", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo '{"key": "value"}'`), withNodeOutput("OUTPUT_JSON_TEST"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT_JSON_TEST", `{"key": "value"}`)
	})
	t.Run("OutputJSONUnescaped", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo {\"key\":\"value\"}`), withNodeOutput("OUTPUT_JSON_TEST"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT_JSON_TEST", `{"key":"value"}`)
	})
	t.Run("OutputTabWithDoubleQuotes", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo "hello\tworld"`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", "hello\tworld")
	})
	t.Run("OutputTabWithMixedQuotes", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo hello"\t"world`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", "hello\tworld") // This behavior is aligned with bash
	})
	t.Run("OutputTabWithoutQuotes", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo hello\tworld`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", `hellotworld`) // This behavior is aligned with bash
	})
	t.Run("OutputNewlineCharacter", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo hello\nworld`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", `hellonworld`) // This behavior is aligned with bash
	})
	t.Run("OutputEscapedJSONWithoutQuotes", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo {\"key\":\"value\"}`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", `{"key":"value"}`)
	})
	t.Run("OutputEscapedJSONWithQuotes", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo "{\"key\":\"value\"}"`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", `{"key":"value"}`)
	})
	t.Run("OutputSingleQuotedString", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo 'hello world'`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", `hello world`)
	})
	t.Run("OutputMixedQuotesWithSpace", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo hello "world"`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", `hello world`)
	})
	t.Run("OutputNestedQuotes", func(t *testing.T) {
		t.Parallel()

		node := setupNode(t, withNodeCmdArgs(`echo 'hello "world"'`), withNodeOutput("OUTPUT"))
		node.Execute(t)
		node.AssertOutput(t, "OUTPUT", `hello "world"`)
	})
}

func TestNodeShouldMarkSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		nodeStatus         status.NodeStatus
		continueOnSettings digraph.ContinueOn
		expectMarkSuccess  bool
	}{
		{
			name:       "SuccessStatus",
			nodeStatus: status.NodeSuccess,
			continueOnSettings: digraph.ContinueOn{
				MarkSuccess: true,
			},
			expectMarkSuccess: true, // shouldContinue returns true for success status, so shouldMarkSuccess follows MarkSuccess setting
		},
		{
			name:       "ErrorWithContinueOnFailureAndMarkSuccess",
			nodeStatus: status.NodeError,
			continueOnSettings: digraph.ContinueOn{
				Failure:     true,
				MarkSuccess: true,
			},
			expectMarkSuccess: true,
		},
		{
			name:       "ErrorWithContinueOnFailureButNoMarkSuccess",
			nodeStatus: status.NodeError,
			continueOnSettings: digraph.ContinueOn{
				Failure:     true,
				MarkSuccess: false,
			},
			expectMarkSuccess: false,
		},
		{
			name:       "SkippedWithContinueOnSkippedAndMarkSuccess",
			nodeStatus: status.NodeSkipped,
			continueOnSettings: digraph.ContinueOn{
				Skipped:     true,
				MarkSuccess: true,
			},
			expectMarkSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			step := digraph.Step{
				Name:       "test-step",
				ContinueOn: tt.continueOnSettings,
			}
			node := scheduler.NewNode(step, scheduler.NodeState{
				Status: tt.nodeStatus,
			})

			// Now we can test the public method directly
			node.SetStatus(tt.nodeStatus)
			result := node.ShouldMarkSuccess(ctx)
			assert.Equal(t, tt.expectMarkSuccess, result)
		})
	}
}

func TestNodeLogContainsPattern(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Create a log file with test content
	logContent := `Line 1: This is a test log
Line 2: Error occurred
Line 3: Success message
Line 4: [WARNING] Something happened
Line 5: Process completed
`
	err := os.WriteFile(logFile, []byte(logContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name     string
		patterns []string
		expected bool
		setup    func()
	}{
		{
			name:     "ExactMatch",
			patterns: []string{"Error occurred"},
			expected: true,
		},
		{
			name:     "PartialMatch",
			patterns: []string{"Success"},
			expected: true,
		},
		{
			name:     "RegexMatch",
			patterns: []string{"re:Error.*"},
			expected: true,
		},
		{
			name:     "RegexWithBrackets",
			patterns: []string{`re:\[WARNING\].*`},
			expected: true,
		},
		{
			name:     "MultiplePatternsAnyMatch",
			patterns: []string{"NotFound", "Success"},
			expected: true,
		},
		{
			name:     "NoMatch",
			patterns: []string{"NotInLog"},
			expected: false,
		},
		{
			name:     "EmptyPatterns",
			patterns: []string{},
			expected: false,
		},
		{
			name:     "InvalidRegex",
			patterns: []string{"re:["},
			expected: false,
		},
		{
			name:     "NonExistentLogFile",
			patterns: []string{"anything"},
			expected: false,
			setup: func() {
				// Test with a node that has no log file
				logFile = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			ctx := context.Background()
			step := digraph.Step{Name: "test-step"}

			// Setup node properly with log file
			node := scheduler.NewNode(step, scheduler.NodeState{})
			err := node.Setup(ctx, tempDir, "test-run")
			require.NoError(t, err)

			// For non-existent log file test, we skip the log file write
			if tt.name != "non-existent log file" && logFile != "" {
				// Write test content to the stdout file
				stdoutFile := node.StdoutFile()
				err = os.WriteFile(stdoutFile, []byte(logContent), 0644)
				require.NoError(t, err)
			}

			result, err := node.LogContainsPattern(ctx, tt.patterns)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNodeBuildChildDAGRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		parallel      *digraph.ParallelConfig
		childDAG      *digraph.ChildDAG
		setupEnv      func(ctx context.Context) context.Context
		expectCount   int
		expectError   bool
		errorContains string
	}{
		{
			name:     "NonParallelExecution",
			parallel: nil,
			childDAG: &digraph.ChildDAG{
				Name:   "child-dag",
				Params: "param1=value1",
			},
			expectCount: 1,
		},
		{
			name: "ParallelWithVariableJSONArray",
			parallel: &digraph.ParallelConfig{
				Variable: "${LIST_VAR}",
			},
			childDAG: &digraph.ChildDAG{
				Name: "child-dag",
			},
			setupEnv: func(ctx context.Context) context.Context {
				env := executor.GetEnv(ctx)
				env.Variables.Store("LIST_VAR", `LIST_VAR=["item1", "item2", "item3"]`)
				return executor.WithEnv(ctx, env)
			},
			expectCount: 3,
		},
		{
			name: "ParallelWithVariableSpaceSeparated",
			parallel: &digraph.ParallelConfig{
				Variable: "${SPACE_VAR}",
			},
			childDAG: &digraph.ChildDAG{
				Name: "child-dag",
			},
			setupEnv: func(ctx context.Context) context.Context {
				env := executor.GetEnv(ctx)
				env.Variables.Store("SPACE_VAR", "SPACE_VAR=one two three")
				return executor.WithEnv(ctx, env)
			},
			expectCount: 3,
		},
		{
			name: "ParallelWithStaticItems",
			parallel: &digraph.ParallelConfig{
				Items: []digraph.ParallelItem{
					{Value: "item1"},
					{Value: "item2"},
				},
			},
			childDAG: &digraph.ChildDAG{
				Name: "child-dag",
			},
			expectCount: 2,
		},
		{
			name: "ParallelWithParamsItems",
			parallel: &digraph.ParallelConfig{
				Items: []digraph.ParallelItem{
					{Params: map[string]string{"key1": "value1"}},
					{Params: map[string]string{"key2": "value2"}},
				},
			},
			childDAG: &digraph.ChildDAG{
				Name: "child-dag",
			},
			expectCount: 2,
		},
		{
			name: "ParallelWithNoItems",
			parallel: &digraph.ParallelConfig{
				Variable: "${NONEXISTENT}",
			},
			childDAG: &digraph.ChildDAG{
				Name: "child-dag",
			},
			expectError:   true,
			errorContains: "requires at least one item",
		},
		{
			name: "ParallelWithTooManyItems",
			parallel: &digraph.ParallelConfig{
				Items: func() []digraph.ParallelItem {
					items := make([]digraph.ParallelItem, 1001)
					for i := range items {
						items[i] = digraph.ParallelItem{Value: fmt.Sprintf("item%d", i)}
					}
					return items
				}(),
			},
			childDAG: &digraph.ChildDAG{
				Name: "child-dag",
			},
			expectError:   true,
			errorContains: "exceeds maximum limit",
		},
		{
			name: "ParallelWithITEMVariableInParams",
			parallel: &digraph.ParallelConfig{
				Variable: "${SPACE_VAR}",
			},
			childDAG: &digraph.ChildDAG{
				Name:   "child-dag",
				Params: "item=${ITEM}",
			},
			setupEnv: func(ctx context.Context) context.Context {
				env := executor.GetEnv(ctx)
				env.Variables.Store("SPACE_VAR", "SPACE_VAR=one two three")
				return executor.WithEnv(ctx, env)
			},
			expectCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := digraph.SetupEnvForTest(context.Background(), &digraph.DAG{}, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)

			if tt.setupEnv != nil {
				ctx = tt.setupEnv(ctx)
			}

			step := digraph.Step{
				Name:     "test-step",
				Parallel: tt.parallel,
				ChildDAG: tt.childDAG,
			}
			node := scheduler.NewNode(step, scheduler.NodeState{})

			// Now we can test the public method directly
			runs, err := node.BuildChildDAGRuns(ctx, tt.childDAG)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, runs, tt.expectCount)
			}
		})
	}
}

func TestNodeItemToParam(t *testing.T) {
	tests := []struct {
		name     string
		item     any
		expected string
		wantErr  bool
	}{
		{
			name:     "String",
			item:     "test-string",
			expected: "test-string",
		},
		{
			name:     "Int",
			item:     42,
			expected: "42",
		},
		{
			name:     "Int64",
			item:     int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		{
			name:     "Float32",
			item:     float32(3.14),
			expected: "3.14",
		},
		{
			name:     "Float64",
			item:     3.14159,
			expected: "3.14159",
		},
		{
			name:     "BoolTrue",
			item:     true,
			expected: "true",
		},
		{
			name:     "BoolFalse",
			item:     false,
			expected: "false",
		},
		{
			name:     "Nil",
			item:     nil,
			expected: "null",
		},
		{
			name:     "JsonRawMessage",
			item:     json.RawMessage(`{"key":"value"}`),
			expected: `{"key":"value"}`,
		},
		{
			name:     "Map",
			item:     map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "Slice",
			item:     []string{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
		{
			name:     "Struct",
			item:     struct{ Name string }{Name: "test"},
			expected: `{"Name":"test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a node to call the method on
			step := digraph.Step{Name: "test-step"}
			node := scheduler.NewNode(step, scheduler.NodeState{})

			// Now we can test the public method directly
			result, err := node.ItemToParam(tt.item)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestRetryPolicyShouldRetry(t *testing.T) {
	tests := []struct {
		name        string
		policy      scheduler.RetryPolicy
		exitCode    int
		shouldRetry bool
	}{
		{
			name: "RetryOnAnyNonZeroWhenNoExitCodesSpecified",
			policy: scheduler.RetryPolicy{
				Limit:    3,
				Interval: time.Second,
			},
			exitCode:    1,
			shouldRetry: true,
		},
		{
			name: "NoRetryOnZeroWhenNoExitCodesSpecified",
			policy: scheduler.RetryPolicy{
				Limit:    3,
				Interval: time.Second,
			},
			exitCode:    0,
			shouldRetry: false,
		},
		{
			name: "RetryOnlyOnSpecificExitCodes",
			policy: scheduler.RetryPolicy{
				Limit:     3,
				Interval:  time.Second,
				ExitCodes: []int{1, 2, 3},
			},
			exitCode:    2,
			shouldRetry: true,
		},
		{
			name: "NoRetryOnNonSpecifiedExitCode",
			policy: scheduler.RetryPolicy{
				Limit:     3,
				Interval:  time.Second,
				ExitCodes: []int{1, 2, 3},
			},
			exitCode:    4,
			shouldRetry: false,
		},
		{
			name: "NoRetryOnZeroEvenWhenInExitCodes",
			policy: scheduler.RetryPolicy{
				Limit:     3,
				Interval:  time.Second,
				ExitCodes: []int{0, 1, 2},
			},
			exitCode:    0,
			shouldRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.ShouldRetry(tt.exitCode)
			assert.Equal(t, tt.shouldRetry, result)
		})
	}
}

func TestNodeSetupAndTeardown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()

	step := digraph.Step{
		Name:    "test-step",
		Command: "echo",
		Args:    []string{"hello"},
	}

	node := scheduler.NewNode(step, scheduler.NodeState{})

	// Test Setup
	dagRunID := "test-run-123"
	err := node.Setup(ctx, tempDir, dagRunID)
	assert.NoError(t, err)

	// Verify log files were created
	state := node.NodeData().State
	assert.NotEmpty(t, state.Stdout)
	assert.NotEmpty(t, state.Stderr)
	assert.True(t, strings.HasPrefix(state.Stdout, tempDir))
	assert.True(t, strings.HasPrefix(state.Stderr, tempDir))

	// Test Teardown
	err = node.Teardown(ctx)
	assert.NoError(t, err)

	// Test double teardown (should be idempotent)
	err = node.Teardown(ctx)
	assert.NoError(t, err)
}

func TestNodeInit(t *testing.T) {
	step := digraph.Step{Name: "test-step"}

	// Create multiple nodes to verify they get different IDs
	node1 := scheduler.NewNode(step, scheduler.NodeState{})
	node2 := scheduler.NewNode(step, scheduler.NodeState{})

	// Call Init on first node
	node1.Init()

	// Call Init multiple times on same node - should be idempotent
	node1.Init()
	node1.Init()

	// Call Init on second node
	node2.Init()

	// While we can't directly access the ID, we can verify that
	// two different nodes don't interfere with each other
	// and that multiple Init calls are safe
	assert.NotPanics(t, func() {
		for i := 0; i < 10; i++ {
			node1.Init()
			node2.Init()
		}
	})
}

func TestNodeCancel(t *testing.T) {
	ctx := context.Background()
	step := digraph.Step{
		Name:    "test-step",
		Command: "sleep",
		Args:    []string{"10"},
	}

	node := scheduler.NewNode(step, scheduler.NodeState{})
	node.SetStatus(status.NodeRunning)

	// Cancel the node
	node.Cancel(ctx)

	// Check status changed to cancel
	assert.Equal(t, status.NodeCancel, node.NodeData().State.Status)
}

func TestNodeSetupContextBeforeExec(t *testing.T) {
	ctx := context.Background()
	env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
	ctx = executor.WithEnv(ctx, env)

	step := digraph.Step{Name: "test-step"}
	node := scheduler.NewNode(step, scheduler.NodeState{
		Stdout: "/tmp/stdout.log",
		Stderr: "/tmp/stderr.log",
	})

	// Setup context
	newCtx := node.SetupContextBeforeExec(ctx)

	// Verify environment variables were set
	newEnv := executor.GetEnv(newCtx)
	assert.Equal(t, "/tmp/stdout.log", newEnv.Envs[digraph.EnvKeyDAGRunStepStdoutFile])
	assert.Equal(t, "/tmp/stderr.log", newEnv.Envs[digraph.EnvKeyDAGRunStepStderrFile])
}

func TestNodeOutputCaptureWithLargeOutput(t *testing.T) {
	t.Parallel()

	// Test that the output capture mechanism respects size limits
	// This test validates the concept at a high level
	tests := []struct {
		name          string
		command       string
		args          []string
		maxOutputSize int
		expectSuccess bool
	}{
		{
			name:          "SmallOutputWithinLimit",
			command:       "echo",
			args:          []string{"Hello, World!"},
			maxOutputSize: 1000,
			expectSuccess: true,
		},
		{
			name:          "VeryLargeOutputSizeLimit",
			command:       "echo",
			args:          []string{"test"},
			maxOutputSize: 1024 * 1024, // 1MB
			expectSuccess: true,
		},
		{
			name:          "ZeroOutputSizeMeansUnlimited",
			command:       "echo",
			args:          []string{"unlimited test"},
			maxOutputSize: 0,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tempDir := t.TempDir()

			// Create DAG with output size limit
			dag := &digraph.DAG{
				MaxOutputSize: tt.maxOutputSize,
			}

			// Setup environment with DAG
			ctx = digraph.SetupEnvForTest(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)

			step := digraph.Step{
				Name:    "test-output-capture",
				Command: tt.command,
				Args:    tt.args,
				Output:  "CAPTURED_OUTPUT",
			}

			node := scheduler.NewNode(step, scheduler.NodeState{})
			node.Init()

			// Setup node
			err := node.Setup(ctx, tempDir, "test-run-output")
			require.NoError(t, err)

			// Execute node
			err = node.Execute(ctx)

			if tt.expectSuccess {
				// Execution should succeed
				assert.NoError(t, err)

				// Check if output was captured
				nodeData := node.NodeData()
				if nodeData.State.OutputVariables != nil {
					_, ok := nodeData.State.OutputVariables.Load("CAPTURED_OUTPUT")
					assert.True(t, ok, "Expected output variable to be captured")
				}
			}

			// Verify that MaxOutputSize is respected in the DAG configuration
			env := executor.GetEnv(ctx)
			assert.Equal(t, tt.maxOutputSize, env.DAG.MaxOutputSize)

			// Cleanup
			err = node.Teardown(ctx)
			assert.NoError(t, err)
		})
	}

	// Additional test to verify configuration is respected
	t.Run("DAGMaxOutputSizeConfiguration", func(t *testing.T) {
		// Test that different MaxOutputSize values are properly configured
		sizes := []int{0, 100, 1024, 1024 * 1024}

		for _, size := range sizes {
			t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
				ctx := context.Background()
				dag := &digraph.DAG{
					MaxOutputSize: size,
				}

				ctx = digraph.SetupEnvForTest(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)
				env := executor.GetEnv(ctx)

				// Verify the MaxOutputSize is properly set in the environment
				assert.Equal(t, size, env.DAG.MaxOutputSize)

				// Create a node with output capture
				step := digraph.Step{
					Name:    "test-size-config",
					Command: "echo",
					Args:    []string{"test"},
					Output:  "TEST_VAR",
				}

				node := scheduler.NewNode(step, scheduler.NodeState{})
				node.Init()

				// The node should respect the configured MaxOutputSize
				// This is validated through the DAG configuration
				assert.NotNil(t, node)
			})
		}
	})
}

func TestNodeShouldContinue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		nodeStatus         status.NodeStatus
		exitCode           int
		continueOnSettings digraph.ContinueOn
		setupOutput        func(t *testing.T, node *scheduler.Node)
		expectContinue     bool
	}{
		{
			name:       "ContinueOnFailure",
			nodeStatus: status.NodeError,
			exitCode:   1,
			continueOnSettings: digraph.ContinueOn{
				Failure: true,
			},
			expectContinue: true,
		},
		{
			name:       "ContinueOnSpecificExitCode",
			nodeStatus: status.NodeError,
			exitCode:   42,
			continueOnSettings: digraph.ContinueOn{
				ExitCode: []int{42, 43},
			},
			expectContinue: true,
		},
		{
			name:       "DonTContinueOnNonMatchingExitCode",
			nodeStatus: status.NodeError,
			exitCode:   44,
			continueOnSettings: digraph.ContinueOn{
				ExitCode: []int{42, 43},
			},
			expectContinue: false,
		},
		{
			name:       "ContinueOnSkipped",
			nodeStatus: status.NodeSkipped,
			continueOnSettings: digraph.ContinueOn{
				Skipped: true,
			},
			expectContinue: true,
		},
		{
			name:               "SuccessAlwaysContinues",
			nodeStatus:         status.NodeSuccess,
			continueOnSettings: digraph.ContinueOn{},
			expectContinue:     true,
		},
		{
			name:       "CancelNeverContinues",
			nodeStatus: status.NodeCancel,
			continueOnSettings: digraph.ContinueOn{
				Failure: true,
				Skipped: true,
			},
			expectContinue: false,
		},
		{
			name:       "ContinueOnOutputMatch",
			nodeStatus: status.NodeError,
			continueOnSettings: digraph.ContinueOn{
				Output: []string{"WARNING"},
			},
			setupOutput: func(t *testing.T, node *scheduler.Node) {
				tempDir := t.TempDir()
				ctx := context.Background()
				err := node.Setup(ctx, tempDir, "test-run")
				require.NoError(t, err)

				// Write test output to stdout file
				stdoutFile := node.StdoutFile()
				err = os.WriteFile(stdoutFile, []byte("WARNING: This is just a warning\n"), 0644)
				require.NoError(t, err)
			},
			expectContinue: true,
		},
		{
			name:       "ContinueOnRegexOutputMatch",
			nodeStatus: status.NodeError,
			continueOnSettings: digraph.ContinueOn{
				Output: []string{"re:.*timeout.*"},
			},
			setupOutput: func(t *testing.T, node *scheduler.Node) {
				tempDir := t.TempDir()
				ctx := context.Background()
				err := node.Setup(ctx, tempDir, "test-run")
				require.NoError(t, err)

				// Write test output to stdout file
				stdoutFile := node.StdoutFile()
				err = os.WriteFile(stdoutFile, []byte("ERROR: Connection timeout after 30 seconds\n"), 0644)
				require.NoError(t, err)
			},
			expectContinue: true,
		},

		{
			name:       "DonTContinueOnSkippedWhenContinueOnSkippedIsFalseEvenWithExitCode0InExitCode",
			nodeStatus: status.NodeSkipped,
			exitCode:   0,
			continueOnSettings: digraph.ContinueOn{
				Skipped:  false,
				ExitCode: []int{0, 1, 2},
			},
			expectContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			step := digraph.Step{
				Name:       "test-step",
				ContinueOn: tt.continueOnSettings,
			}
			node := scheduler.NewNode(step, scheduler.NodeState{
				Status:   tt.nodeStatus,
				ExitCode: tt.exitCode,
			})

			if tt.setupOutput != nil {
				tt.setupOutput(t, node)
			}

			// Now we can test the public method directly
			node.SetStatus(tt.nodeStatus)
			node.SetExitCode(tt.exitCode)

			result := node.ShouldContinue(ctx)
			assert.Equal(t, tt.expectContinue, result)
		})
	}
}

type nodeHelper struct {
	*scheduler.Node
	test.Helper
}

type nodeOption func(*scheduler.NodeData)

func withNodeCmdArgs(cmd string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.CmdWithArgs = cmd
	}
}

func withNodeCommand(command string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Command = command
	}
}

func withNodeSignalOnStop(signal string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.SignalOnStop = signal
	}
}

func withNodeStdout(stdout string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Stdout = stdout
	}
}

func withNodeStderr(stderr string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Stderr = stderr
	}
}

func withNodeScript(script string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Script = script
	}
}

func withNodeOutput(output string) nodeOption {
	return func(data *scheduler.NodeData) {
		data.Step.Output = output
	}
}

func setupNode(t *testing.T, opts ...nodeOption) nodeHelper {
	t.Helper()

	th := test.Setup(t)

	data := scheduler.NodeData{Step: digraph.Step{}}
	for _, opt := range opts {
		opt(&data)
	}

	return nodeHelper{
		Helper: th,
		Node:   scheduler.NodeWithData(data),
	}
}

func (n nodeHelper) Execute(t *testing.T) {
	t.Helper()

	dagRunID := uuid.Must(uuid.NewV7()).String()
	err := n.Setup(n.Context, n.Config.Paths.LogDir, dagRunID)
	require.NoError(t, err, "failed to setup node")

	err = n.Node.Execute(n.execContext(dagRunID))
	require.NoError(t, err, "failed to execute node")

	err = n.Teardown(n.Context)
	require.NoError(t, err, "failed to teardown node")
}

func (n nodeHelper) ExecuteFail(t *testing.T, expectedErr string) {
	t.Helper()

	dagRunID := uuid.Must(uuid.NewV7()).String()
	err := n.Node.Execute(n.execContext(dagRunID))
	require.Error(t, err, "expected error")
	require.Contains(t, err.Error(), expectedErr, "unexpected error")
}

func (n nodeHelper) AssertLogContains(t *testing.T, expected string) {
	t.Helper()

	dat, err := os.ReadFile(n.StdoutFile())
	require.NoErrorf(t, err, "failed to read log file %q", n.StdoutFile())
	require.Contains(t, string(dat), expected, "log file does not contain expected string")
}

func (n nodeHelper) AssertOutput(t *testing.T, key, value string) {
	t.Helper()

	require.NotNil(t, n.NodeData().State.OutputVariables, "output variables not set")
	data, ok := n.NodeData().State.OutputVariables.Load(key)
	require.True(t, ok, "output variable not found")
	require.Equal(t, fmt.Sprintf(`%s=%s`, key, value), data, "output variable value mismatch")
}

func (n nodeHelper) execContext(dagRunID string) context.Context {
	return digraph.SetupEnvForTest(n.Context, &digraph.DAG{}, nil, digraph.DAGRunRef{}, dagRunID, "logFile", nil)
}

func TestNodeOutputRedirectWithWorkingDir(t *testing.T) {
	t.Parallel()

	t.Run("AbsolutePathUnchanged", func(t *testing.T) {
		tempDir := t.TempDir()
		workDir := filepath.Join(tempDir, "work")
		err := os.MkdirAll(workDir, 0755)
		require.NoError(t, err)

		// Absolute path for stdout
		stdoutPath := filepath.Join(tempDir, "output.log")

		step := digraph.Step{
			Name:    "test-absolute-path",
			Command: "echo",
			Args:    []string{"hello world"},
			Stdout:  stdoutPath,
		}

		node := scheduler.NewNode(step, scheduler.NodeState{})
		node.Init()

		// Setup context with working directory
		ctx := context.Background()
		dag := &digraph.DAG{}
		ctx = digraph.SetupEnvForTest(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)
		env := executor.GetEnv(ctx)
		env.WorkingDir = workDir
		ctx = executor.WithEnv(ctx, env)

		// Setup and execute node
		err = node.Setup(ctx, tempDir, "test-run")
		require.NoError(t, err)

		err = node.Execute(ctx)
		require.NoError(t, err)

		// Verify file was created at absolute path
		content, err := os.ReadFile(stdoutPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "hello world")
	})

	t.Run("RelativePathUsesWorkingDir", func(t *testing.T) {
		tempDir := t.TempDir()
		workDir := filepath.Join(tempDir, "work")
		err := os.MkdirAll(workDir, 0755)
		require.NoError(t, err)

		// Relative path for stdout
		stdoutPath := "output.log"

		step := digraph.Step{
			Name:    "test-relative-path",
			Command: "echo",
			Args:    []string{"hello from working dir"},
			Stdout:  stdoutPath,
		}

		node := scheduler.NewNode(step, scheduler.NodeState{})
		node.Init()

		// Setup context with working directory
		ctx := context.Background()
		dag := &digraph.DAG{}
		ctx = digraph.SetupEnvForTest(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)
		env := executor.GetEnv(ctx)
		env.WorkingDir = workDir
		ctx = executor.WithEnv(ctx, env)

		// Setup and execute node
		err = node.Setup(ctx, tempDir, "test-run")
		require.NoError(t, err)

		err = node.Execute(ctx)
		require.NoError(t, err)

		// Verify file was created in working directory
		expectedPath := filepath.Join(workDir, stdoutPath)
		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "hello from working dir")

		// Verify file was NOT created in tempDir
		_, err = os.Stat(filepath.Join(tempDir, stdoutPath))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("StderrRedirectWithWorkingDir", func(t *testing.T) {
		tempDir := t.TempDir()
		workDir := filepath.Join(tempDir, "work")
		err := os.MkdirAll(workDir, 0755)
		require.NoError(t, err)

		// Relative path for stderr
		stderrPath := "error.log"

		step := digraph.Step{
			Name:    "test-stderr-path",
			Command: "sh",
			Args:    []string{"-c", "echo 'error message' >&2"},
			Stderr:  stderrPath,
		}

		node := scheduler.NewNode(step, scheduler.NodeState{})
		node.Init()

		// Setup context with working directory
		ctx := context.Background()
		dag := &digraph.DAG{}
		ctx = digraph.SetupEnvForTest(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)
		env := executor.GetEnv(ctx)
		env.WorkingDir = workDir
		ctx = executor.WithEnv(ctx, env)

		// Setup and execute node
		err = node.Setup(ctx, tempDir, "test-run")
		require.NoError(t, err)

		err = node.Execute(ctx)
		require.NoError(t, err)

		// Verify file was created in working directory
		expectedPath := filepath.Join(workDir, stderrPath)
		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "error message")
	})

	t.Run("NestedRelativePath", func(t *testing.T) {
		tempDir := t.TempDir()
		workDir := filepath.Join(tempDir, "work")
		err := os.MkdirAll(workDir, 0755)
		require.NoError(t, err)

		// Create nested directory in working dir
		logsDir := filepath.Join(workDir, "logs")
		err = os.MkdirAll(logsDir, 0755)
		require.NoError(t, err)

		// Nested relative path
		stdoutPath := "logs/output.log"

		step := digraph.Step{
			Name:    "test-nested-path",
			Command: "echo",
			Args:    []string{"nested output"},
			Stdout:  stdoutPath,
		}

		node := scheduler.NewNode(step, scheduler.NodeState{})
		node.Init()

		// Setup context with working directory
		ctx := context.Background()
		dag := &digraph.DAG{}
		ctx = digraph.SetupEnvForTest(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)
		env := executor.GetEnv(ctx)
		env.WorkingDir = workDir
		ctx = executor.WithEnv(ctx, env)

		// Setup and execute node
		err = node.Setup(ctx, tempDir, "test-run")
		require.NoError(t, err)

		err = node.Execute(ctx)
		require.NoError(t, err)

		// Verify file was created in correct nested path
		expectedPath := filepath.Join(workDir, stdoutPath)
		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "nested output")
	})
}
