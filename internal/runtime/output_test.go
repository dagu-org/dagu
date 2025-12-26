package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNode_LargeOutput(t *testing.T) {
	tests := []struct {
		name       string
		outputSize int
		script     string
		expectHang bool
	}{
		{
			name:       "SmallOutput",
			outputSize: 1024, // 1KB
			script:     `python3 -c "print('x' * 1024)"`,
			expectHang: false,
		},
		{
			name:       "MediumOutput",
			outputSize: 32 * 1024, // 32KB
			script:     `python3 -c "print('x' * (32 * 1024))"`,
			expectHang: false,
		},
		{
			name:       "LargeOutputJustBelow64KB",
			outputSize: 63 * 1024, // 63KB
			script:     `python3 -c "print('x' * (63 * 1024))"`,
			expectHang: false,
		},
		{
			name:       "LargeOutputAt64KB",
			outputSize: 64 * 1024, // 64KB
			script:     `python3 -c "print('x' * (64 * 1024))"`,
			expectHang: false, // Fixed - no longer hangs
		},
		{
			name:       "LargeOutputAbove64KB",
			outputSize: 128 * 1024, // 128KB
			script:     `python3 -c "print('x' * (128 * 1024))"`,
			expectHang: false, // Fixed - no longer hangs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := core.Step{
				Name:   "test",
				Output: "RESULT",
				Commands: []core.CommandEntry{{
					Command: "sh",
					Args:    []string{"-c", tt.script},
				}},
			}

			node := NewNode(step, NodeState{})
			ctx := context.Background()
			// Set up environment context with proper DAG
			dag := &core.DAG{Name: "test"}
			ctx = NewContext(ctx, dag, "test-run", "test.log")

			// Setup node with a temporary directory
			tmpDir := t.TempDir()
			err := node.Prepare(ctx, tmpDir, "test-run")
			require.NoError(t, err)

			// Execute with timeout to detect hanging
			done := make(chan error, 1)
			go func() {
				done <- node.Execute(ctx)
			}()

			select {
			case err := <-done:
				if tt.expectHang {
					t.Errorf("Expected hanging but command completed: %v", err)
				}
				// Verify output was captured correctly
				if !tt.expectHang && err == nil {
					// Access the output variable through NodeData
					nodeData := node.NodeData()
					if nodeData.State.OutputVariables != nil {
						if v, ok := nodeData.State.OutputVariables.Load("RESULT"); ok {
							output := v.(string)
							// Extract the value part after the = sign
							if idx := strings.Index(output, "="); idx != -1 {
								output = output[idx+1:]
							}
							assert.NotEmpty(t, output, "output should be captured")
							assert.True(t, strings.HasPrefix(output, "xxx"), "output should start with x's")
						} else {
							t.Error("RESULT variable not found in OutputVariables")
						}
					} else {
						t.Error("OutputVariables is nil")
					}
				}
			case <-time.After(5 * time.Second):
				if !tt.expectHang {
					t.Errorf("Command hung unexpectedly for output size %d bytes", tt.outputSize)
				}
				// Cancel the context to clean up
				node.Cancel()
			}

			// Cleanup
			_ = node.Teardown()
		})
	}
}

func TestNode_OutputCaptureDeadlock(t *testing.T) {
	// Test specifically for the pipe deadlock issue
	step := core.Step{
		Name:   "deadlock-test",
		Output: "RESULT",
		Commands: []core.CommandEntry{{
			Command: "sh",
			Args: []string{"-c", `
			# Generate exactly 64KB + 1 byte to trigger pipe buffer deadlock
			python3 -c "import sys; sys.stdout.write('x' * (64 * 1024 + 1)); sys.stdout.flush()"
		`},
		}},
	}

	node := NewNode(step, NodeState{})
	ctx := context.Background()
	// Set up environment context with proper DAG
	dag := &core.DAG{Name: "test"}
	ctx = NewContext(ctx, dag, "deadlock-test", "test.log")

	tmpDir := t.TempDir()
	err := node.Prepare(ctx, tmpDir, "deadlock-test")
	require.NoError(t, err)

	// This should complete without hanging
	done := make(chan error, 1)
	go func() {
		done <- node.Execute(ctx)
	}()

	select {
	case err := <-done:
		assert.NoError(t, err, "command should complete successfully")
		// Access the output variable through NodeData
		nodeData := node.NodeData()
		require.NotNil(t, nodeData.State.OutputVariables, "OutputVariables should not be nil")
		v, ok := nodeData.State.OutputVariables.Load("RESULT")
		require.True(t, ok, "RESULT variable should be present")
		output := v.(string)
		// Extract the value part after the = sign
		if idx := strings.Index(output, "="); idx != -1 {
			output = output[idx+1:]
		}
		assert.Len(t, output, 64*1024+1, "output should be exactly 64KB + 1 byte")
	case <-time.After(10 * time.Second):
		t.Fatal("Command execution hung - possible deadlock detected")
	}

	_ = node.Teardown()
}

func TestNode_OutputExceedsLimit(t *testing.T) {
	// Test that output exceeding the limit returns an error
	step := core.Step{
		Name:   "exceed-limit-test",
		Output: "RESULT",
		Commands: []core.CommandEntry{{
			Command: "sh",
			Args: []string{"-c", `
			# Generate 2MB of output (exceeds default 1MB limit)
			python3 -c "print('x' * (2 * 1024 * 1024))"
		`},
		}},
	}

	node := NewNode(step, NodeState{})
	ctx := context.Background()
	// Set up environment context with proper DAG
	dag := &core.DAG{Name: "test"}
	ctx = NewContext(ctx, dag, "exceed-limit-test", "test.log")

	tmpDir := t.TempDir()
	err := node.Prepare(ctx, tmpDir, "exceed-limit-test")
	require.NoError(t, err)

	// Execute should fail with output limit error
	err = node.Execute(ctx)
	if err != nil {
		t.Logf("Error: %v", err)
	}
	assert.Error(t, err, "should return error when output exceeds limit")
	assert.Contains(t, err.Error(), "output exceeded maximum size limit", "error should mention output size limit")

	_ = node.Teardown()
}

func TestNode_CustomOutputLimit(t *testing.T) {
	// Test with custom output limit
	step := core.Step{
		Name:   "custom-limit-test",
		Output: "RESULT",
		Commands: []core.CommandEntry{{
			Command: "sh",
			Args: []string{"-c", `
			# Generate 100KB of output
			python3 -c "print('x' * (100 * 1024))"
		`},
		}},
	}

	node := NewNode(step, NodeState{})
	ctx := context.Background()
	// Set up environment context with custom limit of 50KB
	dag := &core.DAG{
		Name:          "test",
		MaxOutputSize: 50 * 1024, // 50KB limit
	}
	ctx = NewContext(ctx, dag, "custom-limit-test", "test.log")

	tmpDir := t.TempDir()
	err := node.Prepare(ctx, tmpDir, "custom-limit-test")
	require.NoError(t, err)

	// Execute should fail with output limit error
	err = node.Execute(ctx)
	if err != nil {
		t.Logf("Error with custom limit: %v", err)
	}
	assert.Error(t, err, "should return error when output exceeds custom limit")
	assert.Contains(t, err.Error(), "output exceeded maximum size limit", "error should mention output size limit")

	_ = node.Teardown()
}

func TestNode_ConcurrentOutputCapture(t *testing.T) {
	// Test that output capture doesn't interfere with concurrent writes
	step := core.Step{
		Name: "concurrent-test",
		Commands: []core.CommandEntry{{
			Command: "sh",
			Args: []string{"-c", `
			# Generate output from multiple processes concurrently
			for i in $(seq 1 10); do
				(python3 -c "print('Process ' + str($i) + ': ' + 'x' * 10000)") &
			done
			wait
		`},
		}},
		Output: "RESULT",
	}

	node := NewNode(step, NodeState{})
	ctx := context.Background()
	// Set up environment context with proper DAG
	dag := &core.DAG{Name: "test"}
	ctx = NewContext(ctx, dag, "concurrent-test", "test.log")

	tmpDir := t.TempDir()
	err := node.Prepare(ctx, tmpDir, "concurrent-test")
	require.NoError(t, err)

	err = node.Execute(ctx)
	assert.NoError(t, err, "concurrent output should be handled correctly")

	// Access the output variable through NodeData
	nodeData := node.NodeData()
	require.NotNil(t, nodeData.State.OutputVariables, "OutputVariables should not be nil")
	v, ok := nodeData.State.OutputVariables.Load("RESULT")
	require.True(t, ok, "RESULT variable should be present")
	output := v.(string)
	// Extract the value part after the = sign
	if idx := strings.Index(output, "="); idx != -1 {
		output = output[idx+1:]
	}
	assert.NotEmpty(t, output, "output should be captured")
	assert.Contains(t, output, "Process", "output should contain process output")

	_ = node.Teardown()
}
