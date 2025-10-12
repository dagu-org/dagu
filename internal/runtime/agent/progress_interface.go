package agent

import (
	"github.com/dagu-org/dagu/internal/core/execution"
)

// ProgressReporter is the interface for progress display implementations
type ProgressReporter interface {
	// Start begins the progress display
	Start()

	// Stop stops the progress display
	Stop()

	// UpdateNode updates the progress for a specific node
	UpdateNode(node *execution.Node)

	// UpdateStatus updates the overall DAG status
	UpdateStatus(status *execution.DAGRunStatus)

	// SetDAGRunInfo sets the DAG run ID and parameters
	SetDAGRunInfo(dagRunID, params string)
}

// Ensure implementation satisfies the interface
var _ ProgressReporter = (*ProgressTeaDisplay)(nil)
