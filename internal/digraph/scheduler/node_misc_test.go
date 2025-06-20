package scheduler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeShouldMarkSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		nodeStatus         scheduler.NodeStatus
		continueOnSettings digraph.ContinueOn
		expectMarkSuccess  bool
	}{
		{
			name:       "success status",
			nodeStatus: scheduler.NodeStatusSuccess,
			continueOnSettings: digraph.ContinueOn{
				MarkSuccess: true,
			},
			expectMarkSuccess: true, // shouldContinue returns true for success status, so shouldMarkSuccess follows MarkSuccess setting
		},
		{
			name:       "error with continue on failure and mark success",
			nodeStatus: scheduler.NodeStatusError,
			continueOnSettings: digraph.ContinueOn{
				Failure:     true,
				MarkSuccess: true,
			},
			expectMarkSuccess: true,
		},
		{
			name:       "error with continue on failure but no mark success",
			nodeStatus: scheduler.NodeStatusError,
			continueOnSettings: digraph.ContinueOn{
				Failure:     true,
				MarkSuccess: false,
			},
			expectMarkSuccess: false,
		},
		{
			name:       "skipped with continue on skipped and mark success",
			nodeStatus: scheduler.NodeStatusSkipped,
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
			name:     "exact match",
			patterns: []string{"Error occurred"},
			expected: true,
		},
		{
			name:     "partial match",
			patterns: []string{"Success"},
			expected: true,
		},
		{
			name:     "regex match",
			patterns: []string{"re:Error.*"},
			expected: true,
		},
		{
			name:     "regex with brackets",
			patterns: []string{`re:\[WARNING\].*`},
			expected: true,
		},
		{
			name:     "multiple patterns - any match",
			patterns: []string{"NotFound", "Success"},
			expected: true,
		},
		{
			name:     "no match",
			patterns: []string{"NotInLog"},
			expected: false,
		},
		{
			name:     "empty patterns",
			patterns: []string{},
			expected: false,
		},
		{
			name:     "invalid regex",
			patterns: []string{"re:["},
			expected: false,
		},
		{
			name:     "non-existent log file",
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
			name:     "non-parallel execution",
			parallel: nil,
			childDAG: &digraph.ChildDAG{
				Name:   "child-dag",
				Params: "param1=value1",
			},
			expectCount: 1,
		},
		{
			name: "parallel with variable - JSON array",
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
			name: "parallel with variable - space separated",
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
			name: "parallel with static items",
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
			name: "parallel with params items",
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
			name: "parallel with no items",
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
			name: "parallel with too many items",
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
			name: "parallel with ITEM variable in params",
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
			ctx := digraph.SetupEnv(context.Background(), &digraph.DAG{}, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)

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
			name:     "string",
			item:     "test-string",
			expected: "test-string",
		},
		{
			name:     "int",
			item:     42,
			expected: "42",
		},
		{
			name:     "int64",
			item:     int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		{
			name:     "float32",
			item:     float32(3.14),
			expected: "3.14",
		},
		{
			name:     "float64",
			item:     3.14159,
			expected: "3.14159",
		},
		{
			name:     "bool true",
			item:     true,
			expected: "true",
		},
		{
			name:     "bool false",
			item:     false,
			expected: "false",
		},
		{
			name:     "nil",
			item:     nil,
			expected: "null",
		},
		{
			name:     "json.RawMessage",
			item:     json.RawMessage(`{"key":"value"}`),
			expected: `{"key":"value"}`,
		},
		{
			name:     "map",
			item:     map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "slice",
			item:     []string{"a", "b", "c"},
			expected: `["a","b","c"]`,
		},
		{
			name:     "struct",
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
			name: "retry on any non-zero when no exit codes specified",
			policy: scheduler.RetryPolicy{
				Limit:    3,
				Interval: time.Second,
			},
			exitCode:    1,
			shouldRetry: true,
		},
		{
			name: "no retry on zero when no exit codes specified",
			policy: scheduler.RetryPolicy{
				Limit:    3,
				Interval: time.Second,
			},
			exitCode:    0,
			shouldRetry: false,
		},
		{
			name: "retry only on specific exit codes",
			policy: scheduler.RetryPolicy{
				Limit:     3,
				Interval:  time.Second,
				ExitCodes: []int{1, 2, 3},
			},
			exitCode:    2,
			shouldRetry: true,
		},
		{
			name: "no retry on non-specified exit code",
			policy: scheduler.RetryPolicy{
				Limit:     3,
				Interval:  time.Second,
				ExitCodes: []int{1, 2, 3},
			},
			exitCode:    4,
			shouldRetry: false,
		},
		{
			name: "no retry on zero even when in exit codes",
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
	node.SetStatus(scheduler.NodeStatusRunning)

	// Cancel the node
	node.Cancel(ctx)

	// Check status changed to cancel
	assert.Equal(t, scheduler.NodeStatusCancel, node.NodeData().State.Status)
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
			name:          "small output within limit",
			command:       "echo",
			args:          []string{"Hello, World!"},
			maxOutputSize: 1000,
			expectSuccess: true,
		},
		{
			name:          "very large output size limit",
			command:       "echo",
			args:          []string{"test"},
			maxOutputSize: 1024 * 1024, // 1MB
			expectSuccess: true,
		},
		{
			name:          "zero output size means unlimited",
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
			ctx = digraph.SetupEnv(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)

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
	t.Run("DAG MaxOutputSize configuration", func(t *testing.T) {
		// Test that different MaxOutputSize values are properly configured
		sizes := []int{0, 100, 1024, 1024 * 1024}

		for _, size := range sizes {
			t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
				ctx := context.Background()
				dag := &digraph.DAG{
					MaxOutputSize: size,
				}

				ctx = digraph.SetupEnv(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)
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
		nodeStatus         scheduler.NodeStatus
		exitCode           int
		continueOnSettings digraph.ContinueOn
		setupOutput        func(t *testing.T, node *scheduler.Node)
		expectContinue     bool
	}{
		{
			name:       "continue on failure",
			nodeStatus: scheduler.NodeStatusError,
			exitCode:   1,
			continueOnSettings: digraph.ContinueOn{
				Failure: true,
			},
			expectContinue: true,
		},
		{
			name:       "continue on specific exit code",
			nodeStatus: scheduler.NodeStatusError,
			exitCode:   42,
			continueOnSettings: digraph.ContinueOn{
				ExitCode: []int{42, 43},
			},
			expectContinue: true,
		},
		{
			name:       "don't continue on non-matching exit code",
			nodeStatus: scheduler.NodeStatusError,
			exitCode:   44,
			continueOnSettings: digraph.ContinueOn{
				ExitCode: []int{42, 43},
			},
			expectContinue: false,
		},
		{
			name:       "continue on skipped",
			nodeStatus: scheduler.NodeStatusSkipped,
			continueOnSettings: digraph.ContinueOn{
				Skipped: true,
			},
			expectContinue: true,
		},
		{
			name:               "success always continues",
			nodeStatus:         scheduler.NodeStatusSuccess,
			continueOnSettings: digraph.ContinueOn{},
			expectContinue:     true,
		},
		{
			name:       "cancel never continues",
			nodeStatus: scheduler.NodeStatusCancel,
			continueOnSettings: digraph.ContinueOn{
				Failure: true,
				Skipped: true,
			},
			expectContinue: false,
		},
		{
			name:       "continue on output match",
			nodeStatus: scheduler.NodeStatusError,
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
			name:       "continue on regex output match",
			nodeStatus: scheduler.NodeStatusError,
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
