package agent

import (
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"
)

// createProgressReporter creates the appropriate progress reporter based on configuration
func createProgressReporter(dag *digraph.DAG, dagRunID string, params []string) ProgressReporter {
	// Check if we should use the new Bubble Tea implementation
	useBubbleTea := os.Getenv("DAGU_USE_BUBBLETEA_PROGRESS") == "true"

	if useBubbleTea {
		display := NewProgressTeaDisplay(dag)
		display.SetDAGRunInfo(dagRunID, strings.Join(params, " "))
		return display
	}

	// Default to the original implementation
	display := NewProgressDisplay(os.Stderr, dag)
	display.SetDAGRunInfo(dagRunID, strings.Join(params, " "))
	return display
}
