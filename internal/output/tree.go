package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/fatih/color"
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
}

// DefaultConfig returns the default configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		ColorEnabled:   true,
		ShowStdout:     true,
		ShowStderr:     true,
		MaxOutputLines: DefaultMaxOutputLines,
	}
}

// Renderer renders DAG execution status as a tree structure.
type Renderer struct {
	config Config
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
func (r *Renderer) RenderDAGStatus(dag *core.DAG, status *execution.DAGRunStatus) string {
	var buf strings.Builder

	// Header line: ● Running - 2024-01-15 16:52:58
	buf.WriteString(r.renderHeader(status))
	buf.WriteString("\n")

	// DAG line: dag: my_dag (6s 619ms)
	buf.WriteString(r.renderDAGLine(dag, status))
	buf.WriteString("\n")

	// Render each step as a tree branch
	for i, node := range status.Nodes {
		isLast := i == len(status.Nodes)-1
		buf.WriteString(r.renderStep(node, isLast, ""))
	}

	// Final status line
	buf.WriteString(r.renderFinalStatus(status))

	return buf.String()
}

// renderHeader renders the status header line with symbol, status text, and timestamp.
func (r *Renderer) renderHeader(status *execution.DAGRunStatus) string {
	symbol := StatusSymbol(status.Status)
	statusText := StatusText(status.Status)

	// Parse start time, fallback to current time if not set
	startTime := status.StartedAt
	if startTime == "" || startTime == "-" {
		startTime = time.Now().Format("2006-01-02 15:04:05")
	}

	if r.config.ColorEnabled {
		return fmt.Sprintf("%s %s - %s",
			StatusColorize(symbol, status.Status),
			StatusColorize(statusText, status.Status),
			startTime)
	}
	return fmt.Sprintf("%s %s - %s", symbol, statusText, startTime)
}

// renderDAGLine renders the DAG name with total duration.
func (r *Renderer) renderDAGLine(dag *core.DAG, status *execution.DAGRunStatus) string {
	duration := r.calculateDuration(status.StartedAt, status.FinishedAt, status.Status)

	dagName := dag.Name
	if r.config.ColorEnabled {
		dagName = color.CyanString(dag.Name)
	}

	if duration != "" {
		return fmt.Sprintf("dag: %s (%s)", dagName, duration)
	}
	return fmt.Sprintf("dag: %s", dagName)
}

// renderStep renders a single step with its commands and output content.
func (r *Renderer) renderStep(node *execution.Node, isLast bool, prefix string) string {
	var buf strings.Builder

	// Determine branch character based on position
	branch := TreeBranch
	if isLast {
		branch = TreeLastBranch
	}

	// Calculate step duration
	duration := r.calculateNodeDuration(node)

	// Format step name with optional styling
	stepName := node.Step.Name
	if r.config.ColorEnabled {
		stepName = color.New(color.Bold).Sprint(stepName)
	}

	// Build step line: ├─step: build_files (4s 619ms)
	stepLine := fmt.Sprintf("%s%sstep: %s", prefix, branch, stepName)
	if duration != "" {
		durationStr := duration
		if r.config.ColorEnabled {
			durationStr = color.New(color.Faint).Sprintf("(%s)", duration)
		} else {
			durationStr = fmt.Sprintf("(%s)", duration)
		}
		stepLine += " " + durationStr
	}

	// Add status icon for completed steps
	if node.Status != core.NodeRunning && node.Status != core.NodeNotStarted {
		stepLine += " " + r.formatNodeStatus(node.Status)
	}

	buf.WriteString(stepLine)
	buf.WriteString("\n")

	// Calculate prefix for child elements
	childPrefix := prefix
	if isLast {
		childPrefix += TreeSpace
	} else {
		childPrefix += TreePipe
	}

	// Render commands
	commands := node.Step.Commands
	hasCommands := len(commands) > 0

	if !hasCommands {
		// Handle legacy single command format
		cmdStr := r.getLegacyCommand(node)
		if cmdStr != "" {
			buf.WriteString(r.renderCommand(cmdStr, node, true, childPrefix))
		} else {
			// No command, just show stdout/stderr directly under step
			buf.WriteString(r.renderOutputs(node, childPrefix))
		}
	} else {
		// Modern multi-command format
		for i, cmd := range commands {
			isLastCmd := i == len(commands)-1
			buf.WriteString(r.renderCommand(cmd.String(), node, isLastCmd, childPrefix))
		}
	}

	// Render sub-DAG runs if present
	if len(node.SubRuns) > 0 {
		buf.WriteString(r.renderSubRuns(node.SubRuns, childPrefix))
	}

	// Render error message if step failed
	if node.Error != "" && node.Status == core.NodeFailed {
		buf.WriteString(r.renderError(node.Error, childPrefix))
	}

	return buf.String()
}

// getLegacyCommand extracts command string from legacy step format.
func (r *Renderer) getLegacyCommand(node *execution.Node) string {
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

// renderCommand renders a command line with its stdout/stderr output.
func (r *Renderer) renderCommand(cmdStr string, node *execution.Node, isLast bool, prefix string) string {
	var buf strings.Builder

	// Determine if this is the last element (affects branch character)
	hasOutputs := r.config.ShowStdout || r.config.ShowStderr
	branch := TreeBranch
	if isLast && !hasOutputs {
		branch = TreeLastBranch
	}

	// Format command string
	formattedCmd := cmdStr
	if r.config.ColorEnabled {
		formattedCmd = color.New(color.FgHiWhite).Sprint(cmdStr)
	}
	buf.WriteString(fmt.Sprintf("%s%s%s\n", prefix, branch, formattedCmd))

	// Calculate prefix for output content
	outputPrefix := prefix
	if isLast {
		outputPrefix += TreeSpace
	} else {
		outputPrefix += TreePipe
	}

	// Render stdout and stderr
	buf.WriteString(r.renderOutputs(node, outputPrefix))

	return buf.String()
}

// renderOutputs renders stdout and stderr for a node.
func (r *Renderer) renderOutputs(node *execution.Node, prefix string) string {
	var buf strings.Builder

	if r.config.ShowStdout {
		hasStderr := r.config.ShowStderr
		buf.WriteString(r.renderOutput("stdout", node.Stdout, !hasStderr, prefix))
	}

	if r.config.ShowStderr {
		buf.WriteString(r.renderOutput("stderr", node.Stderr, true, prefix))
	}

	return buf.String()
}

// renderOutput renders a single output stream (stdout or stderr) with content.
func (r *Renderer) renderOutput(label string, filePath string, isLast bool, prefix string) string {
	var buf strings.Builder

	branch := TreeBranch
	if isLast {
		branch = TreeLastBranch
	}

	// Read log content with tail limit
	lines, truncated, err := ReadLogFileTail(filePath, r.config.MaxOutputLines)

	// Format label with appropriate color
	labelStr := label
	if r.config.ColorEnabled {
		if label == "stderr" && len(lines) > 0 {
			labelStr = color.YellowString(label)
		} else {
			labelStr = color.New(color.Faint).Sprint(label)
		}
	}

	// Handle empty or unreadable output
	if err != nil || len(lines) == 0 {
		buf.WriteString(fmt.Sprintf("%s%s%s:\n", prefix, branch, labelStr))
		return buf.String()
	}

	// Calculate content prefix for multi-line output
	contentPrefix := prefix
	if isLast {
		contentPrefix += TreeSpace
	} else {
		contentPrefix += TreePipe
	}

	// First line with label
	firstLine := lines[0]
	buf.WriteString(fmt.Sprintf("%s%s%s: %s\n", prefix, branch, labelStr, firstLine))

	// Show truncation indicator if lines were omitted
	if truncated > 0 {
		truncMsg := fmt.Sprintf("... (%d more lines)", truncated)
		if r.config.ColorEnabled {
			truncMsg = color.New(color.Faint).Sprint(truncMsg)
		}
		buf.WriteString(fmt.Sprintf("%s%s\n", contentPrefix, truncMsg))
	}

	// Remaining lines with proper indentation to align with first line content
	labelPadding := strings.Repeat(" ", len(label)+2) // +2 for ": "
	for i := 1; i < len(lines); i++ {
		buf.WriteString(fmt.Sprintf("%s%s%s\n", contentPrefix, labelPadding, lines[i]))
	}

	return buf.String()
}

// renderSubRuns renders references to sub-DAG runs.
func (r *Renderer) renderSubRuns(subRuns []execution.SubDAGRun, prefix string) string {
	var buf strings.Builder

	for i, sub := range subRuns {
		isLast := i == len(subRuns)-1
		branch := TreeBranch
		if isLast {
			branch = TreeLastBranch
		}

		subInfo := fmt.Sprintf("subdag: %s", sub.DAGRunID)
		if sub.Params != "" {
			subInfo += fmt.Sprintf(" [%s]", sub.Params)
		}

		if r.config.ColorEnabled {
			subInfo = color.New(color.Faint).Sprint(subInfo)
		}

		buf.WriteString(fmt.Sprintf("%s%s%s\n", prefix, branch, subInfo))
	}

	return buf.String()
}

// renderError renders an error message in red.
func (r *Renderer) renderError(errMsg string, prefix string) string {
	errStr := fmt.Sprintf("error: %s", errMsg)
	if r.config.ColorEnabled {
		errStr = color.RedString(errStr)
	}
	return fmt.Sprintf("%s%s%s\n", prefix, TreeLastBranch, errStr)
}

// renderFinalStatus renders the final result line at the bottom of the tree.
func (r *Renderer) renderFinalStatus(status *execution.DAGRunStatus) string {
	var result string

	switch status.Status {
	case core.Succeeded:
		result = "Result: Success"
		if r.config.ColorEnabled {
			result = color.GreenString("✓ ") + color.GreenString(result)
		} else {
			result = "✓ " + result
		}
	case core.Failed:
		result = "Result: Failed"
		if r.config.ColorEnabled {
			result = color.RedString("✗ ") + color.RedString(result)
		} else {
			result = "✗ " + result
		}
	case core.Running:
		result = "Status: Running..."
		if r.config.ColorEnabled {
			result = color.New(color.FgHiGreen).Sprint("● ") + result
		} else {
			result = "● " + result
		}
	case core.Aborted:
		result = "Result: Aborted"
		if r.config.ColorEnabled {
			result = color.YellowString("⚠ ") + color.YellowString(result)
		} else {
			result = "⚠ " + result
		}
	case core.PartiallySucceeded:
		result = "Result: Partially Succeeded"
		if r.config.ColorEnabled {
			result = color.YellowString("◐ ") + color.YellowString(result)
		} else {
			result = "◐ " + result
		}
	default:
		result = fmt.Sprintf("Status: %s", status.Status.String())
	}

	return "\n" + result + "\n"
}

// formatNodeStatus formats a node status with its icon symbol.
func (r *Renderer) formatNodeStatus(status core.NodeStatus) string {
	symbol := NodeStatusSymbol(status)
	if r.config.ColorEnabled {
		return NodeStatusColorize(symbol, status)
	}
	return symbol
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
func (r *Renderer) calculateNodeDuration(node *execution.Node) string {
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
	default:
		return core.NotStarted
	}
}
