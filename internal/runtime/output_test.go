package runtime

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
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

func TestOutputCapture_BasicCapture(t *testing.T) {
	t.Parallel()

	t.Run("CapturesSmallOutput", func(t *testing.T) {
		t.Parallel()

		oc := newOutputCapture(1024 * 1024) // 1MB limit

		// Create a pipe to simulate output
		reader, writer, err := os.Pipe()
		require.NoError(t, err)

		ctx := context.Background()
		oc.start(ctx, reader)

		// Write test data
		testData := "Hello, World!"
		_, err = writer.WriteString(testData)
		require.NoError(t, err)
		_ = writer.Close()

		// Wait for capture
		output, err := oc.wait()
		assert.NoError(t, err)
		assert.Equal(t, testData, output)
	})

	t.Run("CapturesLargeOutput", func(t *testing.T) {
		t.Parallel()

		oc := newOutputCapture(1024 * 1024) // 1MB limit

		reader, writer, err := os.Pipe()
		require.NoError(t, err)

		ctx := context.Background()
		oc.start(ctx, reader)

		// Write 100KB of data
		testData := strings.Repeat("x", 100*1024)
		_, err = writer.WriteString(testData)
		require.NoError(t, err)
		_ = writer.Close()

		output, err := oc.wait()
		assert.NoError(t, err)
		assert.Equal(t, testData, output)
	})

	t.Run("TruncatesExcessOutput", func(t *testing.T) {
		t.Parallel()

		maxSize := int64(1024) // 1KB limit
		oc := newOutputCapture(maxSize)

		reader, writer, err := os.Pipe()
		require.NoError(t, err)

		ctx := context.Background()
		oc.start(ctx, reader)

		// Write data larger than limit
		testData := strings.Repeat("x", 2048) // 2KB
		_, err = writer.WriteString(testData)
		require.NoError(t, err)
		_ = writer.Close()

		output, err := oc.wait()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeded maximum size limit")
		assert.Equal(t, int(maxSize), len(output))
	})

	t.Run("HandlesEmptyOutput", func(t *testing.T) {
		t.Parallel()

		oc := newOutputCapture(1024)

		reader, writer, err := os.Pipe()
		require.NoError(t, err)

		ctx := context.Background()
		oc.start(ctx, reader)

		// Close writer immediately with no data
		_ = writer.Close()

		output, err := oc.wait()
		assert.NoError(t, err)
		assert.Empty(t, output)
	})
}

func TestOutputCoordinator_StdoutFile(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsFileName", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{
			stdoutFileName: "/var/log/test.stdout.log",
		}

		result := oc.StdoutFile()
		assert.Equal(t, "/var/log/test.stdout.log", result)
	})

	t.Run("ReturnsEmptyWhenNotSet", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{}

		result := oc.StdoutFile()
		assert.Empty(t, result)
	})
}

func TestOutputCoordinator_FlushWriters(t *testing.T) {
	t.Parallel()

	t.Run("NoErrorWhenClosed", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{
			closed: true,
		}

		err := oc.flushWriters()
		assert.NoError(t, err)
	})

	t.Run("NoErrorWithNilWriters", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{}

		err := oc.flushWriters()
		assert.NoError(t, err)
	})
}

func TestOutputCoordinator_CloseResources(t *testing.T) {
	t.Parallel()

	t.Run("NoErrorWhenAlreadyClosed", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{
			closed: true,
		}

		err := oc.closeResources()
		assert.NoError(t, err)
	})

	t.Run("MarksAsClosed", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{}

		_ = oc.closeResources()
		assert.True(t, oc.closed)
	})
}

// mockWriteCloser is a test implementation of io.WriteCloser
type mockWriteCloser struct {
	buf    *bytes.Buffer
	closed bool
}

func newMockWriteCloser() *mockWriteCloser {
	return &mockWriteCloser{buf: &bytes.Buffer{}}
}

func (m *mockWriteCloser) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func (m *mockWriteCloser) Close() error {
	m.closed = true
	return nil
}

// mockLogWriterFactory is a test implementation of LogWriterFactory
type mockLogWriterFactory struct {
	stdoutWriter *mockWriteCloser
	stderrWriter *mockWriteCloser
}

func newMockLogWriterFactory() *mockLogWriterFactory {
	return &mockLogWriterFactory{
		stdoutWriter: newMockWriteCloser(),
		stderrWriter: newMockWriteCloser(),
	}
}

func (m *mockLogWriterFactory) NewStepWriter(_ context.Context, _ string, streamType int) io.WriteCloser {
	if streamType == execution.StreamTypeStdout {
		return m.stdoutWriter
	}
	return m.stderrWriter
}

func TestOutputCoordinator_SetupRemoteWriters(t *testing.T) {
	t.Parallel()

	t.Run("CreatesSeparateWritersForStdoutAndStderr", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{}
		factory := newMockLogWriterFactory()
		ctx := context.Background()

		data := NodeData{
			Step: core.Step{Name: "test-step"},
			State: NodeState{
				Stdout: "/path/to/stdout.log",
				Stderr: "/path/to/stderr.log",
			},
		}

		err := oc.setupRemoteWriters(ctx, data, factory)
		require.NoError(t, err)

		// Verify writers were created
		assert.NotNil(t, oc.stdoutWriter)
		assert.NotNil(t, oc.stderrWriter)
		assert.Equal(t, "/path/to/stdout.log", oc.stdoutFileName)
		assert.Equal(t, "/path/to/stderr.log", oc.stderrFileName)

		// Verify stdout and stderr writers are different
		assert.NotSame(t, oc.stdoutWriter, oc.stderrWriter)
	})

	t.Run("MergesWritersWhenPathsMatch", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{}
		factory := newMockLogWriterFactory()
		ctx := context.Background()

		// Same path for both stdout and stderr
		data := NodeData{
			Step: core.Step{Name: "test-step"},
			State: NodeState{
				Stdout: "/path/to/combined.log",
				Stderr: "/path/to/combined.log",
			},
		}

		err := oc.setupRemoteWriters(ctx, data, factory)
		require.NoError(t, err)

		// When paths are the same, stderr should use the same writer as stdout
		assert.Same(t, oc.stdoutWriter, oc.stderrWriter)
	})
}

func TestOutputCoordinator_CapturedOutput(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsCachedResult", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{
			outputCaptured: true,
			outputData:     "cached output",
		}
		ctx := context.Background()

		output, err := oc.capturedOutput(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "cached output", output)
	})

	t.Run("ReturnsEmptyWhenNoCapture", func(t *testing.T) {
		t.Parallel()

		oc := &OutputCoordinator{}
		ctx := context.Background()

		output, err := oc.capturedOutput(ctx)
		assert.NoError(t, err)
		assert.Empty(t, output)
	})

	t.Run("CapturesFromOutputCapture", func(t *testing.T) {
		t.Parallel()

		// Create a pipe for output
		reader, writer, err := os.Pipe()
		require.NoError(t, err)

		oc := &OutputCoordinator{
			outputCapture: newOutputCapture(1024 * 1024),
			outputReader:  reader,
			outputWriter:  writer,
		}

		ctx := context.Background()

		// Start capturing
		oc.outputCapture.start(ctx, reader)

		// Write test data
		testData := "captured test output"
		_, err = writer.WriteString(testData)
		require.NoError(t, err)

		// Get captured output (this will close the writer)
		output, err := oc.capturedOutput(ctx)
		assert.NoError(t, err)
		assert.Equal(t, testData, output)

		// Verify caching works
		assert.True(t, oc.outputCaptured)
	})

	t.Run("AccumulatesOutputOnRetry", func(t *testing.T) {
		t.Parallel()

		// Create a pipe for output
		reader, writer, err := os.Pipe()
		require.NoError(t, err)

		oc := &OutputCoordinator{
			outputCapture: newOutputCapture(1024 * 1024),
			outputReader:  reader,
			outputWriter:  writer,
			outputData:    "previous output", // Simulating previous attempt
		}

		ctx := context.Background()

		// Start capturing
		oc.outputCapture.start(ctx, reader)

		// Write test data
		testData := "new output"
		_, err = writer.WriteString(testData)
		require.NoError(t, err)

		// Get captured output
		output, err := oc.capturedOutput(ctx)
		assert.NoError(t, err)

		// Should contain both previous and new output
		assert.Contains(t, output, "previous output")
		assert.Contains(t, output, "new output")
	})
}
