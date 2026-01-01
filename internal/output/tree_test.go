package output

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
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
	if !strings.Contains(output, "test-dag") {
		t.Error("Output should contain DAG name")
	}
	if !strings.Contains(output, "step1") {
		t.Error("Output should contain step name")
	}
	if !strings.Contains(output, "[passed]") {
		t.Error("Output should contain passed label")
	}
	if !strings.Contains(output, "Result: Passed") {
		t.Error("Output should contain passed result")
	}
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

	if !strings.Contains(output, "[failed]") {
		t.Error("Output should contain failed label")
	}
	if !strings.Contains(output, "error:") {
		t.Error("Output should contain error label")
	}
	if !strings.Contains(output, "command exited with code 1") {
		t.Error("Output should contain error message")
	}
	if !strings.Contains(output, "Result: Failed") {
		t.Error("Output should contain failed result")
	}
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
	if !strings.Contains(output, "├─") {
		t.Error("Output should contain branch characters for non-last steps")
	}
	if !strings.Contains(output, "└─") {
		t.Error("Output should contain last branch character")
	}

	// Verify all step names are present
	if !strings.Contains(output, "step1") {
		t.Error("Output should contain step1")
	}
	if !strings.Contains(output, "step2") {
		t.Error("Output should contain step2")
	}
	if !strings.Contains(output, "step3") {
		t.Error("Output should contain step3")
	}
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

	if !strings.Contains(output, "[running]") {
		t.Error("Output should contain running label")
	}
	if !strings.Contains(output, "Running") {
		t.Error("Output should contain Running status")
	}
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

	if !strings.Contains(output, "[canceled]") {
		t.Error("Output should contain canceled label")
	}
	if !strings.Contains(output, "Canceled") {
		t.Error("Output should contain Canceled result")
	}
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

	if !strings.Contains(output, "[partial]") {
		t.Error("Output should contain partial label")
	}
	if !strings.Contains(output, "Result: Partial") {
		t.Error("Output should contain Partial result")
	}
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

	if !strings.Contains(output, "Queued") {
		t.Error("Output should contain Queued status")
	}
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

	if !strings.Contains(output, "Not Started") {
		t.Error("Output should contain Not Started status")
	}
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

	if !strings.Contains(output, "[skipped]") {
		t.Error("Output should contain skipped label")
	}
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

	if !strings.Contains(output, "subdag:") {
		t.Error("Output should contain subdag label")
	}
	if !strings.Contains(output, "sub-run-123") {
		t.Error("Output should contain sub-run ID")
	}
	if !strings.Contains(output, "param1=value1") {
		t.Error("Output should contain sub-run params")
	}
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

	if !strings.Contains(output, "sub-run-456") {
		t.Error("Output should contain sub-run ID")
	}
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

	if strings.Contains(output, "stdout:") {
		t.Error("Output should not contain stdout when disabled")
	}
	if strings.Contains(output, "stderr:") {
		t.Error("Output should not contain stderr when disabled")
	}
}

func TestRenderDAGStatus_WithActualLogFiles(t *testing.T) {
	t.Parallel()
	// Create temporary log files
	stdoutFile, err := os.CreateTemp("", "stdout-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(stdoutFile.Name())
	_, _ = stdoutFile.WriteString("Hello from stdout\nLine 2\n")
	stdoutFile.Close()

	stderrFile, err := os.CreateTemp("", "stderr-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(stderrFile.Name())
	_, _ = stderrFile.WriteString("Warning: something happened\n")
	stderrFile.Close()

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

	if !strings.Contains(output, "Hello from stdout") {
		t.Error("Output should contain stdout content")
	}
	if !strings.Contains(output, "Warning: something happened") {
		t.Error("Output should contain stderr content")
	}
}

func TestRenderDAGStatus_WithTruncatedOutput(t *testing.T) {
	t.Parallel()
	// Create temporary log file with many lines
	stdoutFile, err := os.CreateTemp("", "stdout-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(stdoutFile.Name())
	for i := 1; i <= 100; i++ {
		_, _ = stdoutFile.WriteString(fmt.Sprintf("Line %d\n", i))
	}
	stdoutFile.Close()

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

	if !strings.Contains(output, "... (90 more lines)") {
		t.Error("Output should contain truncation indicator")
	}
	if !strings.Contains(output, "Line 91") {
		t.Error("Output should contain tail lines starting from Line 91")
	}
}

func TestRenderDAGStatus_EmptyStartTime(t *testing.T) {
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

	// Should use current time as fallback
	if !strings.Contains(output, "dag: test-dag") {
		t.Error("Output should contain DAG name")
	}
}

func TestRenderDAGStatus_DashStartTime(t *testing.T) {
	t.Parallel()
	dag := &core.DAG{Name: "test-dag"}
	status := &execution.DAGRunStatus{
		Status:    core.NotStarted,
		StartedAt: "-",
		Nodes:     []*execution.Node{},
	}

	config := DefaultConfig()
	config.ColorEnabled = false

	renderer := NewRenderer(config)
	output := renderer.RenderDAGStatus(dag, status)

	if !strings.Contains(output, "dag: test-dag") {
		t.Error("Output should contain DAG name")
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
	if strings.Contains(output, "dag: test-dag (") {
		t.Error("Output should not contain duration when not started")
	}
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
	if !strings.Contains(output, "dag: test-dag") {
		t.Error("Output should contain DAG name even with invalid times")
	}
}

func TestReadLogFileTail_AllLines(t *testing.T) {
	t.Parallel()
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write 10 lines
	for i := 1; i <= 10; i++ {
		_, _ = tmpfile.WriteString(fmt.Sprintf("Line %d\n", i))
	}
	tmpfile.Close()

	// Test reading all lines (no limit)
	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(lines) != 10 {
		t.Errorf("Expected 10 lines, got %d", len(lines))
	}
	if truncated != 0 {
		t.Errorf("Expected 0 truncated, got %d", truncated)
	}
}

func TestReadLogFileTail_WithLimit(t *testing.T) {
	t.Parallel()
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write 100 lines
	for i := 1; i <= 100; i++ {
		_, _ = tmpfile.WriteString(fmt.Sprintf("Line %d\n", i))
	}
	tmpfile.Close()

	// Test reading last 10 lines
	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 10)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(lines) != 10 {
		t.Errorf("Expected 10 lines, got %d", len(lines))
	}
	if truncated != 90 {
		t.Errorf("Expected 90 truncated, got %d", truncated)
	}
	if lines[0] != "Line 91" {
		t.Errorf("Expected first line to be 'Line 91', got '%s'", lines[0])
	}
	if lines[9] != "Line 100" {
		t.Errorf("Expected last line to be 'Line 100', got '%s'", lines[9])
	}
}

func TestReadLogFileTail_NegativeLimit(t *testing.T) {
	t.Parallel()
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	_, _ = tmpfile.WriteString("Line 1\nLine 2\n")
	tmpfile.Close()

	// Negative limit should return all lines
	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), -5)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines with negative limit, got %d", len(lines))
	}
	if truncated != 0 {
		t.Errorf("Expected 0 truncated, got %d", truncated)
	}
}

func TestReadLogFileTail_EmptyFile(t *testing.T) {
	t.Parallel()
	// Create an empty temporary file
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 10)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("Expected 0 lines for empty file, got %d", len(lines))
	}
	if truncated != 0 {
		t.Errorf("Expected 0 truncated, got %d", truncated)
	}
}

func TestReadLogFileTail_NonexistentFile(t *testing.T) {
	t.Parallel()
	lines, truncated, err := ReadLogFileTail("/nonexistent/path/file.log", 10)
	if err != nil {
		t.Fatalf("Should not return error for nonexistent file, got: %v", err)
	}
	if lines != nil {
		t.Error("Should return nil lines for nonexistent file")
	}
	if truncated != 0 {
		t.Errorf("Expected 0 truncated, got %d", truncated)
	}
}

func TestReadLogFileTail_EmptyPath(t *testing.T) {
	t.Parallel()
	lines, truncated, err := ReadLogFileTail("", 10)
	if err != nil {
		t.Fatalf("Should not return error for empty path, got: %v", err)
	}
	if lines != nil {
		t.Error("Should return nil lines for empty path")
	}
	if truncated != 0 {
		t.Errorf("Expected 0 truncated, got %d", truncated)
	}
}

func TestReadLogFileTail_BinaryContent(t *testing.T) {
	t.Parallel()
	// Create a temporary file with binary content
	tmpfile, err := os.CreateTemp("", "test-log-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write binary content with null bytes
	_, _ = tmpfile.Write([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE})
	tmpfile.Close()

	lines, _, err := ReadLogFileTail(tmpfile.Name(), 10)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(lines) != 1 {
		t.Errorf("Expected 1 line for binary detection, got %d", len(lines))
	}
	if lines[0] != "(binary data)" {
		t.Errorf("Expected '(binary data)', got '%s'", lines[0])
	}
}

func TestReadLogFileAll(t *testing.T) {
	t.Parallel()
	tmpfile, err := os.CreateTemp("", "test-log-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	_, _ = tmpfile.WriteString("Line 1\nLine 2\nLine 3\n")
	tmpfile.Close()

	lines, err := ReadLogFileAll(tmpfile.Name())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
}

func TestReadLogFileAll_EmptyPath(t *testing.T) {
	t.Parallel()
	lines, err := ReadLogFileAll("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if lines != nil {
		t.Error("Should return nil for empty path")
	}
}

func TestStatusSymbol(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status core.Status
		symbol string
	}{
		{core.Running, "●"},
		{core.Succeeded, "✓"},
		{core.Failed, "✗"},
		{core.Aborted, "⚠"},
		{core.PartiallySucceeded, "◐"},
		{core.Queued, "◌"},
		{core.NotStarted, "○"},
		{core.Status(999), "○"}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := StatusSymbol(tt.status); got != tt.symbol {
				t.Errorf("StatusSymbol(%v) = %s, want %s", tt.status, got, tt.symbol)
			}
		})
	}
}

func TestNodeStatusSymbol(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status core.NodeStatus
		symbol string
	}{
		{core.NodeRunning, "●"},
		{core.NodeSucceeded, "✓"},
		{core.NodeFailed, "✗"},
		{core.NodeAborted, "⚠"},
		{core.NodeSkipped, "○"},
		{core.NodePartiallySucceeded, "◐"},
		{core.NodeNotStarted, "○"},
		{core.NodeStatus(999), "○"}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			if got := NodeStatusSymbol(tt.status); got != tt.symbol {
				t.Errorf("NodeStatusSymbol(%v) = %s, want %s", tt.status, got, tt.symbol)
			}
		})
	}
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
			if got := StatusText(tt.status); got != tt.text {
				t.Errorf("StatusText(%v) = %s, want %s", tt.status, got, tt.text)
			}
		})
	}
}

func TestStatusColorize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status core.Status
	}{
		{core.Running},
		{core.Succeeded},
		{core.Failed},
		{core.Aborted},
		{core.PartiallySucceeded},
		{core.Queued},
		{core.NotStarted},
		{core.Status(999)}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			result := StatusColorize("test", tt.status)
			// Just verify it returns something - actual color testing would require more setup
			if result == "" {
				t.Errorf("StatusColorize(%v) returned empty string", tt.status)
			}
		})
	}
}

func TestNodeStatusColorize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status core.NodeStatus
	}{
		{core.NodeRunning},
		{core.NodeSucceeded},
		{core.NodeFailed},
		{core.NodeAborted},
		{core.NodeSkipped},
		{core.NodePartiallySucceeded},
		{core.NodeNotStarted},
		{core.NodeStatus(999)}, // Unknown status
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			result := NodeStatusColorize("test", tt.status)
			// Just verify it returns something
			if result == "" {
				t.Errorf("NodeStatusColorize(%v) returned empty string", tt.status)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	if !config.ColorEnabled {
		t.Error("ColorEnabled should be true by default")
	}
	if !config.ShowStdout {
		t.Error("ShowStdout should be true by default")
	}
	if !config.ShowStderr {
		t.Error("ShowStderr should be true by default")
	}
	if config.MaxOutputLines != DefaultMaxOutputLines {
		t.Errorf("MaxOutputLines should be %d by default, got %d",
			DefaultMaxOutputLines, config.MaxOutputLines)
	}
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
	if !strings.Contains(output, "├─first_step") {
		t.Errorf("First step should use branch character '├─', got output:\n%s", output)
	}
	if !strings.Contains(output, "└─last_step") {
		t.Errorf("Last step should use last branch character '└─', got output:\n%s", output)
	}
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
			if got := isBinaryContent(tt.data); got != tt.expected {
				t.Errorf("isBinaryContent(%q) = %v, want %v", tt.data, got, tt.expected)
			}
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

	if !strings.Contains(output, "echo first") {
		t.Error("Output should contain first command")
	}
	if !strings.Contains(output, "echo second") {
		t.Error("Output should contain second command")
	}
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

	if !strings.Contains(output, "echo hello world") {
		t.Error("Output should contain legacy command")
	}
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

	if !strings.Contains(output, "echo hello world") {
		t.Error("Output should contain legacy command with args")
	}
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

	if !strings.Contains(output, "no-cmd-step") {
		t.Error("Output should contain step name")
	}
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
			if len(got) != len(tt.expected) {
				t.Errorf("trimTrailingEmptyLines() len = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("trimTrailingEmptyLines()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
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
			if got := nodeStatusToStatus(tt.nodeStatus); got != tt.expected {
				t.Errorf("nodeStatusToStatus(%v) = %v, want %v", tt.nodeStatus, got, tt.expected)
			}
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
	if !strings.Contains(output, "Status:") {
		t.Error("Output should contain status line")
	}
}

func TestRenderDAGStatus_OnlyStdout(t *testing.T) {
	t.Parallel()
	stdoutFile, err := os.CreateTemp("", "stdout-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(stdoutFile.Name())
	_, _ = stdoutFile.WriteString("Hello stdout\n")
	stdoutFile.Close()

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

	if !strings.Contains(output, "stdout:") {
		t.Error("Output should contain stdout")
	}
	if strings.Contains(output, "stderr:") {
		t.Error("Output should not contain stderr")
	}
}

func TestRenderDAGStatus_OnlyStderr(t *testing.T) {
	t.Parallel()
	stderrFile, err := os.CreateTemp("", "stderr-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(stderrFile.Name())
	_, _ = stderrFile.WriteString("Hello stderr\n")
	stderrFile.Close()

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

	if strings.Contains(output, "stdout:") {
		t.Error("Output should not contain stdout")
	}
	if !strings.Contains(output, "stderr:") {
		t.Error("Output should contain stderr")
	}
}

func TestReadLogFileTail_PermissionDenied(t *testing.T) {
	t.Parallel()
	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "test-perm-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpfileName := tmpfile.Name()
	_, _ = tmpfile.WriteString("test content\n")
	tmpfile.Close()
	defer os.Remove(tmpfileName)

	// Remove read permissions
	if err := os.Chmod(tmpfileName, 0o000); err != nil {
		t.Skip("Cannot change permissions on this system")
	}
	defer os.Chmod(tmpfileName, 0o644)

	lines, truncated, err := ReadLogFileTail(tmpfileName, 10)
	// Should return an error for permission denied
	if err == nil {
		// Some systems (e.g., root user) can still read the file
		t.Skip("System allows reading file without read permission")
	}
	if lines != nil {
		t.Error("Should return nil lines on permission error")
	}
	if truncated != 0 {
		t.Error("Should return 0 truncated on error")
	}
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
	if !strings.Contains(output, "dag: test-dag") {
		t.Error("Output should contain DAG name even with invalid finishedAt")
	}
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
	if !strings.Contains(output, "dag: test-dag") {
		t.Error("Output should contain DAG name")
	}
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
	if !strings.Contains(output, "dag: test-dag") {
		t.Error("Output should contain DAG name")
	}
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
	if !strings.Contains(output, "step1") {
		t.Error("Output should contain step name")
	}
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
	if !strings.Contains(output, "running-step") {
		t.Error("Output should contain step name")
	}
	// Duration should be present
	if !strings.Contains(output, "(") {
		t.Error("Output should contain duration for running step")
	}
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
			if len(got) != len(tt.expected) {
				t.Errorf("cleanLogLine(%q) len = %d, want %d", tt.input, len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("cleanLogLine(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
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
			if got != tt.expected {
				t.Errorf("cleanControlChars(%q) = %q, want %q", tt.input, got, tt.expected)
			}
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
			if got != tt.expected {
				t.Errorf("cleanErrorMessage() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestReadLogFileTail_WithCarriageReturns(t *testing.T) {
	t.Parallel()
	// Create a temporary file with curl-like progress output
	tmpfile, err := os.CreateTemp("", "test-curl-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Write curl-like progress output
	_, _ = tmpfile.WriteString("  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0\r100   123  100   123    0     0   1000      0 --:--:-- --:--:-- --:--:--  1000\n")
	_, _ = tmpfile.WriteString("Downloaded file.txt\n")
	tmpfile.Close()

	lines, truncated, err := ReadLogFileTail(tmpfile.Name(), 0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if truncated != 0 {
		t.Errorf("Expected 0 truncated, got %d", truncated)
	}
	// Should have cleaned up the progress line
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %v", len(lines), lines)
	}
	// First line should be the final progress state (after \r)
	if len(lines) > 0 && !strings.Contains(lines[0], "100") {
		t.Errorf("First line should contain '100' (final progress), got: %s", lines[0])
	}
}
