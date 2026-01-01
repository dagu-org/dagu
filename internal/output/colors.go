// Package output provides tree-structured rendering for DAG execution status.
package output

import (
	"github.com/dagu-org/dagu/internal/core"
	"github.com/fatih/color"
)

// Status symbols using Unicode characters for visual clarity.
const (
	SymbolRunning        = "●" // Filled circle for running
	SymbolSucceeded      = "✓" // Check mark for success
	SymbolFailed         = "✗" // X mark for failure
	SymbolSkipped        = "○" // Empty circle for skipped
	SymbolAborted        = "⚠" // Warning sign for aborted
	SymbolPartialSuccess = "◐" // Half-filled circle for partial success
	SymbolNotStarted     = "○" // Empty circle for not started
	SymbolQueued         = "◌" // Dotted circle for queued
)

// StatusSymbol returns the appropriate Unicode symbol for a DAG status.
func StatusSymbol(status core.Status) string {
	switch status {
	case core.Running:
		return SymbolRunning
	case core.Succeeded:
		return SymbolSucceeded
	case core.Failed:
		return SymbolFailed
	case core.Aborted:
		return SymbolAborted
	case core.PartiallySucceeded:
		return SymbolPartialSuccess
	case core.Queued:
		return SymbolQueued
	case core.NotStarted:
		return SymbolNotStarted
	default:
		return SymbolNotStarted
	}
}

// StatusText returns human-readable status text for a DAG status.
func StatusText(status core.Status) string {
	switch status {
	case core.Running:
		return "Running"
	case core.Succeeded:
		return "Succeeded"
	case core.Failed:
		return "Failed"
	case core.Aborted:
		return "Aborted"
	case core.PartiallySucceeded:
		return "Partially Succeeded"
	case core.Queued:
		return "Queued"
	case core.NotStarted:
		return "Not Started"
	default:
		return status.String()
	}
}

// StatusColorize applies color formatting to a string based on DAG status.
// Returns the colorized string if color is enabled, otherwise returns unchanged.
func StatusColorize(s string, status core.Status) string {
	switch status {
	case core.Running:
		return color.New(color.FgHiGreen).Sprint(s)
	case core.Succeeded:
		return color.GreenString(s)
	case core.Failed:
		return color.RedString(s)
	case core.Aborted:
		return color.YellowString(s)
	case core.PartiallySucceeded:
		return color.New(color.FgYellow).Sprint(s)
	case core.Queued:
		return color.BlueString(s)
	case core.NotStarted:
		return color.New(color.Faint).Sprint(s)
	default:
		return s
	}
}

// NodeStatusSymbol returns the appropriate Unicode symbol for a node status.
func NodeStatusSymbol(status core.NodeStatus) string {
	switch status {
	case core.NodeRunning:
		return SymbolRunning
	case core.NodeSucceeded:
		return SymbolSucceeded
	case core.NodeFailed:
		return SymbolFailed
	case core.NodeAborted:
		return SymbolAborted
	case core.NodeSkipped:
		return SymbolSkipped
	case core.NodePartiallySucceeded:
		return SymbolPartialSuccess
	case core.NodeNotStarted:
		return SymbolNotStarted
	default:
		return SymbolNotStarted
	}
}

// NodeStatusColorize applies color formatting to a string based on node status.
// Returns the colorized string if color is enabled, otherwise returns unchanged.
func NodeStatusColorize(s string, status core.NodeStatus) string {
	switch status {
	case core.NodeRunning:
		return color.New(color.FgHiGreen).Sprint(s)
	case core.NodeSucceeded:
		return color.GreenString(s)
	case core.NodeFailed:
		return color.RedString(s)
	case core.NodeAborted:
		return color.YellowString(s)
	case core.NodeSkipped:
		return color.New(color.Faint).Sprint(s)
	case core.NodePartiallySucceeded:
		return color.New(color.FgYellow).Sprint(s)
	case core.NodeNotStarted:
		return color.New(color.Faint).Sprint(s)
	default:
		return s
	}
}
