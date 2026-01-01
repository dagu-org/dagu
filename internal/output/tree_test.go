package output

import (
	"fmt"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/require"
)

func TestRenderDAGStatus_BasicSuccess(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:     core.Succeeded,
		StartedAt:  "2024-01-15 10:00:00",
		FinishedAt: "2024-01-15 10:01:00",
		Nodes: []*execution.Node{
			{
				Step:       core.Step{Name: "step1", Command: "echo", Args: []string{"hello"}},
				Status:     core.NodeSucceeded,
				StartedAt:  "2024-01-15 10:00:00",
				FinishedAt: "2024-01-15 10:00:30",
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Verify key elements
	require.Contains(t, output, "test-dag", "Output should contain DAG name")
	require.Contains(t, output, "step1", "Output should contain step name")
	require.Contains(t, output, "[succeeded]", "Output should contain succeeded label")
	require.Contains(t, output, "Result: Succeeded", "Output should contain succeeded result")
}

func TestRenderDAGStatus_FailedStep(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "failed-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Failed,
		Nodes: []*execution.Node{
			{
				Step:   core.Step{Name: "failing-step", Command: "exit", Args: []string{"1"}},
				Status: core.NodeFailed,
				Error:  "command exited with code 1",
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "[failed]", "Output should contain failed label")
	require.Contains(t, output, "error:", "Output should contain error label")
	require.Contains(t, output, "command exited with code 1", "Output should contain error message")
	require.Contains(t, output, "Result: Failed", "Output should contain failed result")
}

func TestRenderDAGStatus_MultipleSteps(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "multi-step-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{Step: core.Step{Name: "step1"}, Status: core.NodeSucceeded},
			{Step: core.Step{Name: "step2"}, Status: core.NodeSucceeded},
			{Step: core.Step{Name: "step3"}, Status: core.NodeSucceeded},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Verify tree structure characters
	require.Contains(t, output, "├─", "Output should contain branch characters for non-last steps")
	require.Contains(t, output, "└─", "Output should contain last branch character")

	// Verify all step names are present
	require.Contains(t, output, "step1", "Output should contain step1")
	require.Contains(t, output, "step2", "Output should contain step2")
	require.Contains(t, output, "step3", "Output should contain step3")
}

func TestRenderDAGStatus_RunningStatus(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "running-dag"}
	status := &execution.DAGRunStatus{
		Status:    core.Running,
		StartedAt: "2024-01-15 10:00:00",
		Nodes: []*execution.Node{
			{Step: core.Step{Name: "running-step"}, Status: core.NodeRunning, StartedAt: "2024-01-15 10:00:00"},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "[running]", "Output should contain running label")
	require.Contains(t, output, "Running", "Output should contain Running status")
}

func TestRenderDAGStatus_AbortedStatus(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "aborted-dag"}
	status := &execution.DAGRunStatus{
		Status:     core.Aborted,
		StartedAt:  "2024-01-15 10:00:00",
		FinishedAt: "2024-01-15 10:00:30",
		Nodes: []*execution.Node{
			{Step: core.Step{Name: "aborted-step"}, Status: core.NodeAborted},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "[aborted]", "Output should contain aborted label")
	require.Contains(t, output, "Result: Aborted", "Output should contain Aborted result")
}

func TestRenderDAGStatus_PartiallySucceededStatus(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "partial-dag"}
	status := &execution.DAGRunStatus{
		Status:     core.PartiallySucceeded,
		StartedAt:  "2024-01-15 10:00:00",
		FinishedAt: "2024-01-15 10:00:30",
		Nodes: []*execution.Node{
			{Step: core.Step{Name: "partial-step"}, Status: core.NodePartiallySucceeded},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "[partially_succeeded]", "Output should contain partial label")
	require.Contains(t, output, "Result: Partially Succeeded", "Output should contain Partially Succeeded result")
}

func TestRenderDAGStatus_QueuedStatus(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "queued-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Queued,
		Nodes:  []*execution.Node{},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "Queued", "Output should contain Queued status")
}

func TestRenderDAGStatus_NotStartedStatus(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "not-started-dag"}
	status := &execution.DAGRunStatus{
		Status: core.NotStarted,
		Nodes: []*execution.Node{
			{Step: core.Step{Name: "not-started-step"}, Status: core.NodeNotStarted},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "Not Started", "Output should contain Not Started status")
}

func TestRenderDAGStatus_SkippedStep(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "skipped-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{Step: core.Step{Name: "skipped-step"}, Status: core.NodeSkipped},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "[skipped]", "Output should contain skipped label")
}

func TestRenderDAGStatus_WithSubRuns(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "parent-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step:   core.Step{Name: "sub-step"},
				Status: core.NodeSucceeded,
				SubRuns: []execution.SubDAGRun{
					{DAGRunID: "sub-run-123", Params: "param1=value1"},
				},
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "subdag:", "Output should contain subdag label")
	require.Contains(t, output, "sub-run-123", "Output should contain sub-run ID")
	require.Contains(t, output, "param1=value1", "Output should contain sub-run params")
}

func TestRenderDAGStatus_WithSubRunsNoParams(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "parent-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step:   core.Step{Name: "sub-step"},
				Status: core.NodeSucceeded,
				SubRuns: []execution.SubDAGRun{
					{DAGRunID: "sub-run-456"},
				},
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "sub-run-456", "Output should contain sub-run ID")
}

func TestRenderDAGStatus_DisabledOutputs(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step:   core.Step{Name: "step1", Command: "echo"},
				Status: core.NodeSucceeded,
				Stdout: "/tmp/nonexistent-stdout.log",
				Stderr: "/tmp/nonexistent-stderr.log",
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false
	config.ShowStdout = false
	config.ShowStderr = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.NotContains(t, output, "stdout:", "Output should not contain stdout when disabled")
	require.NotContains(t, output, "stderr:", "Output should not contain stderr when disabled")
}

func TestRenderDAGStatus_WithActualLogFiles(t *testing.T) {
	t.Parallel()
	// Create temporary log files
	stdoutFile, err := os.CreateTemp("", "stdout-*.log")
	require.NoError(t, err)
	defer func() { _ = os.Remove(stdoutFile.Name()) }()
	_, _ = stdoutFile.WriteString("Hello from stdout\nLine 2\n")
	_ = stdoutFile.Close()

	stderrFile, err := os.CreateTemp("", "stderr-*.log")
	require.NoError(t, err)
	defer func() { _ = os.Remove(stderrFile.Name()) }()
	_, _ = stderrFile.WriteString("Warning: something happened\n")
	_ = stderrFile.Close()

	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:     core.Succeeded,
		StartedAt:  "2024-01-15 10:00:00",
		FinishedAt: "2024-01-15 10:00:30",
		Nodes: []*execution.Node{
			{
				Step:       core.Step{Name: "step1", Command: "echo"},
				Status:     core.NodeSucceeded,
				StartedAt:  "2024-01-15 10:00:00",
				FinishedAt: "2024-01-15 10:00:30",
				Stdout:     stdoutFile.Name(),
				Stderr:     stderrFile.Name(),
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "Hello from stdout", "Output should contain stdout content")
	require.Contains(t, output, "Warning: something happened", "Output should contain stderr content")
}

func TestRenderDAGStatus_WithTruncatedOutput(t *testing.T) {
	t.Parallel()
	// Create temporary log file with many lines
	stdoutFile, err := os.CreateTemp("", "stdout-*.log")
	require.NoError(t, err)
	defer func() { _ = os.Remove(stdoutFile.Name()) }()
	for i := 1; i <= 100; i++ {
		_, _ = fmt.Fprintf(stdoutFile, "Line %d\n", i)
	}
	_ = stdoutFile.Close()

	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step:   core.Step{Name: "step1"},
				Status: core.NodeSucceeded,
				Stdout: stdoutFile.Name(),
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false
	config.MaxOutputLines = 10

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "... (90 more lines)", "Output should contain truncation indicator")
	require.Contains(t, output, "Line 91", "Output should contain tail lines starting from Line 91")
}

func TestRenderDAGStatus_StartTimeFallback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		startedAt string
	}{
		{name: "empty", startedAt: ""},
		{name: "dash", startedAt: "-"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := &core.DAG{Name: "test-dag"}
			status := &execution.DAGRunStatus{
				Status:    core.NotStarted,
				StartedAt: tt.startedAt,
				Nodes:     []*execution.Node{},
			}

			config := DefaultConfig()
			config.ColorEnabled = false

			renderer := NewRenderer(config)
			output := renderer.RenderDAGStatus(dag, status)

			// Should use current time as fallback
			require.Contains(t, output, "dag: test-dag", "Output should contain DAG name")
		})
	}
}

func TestRenderDAGStatus_NoDuration(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:    core.NotStarted,
		StartedAt: "",
		Nodes:     []*execution.Node{},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Should not have duration in parentheses
	require.NotContains(t, output, "dag: test-dag (", "Output should not contain duration when not started")
}

func TestRenderDAGStatus_InvalidTimeFormat(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:     core.Succeeded,
		StartedAt:  "invalid-time",
		FinishedAt: "also-invalid",
		Nodes:      []*execution.Node{},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Should handle gracefully without duration
	require.Contains(t, output, "dag: test-dag", "Output should contain DAG name even with invalid times")
}

func TestReadLogFileTail_AllLines(t *testing.T) {
	t.Parallel()
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Write 10 lines
	for i := 1; i <= 10; i++ {
		_, _ = fmt.Fprintf(tmpfile, "Line %d\n", i)
	}
	_ = tmpfile.Close()

	// Test reading all lines (no limit)
	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 0)
	require.NoError(t, err)
	require.Len(t, lines, 10, "Expected 10 lines")
	require.Equal(t, 0, truncated, "Expected 0 truncated")
}

func TestReadLogFileTail_WithLimit(t *testing.T) {
	t.Parallel()
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Write 100 lines
	for i := 1; i <= 100; i++ {
		_, _ = fmt.Fprintf(tmpfile, "Line %d\n", i)
	}
	_ = tmpfile.Close()

	// Test reading last 10 lines
	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 10)
	require.NoError(t, err)
	require.Len(t, lines, 10, "Expected 10 lines")
	require.Equal(t, 90, truncated, "Expected 90 truncated")
	require.Equal(t, "Line 91", lines[0], "Expected first line to be 'Line 91'")
	require.Equal(t, "Line 100", lines[9], "Expected last line to be 'Line 100'")
}

func TestReadLogFileTail_NegativeLimit(t *testing.T) {
	t.Parallel()
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	_, _ = tmpfile.WriteString("Line 1\nLine 2\n")
	_ = tmpfile.Close()

	// Negative limit should return all lines
	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), -5)
	require.NoError(t, err)
	require.Len(t, lines, 2, "Expected 2 lines with negative limit")
	require.Equal(t, 0, truncated, "Expected 0 truncated")
}

func TestReadLogFileTail_EmptyFile(t *testing.T) {
	t.Parallel()
	// Create an empty temporary file
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }()
	_ = tmpfile.Close()

	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 10)
	require.NoError(t, err)
	require.Len(t, lines, 0, "Expected 0 lines for empty file")
	require.Equal(t, 0, truncated, "Expected 0 truncated")
}

func TestReadLogFileTail_NonexistentFile(t *testing.T) {
	t.Parallel()
	lines, truncated, err := ReadLogFileTail("/nonexistent/path/file.log", 10)
	require.NoError(t, err, "Should not return error for nonexistent file")
	require.Nil(t, lines, "Should return nil lines for nonexistent file")
	require.Equal(t, 0, truncated, "Expected 0 truncated")
}

func TestReadLogFileTail_EmptyPath(t *testing.T) {
	t.Parallel()
	lines, truncated, err := ReadLogFileTail("", 10)
	require.NoError(t, err, "Should not return error for empty path")
	require.Nil(t, lines, "Should return nil lines for empty path")
	require.Equal(t, 0, truncated, "Expected 0 truncated")
}

func TestReadLogFileTail_BinaryContent(t *testing.T) {
	t.Parallel()
	// Create a temporary file with binary content
	tmpfile, err := os.CreateTemp("", "test-log-*.bin")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Write binary content with null bytes
	_, _ = tmpfile.Write([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE})
	_ = tmpfile.Close()

	lines, _, err := ReadLogFileTail(tmpfile.Name(), 10)
	require.NoError(t, err)
	require.Len(t, lines, 1, "Expected 1 line for binary detection")
	require.Equal(t, "(binary data)", lines[0], "Expected '(binary data)'")
}

func TestReadLogFileTail_LargeFile(t *testing.T) {
	t.Parallel()
	tmpfile, err := os.CreateTemp("", "test-large-*.txt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Write more than 10MB
	data := make([]byte, 11*1024*1024)
	for i := range data {
		data[i] = 'x'
	}
	_, _ = tmpfile.Write(data)
	_ = tmpfile.Close()

	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 10)
	require.NoError(t, err)
	require.Equal(t, 0, truncated, "Expected truncated=0")
	require.Len(t, lines, 1, "Expected file too large message")
	require.Equal(t, "(file too large, >10MB)", lines[0], "Expected file too large message")
}

func TestStatusText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status core.Status
		text   string
	}{
		{core.Running, "Running"},
		{core.Succeeded, "Succeeded"},
		{core.Failed, "Failed"},
		{core.Aborted, "Aborted"},
		{core.PartiallySucceeded, "Partially Succeeded"},
		{core.Queued, "Queued"},
		{core.NotStarted, "Not Started"},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.text, StatusText(tt.status), "StatusText(%v) mismatch", tt.status)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	require.True(t, config.ColorEnabled, "ColorEnabled should be true by default")
	require.True(t, config.ShowStdout, "ShowStdout should be true by default")
	require.True(t, config.ShowStderr, "ShowStderr should be true by default")
	require.Equal(t, DefaultMaxOutputLines, config.MaxOutputLines,
		"MaxOutputLines should be %d by default", DefaultMaxOutputLines)
}

func TestRenderDAGStatus_TreeStructure(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "tree-test"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{Step: core.Step{Name: "first_step"}, Status: core.NodeSucceeded},
			{Step: core.Step{Name: "last_step"}, Status: core.NodeSucceeded},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false
	config.ShowStdout = false
	config.ShowStderr = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Check that tree structure is correct
	require.Contains(t, output, "├─first_step",
		"First step should use branch character '├─'")
	require.Contains(t, output, "└─last_step",
		"Last step should use last branch character '└─'")
}

func TestIsBinaryContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{"empty", []byte{}, false},
		{"text", []byte("hello world"), false},
		{"with null", []byte{0x00, 'a', 'b'}, true},
		{"high non-printable", []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a}, true},
		{"mostly text", []byte("hello\nworld\ttab"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, isBinaryContent(tt.data), "isBinaryContent(%q) mismatch", tt.data)
		})
	}
}

func TestRenderDAGStatus_WithMultipleCommands(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "multi-cmd-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step: core.Step{
					Name: "multi-step",
					Commands: []core.CommandEntry{
						{Command: "echo", Args: []string{"first"}, CmdWithArgs: "echo first"},
						{Command: "echo", Args: []string{"second"}, CmdWithArgs: "echo second"},
					},
				},
				Status: core.NodeSucceeded,
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false
	config.ShowStdout = false
	config.ShowStderr = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "echo first", "Output should contain first command")
	require.Contains(t, output, "echo second", "Output should contain second command")
}

func TestRenderDAGStatus_WithLegacyCommand(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "legacy-cmd-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step: core.Step{
					Name:        "legacy-step",
					CmdWithArgs: "echo hello world",
				},
				Status: core.NodeSucceeded,
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "echo hello world", "Output should contain legacy command")
}

func TestRenderDAGStatus_WithLegacyCommandAndArgs(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "legacy-cmd-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step: core.Step{
					Name:    "legacy-step",
					Command: "echo",
					Args:    []string{"hello", "world"},
				},
				Status: core.NodeSucceeded,
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "echo hello world", "Output should contain legacy command with args")
}

func TestRenderDAGStatus_NoCommand(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "no-cmd-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step:   core.Step{Name: "no-cmd-step"},
				Status: core.NodeSucceeded,
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "no-cmd-step", "Output should contain step name")
}

func TestTrimTrailingEmptyLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"no trailing", []string{"a", "b"}, []string{"a", "b"}},
		{"one trailing", []string{"a", "b", ""}, []string{"a", "b"}},
		{"multiple trailing", []string{"a", "", ""}, []string{"a"}},
		{"all empty", []string{"", "", ""}, []string{}},
		{"empty input", []string{}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimTrailingEmptyLines(tt.input)
			require.Equal(t, tt.expected, got, "trimTrailingEmptyLines() mismatch")
		})
	}
}

func TestNodeStatusToStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		nodeStatus core.NodeStatus
		expected   core.Status
	}{
		{core.NodeRunning, core.Running},
		{core.NodeSucceeded, core.Succeeded},
		{core.NodeFailed, core.Failed},
		{core.NodeAborted, core.Aborted},
		{core.NodePartiallySucceeded, core.PartiallySucceeded},
		{core.NodeSkipped, core.NotStarted},
		{core.NodeNotStarted, core.NotStarted},
		{core.NodeStatus(999), core.NotStarted}, // Unknown
	}

	for _, tt := range tests {
		t.Run(tt.nodeStatus.String(), func(t *testing.T) {
			require.Equal(t, tt.expected, nodeStatusToStatus(tt.nodeStatus),
				"nodeStatusToStatus(%v) mismatch", tt.nodeStatus)
		})
	}
}

func TestRenderDAGStatus_UnknownStatus(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "unknown-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Status(999), // Unknown status
		Nodes:  []*execution.Node{},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Should handle unknown status gracefully
	require.Contains(t, output, "Result:", "Output should contain result line")
}

func TestRenderDAGStatus_OnlyStdout(t *testing.T) {
	t.Parallel()
	stdoutFile, err := os.CreateTemp("", "stdout-*.log")
	require.NoError(t, err)
	defer func() { _ = os.Remove(stdoutFile.Name()) }()
	_, _ = stdoutFile.WriteString("Hello stdout\n")
	_ = stdoutFile.Close()

	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step:   core.Step{Name: "step1", Command: "echo"},
				Status: core.NodeSucceeded,
				Stdout: stdoutFile.Name(),
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false
	config.ShowStdout = true
	config.ShowStderr = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.Contains(t, output, "stdout:", "Output should contain stdout")
	require.NotContains(t, output, "stderr:", "Output should not contain stderr")
}

func TestRenderDAGStatus_OnlyStderr(t *testing.T) {
	t.Parallel()
	stderrFile, err := os.CreateTemp("", "stderr-*.log")
	require.NoError(t, err)
	defer func() { _ = os.Remove(stderrFile.Name()) }()
	_, _ = stderrFile.WriteString("Hello stderr\n")
	_ = stderrFile.Close()

	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step:   core.Step{Name: "step1", Command: "echo"},
				Status: core.NodeSucceeded,
				Stderr: stderrFile.Name(),
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false
	config.ShowStdout = false
	config.ShowStderr = true

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	require.NotContains(t, output, "stdout:", "Output should not contain stdout")
	require.Contains(t, output, "stderr:", "Output should contain stderr")
}

func TestReadLogFileTail_PermissionDenied(t *testing.T) {
	t.Parallel()
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test-perm-*.txt")
	require.NoError(t, err)
	tmpfileName := tmpfile.Name()
	_, _ = tmpfile.WriteString("test content\n")
	_ = tmpfile.Close()
	defer func() { _ = os.Remove(tmpfileName) }()

	// Remove read permissions
	if err := os.Chmod(tmpfileName, 0o000); err != nil {
		t.Skip("Cannot change permissions on this system")
	}
	defer func() { _ = os.Chmod(tmpfileName, 0o644) }()

	lines, truncated, err := ReadLogFileTail(tmpfileName, 10)
	// Should return an error for permission denied
	if err == nil {
		// Some systems (e.g., root user) can still read the file
		t.Skip("System allows reading file without read permission")
	}
	require.Nil(t, lines, "Should return nil lines on permission error")
	require.Equal(t, 0, truncated, "Should return 0 truncated on error")
}

func TestCalculateDuration_InvalidFinishedAt(t *testing.T) {
	t.Parallel()
	// Test when finishedAt is an invalid time string
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:     core.Succeeded,
		StartedAt:  "2024-01-15 10:00:00",
		FinishedAt: "invalid-time-format",
		Nodes:      []*execution.Node{},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Should still render, falling back to time.Now() for duration calculation
	require.Contains(t, output, "dag: test-dag", "Output should contain DAG name even with invalid finishedAt")
}

func TestCalculateDuration_NotRunningWithDashFinishedAt(t *testing.T) {
	t.Parallel()
	// Test when status is not Running and finishedAt is "-"
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:     core.Succeeded,
		StartedAt:  "2024-01-15 10:00:00",
		FinishedAt: "-",
		Nodes:      []*execution.Node{},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// When finishedAt is "-" and status is not Running, duration should be empty
	// So output should just have "dag: test-dag" without duration in parentheses
	require.Contains(t, output, "dag: test-dag", "Output should contain DAG name")
}

func TestCalculateDuration_NotRunningWithEmptyFinishedAt(t *testing.T) {
	t.Parallel()
	// Test when status is not Running and finishedAt is empty
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:     core.Succeeded,
		StartedAt:  "2024-01-15 10:00:00",
		FinishedAt: "",
		Nodes:      []*execution.Node{},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// When finishedAt is "" and status is not Running, duration should be empty
	require.Contains(t, output, "dag: test-dag", "Output should contain DAG name")
}

func TestRenderDAGStatus_NodeDurationWithInvalidTime(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status: core.Succeeded,
		Nodes: []*execution.Node{
			{
				Step:       core.Step{Name: "step1"},
				Status:     core.NodeSucceeded,
				StartedAt:  "2024-01-15 10:00:00",
				FinishedAt: "invalid-time",
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Should still render the step
	require.Contains(t, output, "step1", "Output should contain step name")
}

func TestRenderDAGStatus_RunningNodeCalculatesDuration(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:    core.Running,
		StartedAt: "2024-01-15 10:00:00",
		Nodes: []*execution.Node{
			{
				Step:      core.Step{Name: "running-step"},
				Status:    core.NodeRunning,
				StartedAt: "2024-01-15 10:00:00",
			},
		},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	// Running status should calculate duration using time.Now()
	require.Contains(t, output, "running-step", "Output should contain step name")
	// Duration should be present
	require.Contains(t, output, "(", "Output should contain duration for running step")
}

func TestCleanLogLine_CarriageReturn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "no carriage return",
			input:    "hello world",
			expected: []string{"hello world"},
		},
		{
			name:     "single carriage return",
			input:    "first\rsecond",
			expected: []string{"second"},
		},
		{
			name:     "multiple carriage returns",
			input:    "a\rb\rc\rd",
			expected: []string{"d"},
		},
		{
			name:     "curl progress bar",
			input:    "  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0\r100   123  100   123    0     0   1000      0 --:--:-- --:--:-- --:--:--  1000",
			expected: []string{"100   123  100   123    0     0   1000      0 --:--:-- --:--:-- --:--:--  1000"},
		},
		{
			name:     "empty after carriage return",
			input:    "something\r",
			expected: []string{"something"},
		},
		{
			name:     "all empty segments",
			input:    "\r\r\r",
			expected: nil,
		},
		{
			name:     "empty line",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanLogLine(tt.input)
			require.Equal(t, tt.expected, got, "cleanLogLine(%q) mismatch", tt.input)
		})
	}
}

func TestCleanControlChars(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal text", "hello world", "hello world"},
		{"with tab", "hello\tworld", "hello\tworld"},
		{"with bell char", "hello\x07world", "helloworld"},
		{"with escape", "hello\x1bworld", "helloworld"},
		{"with backspace", "hello\x08world", "helloworld"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanControlChars(tt.input)
			require.Equal(t, tt.expected, got, "cleanControlChars(%q) mismatch", tt.input)
		})
	}
}

func TestCleanErrorMessage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no stderr tail",
			input:    "command failed: exit status 1",
			expected: "command failed: exit status 1",
		},
		{
			name:     "with stderr tail newline prefix",
			input:    "command failed: exit status 1\nrecent stderr (tail):\nsome error output",
			expected: "command failed: exit status 1",
		},
		{
			name:     "with stderr tail no newline prefix",
			input:    "failed to execute: exit status 56recent stderr (tail):\nerror details",
			expected: "failed to execute: exit status 56",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanErrorMessage(tt.input)
			require.Equal(t, tt.expected, got, "cleanErrorMessage() mismatch")
		})
	}
}

func TestReadLogFileTail_WithCarriageReturns(t *testing.T) {
	t.Parallel()
	// Create a temporary file with curl-like progress output
	tmpfile, err := os.CreateTemp("", "test-curl-*.txt")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Write curl-like progress output
	_, _ = tmpfile.WriteString("  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0\r100   123  100   123    0     0   1000      0 --:--:-- --:--:-- --:--:--  1000\n")
	_, _ = tmpfile.WriteString("Downloaded file.txt\n")
	_ = tmpfile.Close()

	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 0)
	require.NoError(t, err)
	require.Equal(t, 0, truncated, "Expected 0 truncated")
	// Should have cleaned up the progress line
	require.Len(t, lines, 2, "Expected 2 lines")
	// First line should be the final progress state (after \r)
	require.Contains(t, lines[0], "100", "First line should contain '100' (final progress)")
}
