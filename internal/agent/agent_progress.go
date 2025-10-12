package agent

import (
	"strings"

	"github.com/dagu-org/dagu/internal/core"
)

// createProgressReporter creates the progress reporter
func createProgressReporter(dag *core.DAG, dagRunID string, params []string) ProgressReporter {
	display := NewProgressTeaDisplay(dag)
	display.SetDAGRunInfo(dagRunID, strings.Join(params, " "))
	return display
}
