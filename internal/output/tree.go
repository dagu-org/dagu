package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// Tree drawing characters using Unicode box-drawing characters.
const (
	TreeBranch     = "├─" // Branch with siblings below
	TreeLastBranch = "└─" // Last branch (no siblings below)
	TreePipe       = "│ " // Vertical continuation line
	TreeSpace      = "  " // Empty space (no vertical line needed)
)

// DefaultMaxOutputLines is the default number of lines to show for stdout/stderr.
// Only the last N lines are shown (tail) when output exceeds this limit.
const DefaultMaxOutputLines = 50

// DefaultMaxWidth is the maximum line width before wrapping.
const DefaultMaxWidth = 80

// Config holds configuration for tree rendering.
type Config struct {
	// ColorEnabled enables colored output using ANSI escape codes.
	// Should be auto-detected based on terminal capability.
	ColorEnabled bool

	// ShowStdout enables display of stdout content in the tree.
	ShowStdout bool

	// ShowStderr enables display of stderr content in the tree.
	ShowStderr bool

	// MaxOutputLines limits stdout/stderr to last N lines (tail).
	// Set to 0 for unlimited output.
	MaxOutputLines int

	// MaxWidth is the maximum line width before wrapping.
	// Set to 0 for no wrapping.
	MaxWidth int
}

// DefaultConfig returns the default configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		ColorEnabled:   true,
		ShowStdout:     true,
		ShowStderr:     true,
		MaxOutputLines: DefaultMaxOutputLines,
		MaxWidth:       DefaultMaxWidth,
	}
}

// Renderer renders DAG execution status as a tree structure.
type Renderer struct {
	config Config
}

// text returns text with a soft light blue color for visual distinction.
func (r *Renderer) text(s string) string {
	if !r.config.ColorEnabled {
		return s
	}
	// Use ANSI 256 color 110 - a soft, muted light blue
	return "\033[38;5;110m" + s + "\033[0m"
}

// gray returns text in gray color for secondary information like duration.
func (r *Renderer) gray(s string) string {
	if !r.config.ColorEnabled {
		return s
	}
	// Use ANSI 256 color 245 - a medium gray
	return "\033[38;5;245m" + s + "\033[0m"
}

// NewRenderer creates a new tree renderer with the given configuration.
func NewRenderer(config Config) *Renderer {
	return &Renderer{config: config}
}

// RenderDAGStatus renders the complete DAG status as a pipelight-style tree.
// The output format is:
//
//	● Running - 2024-01-15 16:52:58
//	dag: my_dag (6s 619ms)
//	├─step: build (4s)
//	│ ├─echo hello
//	│ │ ├─stdout: hello
//	│ │ └─stderr:
//	│ └─result: ✓
//	└─step: test (2s)
//	  └─npm test
//	    ├─stdout: All tests passed
//	    └─stderr:
func (r *Renderer) RenderDAGStatus(dag *core.DAG, status *exec.DAGRunStatus) string {
	var buf strings.Builder

	// Header line
	buf.WriteString(r.renderHeader(status))
	buf.WriteString("\n\n")

	// DAG line
	buf.WriteString(r.renderDAGLine(dag, status))
	buf.WriteString("\n")

	// Tree continuation after DAG line (if there are steps)
	if len(status.Nodes) > 0 {
		buf.WriteString("│\n")
	}

	// Render each step as a tree branch
	for i, node := range status.Nodes {
		isLast := i == len(status.Nodes)-1
		buf.WriteString(r.renderStep(node, isLast, ""))
	}

	// Final status line
	buf.WriteString(r.renderFinalStatus(status))

	return buf.String()
}

// renderHeader renders the status header line with status text and timestamp.
// White color (default).
func (r *Renderer) renderHeader(status *exec.DAGRunStatus) string {
	statusText := StatusText(status.Status)

	// Parse start time, fallback to current time if not set
	startTime := status.StartedAt
	if startTime == "" || startTime == "-" {
		startTime = time.Now().Format("2006-01-02 15:04:05")
	}

	return fmt.Sprintf("%s - %s", statusText, startTime)
}

// renderDAGLine renders the DAG name with total duration.
// White color (default) - root of the tree.
func (r *Renderer) renderDAGLine(dag *core.DAG, status *exec.DAGRunStatus) string {
	duration := r.calculateDuration(status.StartedAt, status.FinishedAt, status.Status)

	if duration != "" {
		return fmt.Sprintf("dag: %s %s", dag.Name, r.gray("("+duration+")"))
	}
	return fmt.Sprintf("dag: %s", dag.Name)
}

// renderStep renders a single step with its commands and output content.
func (r *Renderer) renderStep(node *exec.Node, isLast bool, prefix string) string {
	var buf strings.Builder

	// Determine branch character based on position
	branch := TreeBranch
	if isLast {
		branch = TreeLastBranch
	}

	// Build step line with separate coloring for name and duration
	var lineParts []string
	lineParts = append(lineParts, r.text(node.Step.Name))

	// Add duration for steps that ran (in gray)
	if node.Status == core.NodeSucceeded || node.Status == core.NodeFailed ||
		node.Status == core.NodeRunning || node.Status == core.NodePartiallySucceeded {
		duration := r.calculateNodeDuration(node)
		if duration != "" {
			lineParts = append(lineParts, r.gray("("+duration+")"))
		}
	}

	// Add status label
	statusLabel := r.getStatusLabel(node.Status)
	if statusLabel != "" {
		lineParts = append(lineParts, r.text(statusLabel))
	}

	buf.WriteString(prefix + branch + strings.Join(lineParts, " "))
	buf.WriteString("\n")

	// For skipped/aborted/not-started steps, don't show details
	if node.Status == core.NodeSkipped || node.Status == core.NodeAborted || node.Status == core.NodeNotStarted {
		// Add spacing after step (with tree continuation if not last)
		if !isLast {
			buf.WriteString(prefix + TreePipe + "\n")
		}
		return buf.String()
	}

	// Calculate prefix for child elements
	childPrefix := prefix
	if isLast {
		childPrefix += TreeSpace
	} else {
		childPrefix += TreePipe
	}

	// Check what child elements we have
	hasOutput := r.hasOutput(node)
	hasError := node.Error != "" && node.Status == core.NodeFailed
	hasSubRuns := len(node.SubRuns) > 0

	// Count remaining fields for proper spacing
	fieldCount := 0
	if hasOutput {
		fieldCount++
	}
	if hasSubRuns {
		fieldCount++
	}
	if hasError {
		fieldCount++
	}

	// Track if we've written any field (for spacing)
	wroteField := false
	remainingFields := fieldCount

	// Helper to add vertical spacing between fields with tree continuation
	addFieldSpacing := func() {
		if wroteField && remainingFields > 0 {
			// Show tree continuation: childPrefix + "│" for connected look
			buf.WriteString(childPrefix + "│\n")
		}
		wroteField = true
	}

	// Render commands
	commands := node.Step.Commands
	hasCommands := len(commands) > 0

	if !hasCommands {
		// Handle legacy single command format
		cmdStr := r.getLegacyCommand(node)
		if cmdStr != "" {
			isLastChild := !hasOutput && !hasError && !hasSubRuns
			buf.WriteString(r.renderCommandLine(cmdStr, isLastChild, childPrefix))
			wroteField = true
		}
	} else {
		// Modern multi-command format
		for i, cmd := range commands {
			isLastCmd := i == len(commands)-1 && !hasOutput && !hasError && !hasSubRuns
			buf.WriteString(r.renderCommandLine(cmd.String(), isLastCmd, childPrefix))
		}
		wroteField = true
	}

	// Render stdout/stderr
	if hasOutput {
		addFieldSpacing()
		remainingFields--
		isLastOutput := !hasError && !hasSubRuns
		buf.WriteString(r.renderOutputs(node, isLastOutput, childPrefix))
	}

	// Render sub-DAG runs if present
	if hasSubRuns {
		addFieldSpacing()
		remainingFields--
		isLastSubRuns := !hasError
		buf.WriteString(r.renderSubRuns(node.SubRuns, isLastSubRuns, childPrefix))
	}

	// Render error message if step failed
	if hasError {
		addFieldSpacing()
		remainingFields--
		buf.WriteString(r.renderError(node.Error, childPrefix))
	}

	// Add spacing after step (with tree continuation if not last step)
	if !isLast {
		buf.WriteString(prefix + TreePipe + "\n")
	}

	return buf.String()
}

// getStatusLabel returns a text label for the node status.
// All labels are plain text - no colors.
func (r *Renderer) getStatusLabel(status core.NodeStatus) string {
	if status == core.NodeNotStarted {
		return ""
	}
	return "[" + status.String() + "]"
}

// hasOutput checks if the node has any stdout or stderr content.
func (r *Renderer) hasOutput(node *exec.Node) bool {
	if !r.config.ShowStdout && !r.config.ShowStderr {
		return false
	}
	if r.config.ShowStdout && node.Stdout != "" {
		lines, _, _ := ReadLogFileTail(node.Stdout, 1)
		if len(lines) > 0 {
			return true
		}
	}
	if r.config.ShowStderr && node.Stderr != "" {
		lines, _, _ := ReadLogFileTail(node.Stderr, 1)
		if len(lines) > 0 {
			return true
		}
	}
	return false
}

// renderCommandLine renders a single command line with optional wrapping.
func (r *Renderer) renderCommandLine(cmdStr string, isLast bool, prefix string) string {
	branch := TreeBranch
	if isLast {
		branch = TreeLastBranch
	}

	// Apply wrapping if configured
	if r.config.MaxWidth > 0 {
		prefixLen := len(prefix) + len(branch)
		maxContentWidth := r.config.MaxWidth - prefixLen
		if maxContentWidth > 20 && len(cmdStr) > maxContentWidth {
			return r.renderWrappedLine(cmdStr, branch, isLast, prefix)
		}
	}

	// Tree lines faint, text sepia
	return prefix + branch + r.text(cmdStr) + "\n"
}

// renderWrappedLine renders a line with word wrapping.
func (r *Renderer) renderWrappedLine(text string, branch string, isLast bool, prefix string) string {
	var buf strings.Builder

	prefixLen := len(prefix) + len(branch)
	maxContentWidth := r.config.MaxWidth - prefixLen
	if maxContentWidth < 20 {
		maxContentWidth = 20
	}

	// Calculate continuation prefix (tree lines faint)
	contPrefix := prefix
	if isLast {
		contPrefix += TreeSpace
	} else {
		contPrefix += TreePipe
	}

	lines := wrapText(text, maxContentWidth)
	for i, line := range lines {
		if i == 0 {
			// First line: tree branch faint, text sepia
			buf.WriteString(prefix + branch + r.text(line) + "\n")
		} else {
			// Continuation lines: indented, text sepia
			buf.WriteString(contPrefix + "  " + r.text(line) + "\n")
		}
	}

	return buf.String()
}

// wrapText wraps text at word boundaries to fit within maxWidth.
func wrapText(text string, maxWidth int) []string {
	if len(text) <= maxWidth {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	var currentLine strings.Builder

	for _, word := range words {
		if len(word) > maxWidth {
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}
			for len(word) > maxWidth {
				lines = append(lines, word[:maxWidth])
				word = word[maxWidth:]
			}
			if len(word) > 0 {
				currentLine.WriteString(word)
			}
			continue
		}

		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= maxWidth {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// getLegacyCommand extracts command string from legacy step format.
func (r *Renderer) getLegacyCommand(node *exec.Node) string {
	if node.Step.CmdWithArgs != "" {
		return node.Step.CmdWithArgs
	}
	if node.Step.Command != "" {
		cmd := node.Step.Command
		if len(node.Step.Args) > 0 {
			cmd += " " + strings.Join(node.Step.Args, " ")
		}
		return cmd
	}
	return ""
}

// renderOutputs renders stdout and stderr for a node.
func (r *Renderer) renderOutputs(node *exec.Node, isLast bool, prefix string) string {
	var buf strings.Builder

	// Check which outputs have content
	hasStdoutContent := false
	hasStderrContent := false

	if r.config.ShowStdout && node.Stdout != "" {
		lines, _, _ := ReadLogFileTail(node.Stdout, 1)
		hasStdoutContent = len(lines) > 0
	}
	if r.config.ShowStderr && node.Stderr != "" {
		lines, _, _ := ReadLogFileTail(node.Stderr, 1)
		hasStderrContent = len(lines) > 0
	}

	if hasStdoutContent {
		isLastOutput := isLast && !hasStderrContent
		buf.WriteString(r.renderOutput("stdout", node.Stdout, isLastOutput, prefix))
	}

	if hasStderrContent {
		buf.WriteString(r.renderOutput("stderr", node.Stderr, isLast, prefix))
	}

	return buf.String()
}

// renderOutput renders a single output stream (stdout or stderr) with content.
func (r *Renderer) renderOutput(label string, filePath string, isLast bool, prefix string) string {
	// Read log content with tail limit
	lines, truncated, err := ReadLogFileTail(filePath, r.config.MaxOutputLines)

	// Skip empty output entirely - don't show empty labels
	if err != nil || len(lines) == 0 {
		return ""
	}

	var buf strings.Builder

	branch := TreeBranch
	if isLast {
		branch = TreeLastBranch
	}

	// Calculate content prefix for multi-line output
	contentPrefix := prefix
	if isLast {
		contentPrefix += TreeSpace
	} else {
		contentPrefix += TreePipe
	}

	// Calculate max width for content wrapping
	contentIndent := len(contentPrefix) + 2 // "  " indent
	maxContentWidth := r.config.MaxWidth - contentIndent
	if maxContentWidth < 20 {
		maxContentWidth = 20
	}

	// Helper to write wrapped content lines
	writeContentLine := func(line string) {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			return
		}
		if len(trimmedLine) > maxContentWidth {
			// Wrap long lines
			wrapped := wrapText(trimmedLine, maxContentWidth)
			for _, wl := range wrapped {
				buf.WriteString(contentPrefix + "  " + r.text(wl) + "\n")
			}
		} else {
			buf.WriteString(contentPrefix + "  " + r.text(trimmedLine) + "\n")
		}
	}

	// Show truncation indicator first if lines were omitted
	if truncated > 0 {
		truncMsg := fmt.Sprintf("... (%d more lines)", truncated)
		buf.WriteString(prefix + branch + r.text(label+": "+truncMsg) + "\n")

		for _, line := range lines {
			writeContentLine(line)
		}
	} else if len(lines) == 1 {
		// Single line - show inline with label if short enough
		trimmed := strings.TrimSpace(lines[0])
		labelLine := label + ": " + trimmed
		if len(labelLine) <= maxContentWidth {
			buf.WriteString(prefix + branch + r.text(labelLine) + "\n")
		} else {
			buf.WriteString(prefix + branch + r.text(label+":") + "\n")
			writeContentLine(trimmed)
		}
	} else {
		// Multiple lines - show label on its own line, then content
		buf.WriteString(prefix + branch + r.text(label+":") + "\n")
		for _, line := range lines {
			writeContentLine(line)
		}
	}

	return buf.String()
}

// renderSubRuns renders references to sub-DAG runs.
func (r *Renderer) renderSubRuns(subRuns []exec.SubDAGRun, isLastSection bool, prefix string) string {
	var buf strings.Builder

	for i, sub := range subRuns {
		isLastItem := i == len(subRuns)-1
		branch := TreeBranch
		if isLastItem && isLastSection {
			branch = TreeLastBranch
		}

		subInfo := fmt.Sprintf("subdag: %s", sub.DAGRunID)
		if sub.Params != "" {
			subInfo += fmt.Sprintf(" [%s]", sub.Params)
		}

		// Tree lines faint, text sepia
		buf.WriteString(prefix + branch + r.text(subInfo) + "\n")
	}

	return buf.String()
}

// renderError renders an error message with wrapping.
func (r *Renderer) renderError(errMsg string, prefix string) string {
	// Clean the error message - remove "recent stderr (tail):" section
	// since we already display stderr separately in the tree
	cleanedErr := cleanErrorMessage(errMsg)

	errStr := fmt.Sprintf("error: %s", cleanedErr)

	// Calculate max width for wrapping
	prefixLen := len(prefix) + len(TreeLastBranch)
	maxWidth := r.config.MaxWidth - prefixLen
	if maxWidth < 20 {
		maxWidth = 20
	}

	if len(errStr) <= maxWidth {
		return prefix + TreeLastBranch + r.text(errStr) + "\n"
	}

	// Wrap long error messages
	var buf strings.Builder
	wrapped := wrapText(errStr, maxWidth)
	for i, line := range wrapped {
		if i == 0 {
			buf.WriteString(prefix + TreeLastBranch + r.text(line) + "\n")
		} else {
			// 4 spaces to match width of "└─" + indent (2 spaces)
			buf.WriteString(prefix + "    " + r.text(line) + "\n")
		}
	}
	return buf.String()
}

// cleanErrorMessage removes the "recent stderr (tail):" section from error messages
// since stderr is already displayed separately in the tree output.
func cleanErrorMessage(errMsg string) string {
	// Find and remove the "recent stderr (tail):" section
	if idx := strings.Index(errMsg, "\nrecent stderr (tail):"); idx != -1 {
		return strings.TrimSpace(errMsg[:idx])
	}
	if idx := strings.Index(errMsg, "recent stderr (tail):"); idx != -1 {
		return strings.TrimSpace(errMsg[:idx])
	}
	return errMsg
}

// renderFinalStatus renders the final result line at the bottom of the tree.
// White color (default).
func (r *Renderer) renderFinalStatus(status *exec.DAGRunStatus) string {
	prefix := "Result"
	if status.Status == core.Running {
		prefix = "Status"
	}
	return fmt.Sprintf("\n%s: %s\n", prefix, StatusText(status.Status))
}

// calculateDuration calculates the duration string between start and finish times.
func (r *Renderer) calculateDuration(startedAt, finishedAt string, status core.Status) string {
	if startedAt == "" || startedAt == "-" {
		return ""
	}

	start, err := stringutil.ParseTime(startedAt)
	if err != nil {
		return ""
	}

	var end time.Time
	if finishedAt != "" && finishedAt != "-" {
		end, err = stringutil.ParseTime(finishedAt)
		if err != nil {
			end = time.Now()
		}
	} else if status == core.Running {
		end = time.Now()
	} else {
		return ""
	}

	return stringutil.FormatDuration(end.Sub(start))
}

// calculateNodeDuration calculates duration for a specific node.
func (r *Renderer) calculateNodeDuration(node *exec.Node) string {
	nodeStatus := nodeStatusToStatus(node.Status)
	return r.calculateDuration(node.StartedAt, node.FinishedAt, nodeStatus)
}

// nodeStatusToStatus converts NodeStatus to Status for duration calculation.
func nodeStatusToStatus(ns core.NodeStatus) core.Status {
	switch ns {
	case core.NodeRunning:
		return core.Running
	case core.NodeSucceeded:
		return core.Succeeded
	case core.NodeFailed:
		return core.Failed
	case core.NodeAborted:
		return core.Aborted
	case core.NodePartiallySucceeded:
		return core.PartiallySucceeded
	case core.NodeNotStarted:
		return core.NotStarted
	case core.NodeSkipped:
		return core.NotStarted
	case core.NodeWaiting:
		return core.Waiting
	case core.NodeRejected:
		return core.Rejected
	}
	return core.NotStarted
}
