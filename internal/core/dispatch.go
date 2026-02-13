package core

import "github.com/dagu-org/dagu/internal/cmn/config"

// ShouldDispatchToCoordinator decides whether a DAG should be dispatched
// to the coordinator for distributed execution.
func ShouldDispatchToCoordinator(dag *DAG, hasCoordinator bool, defaultMode config.ExecutionMode) bool {
	if dag.ForceLocal {
		return false
	}
	if !hasCoordinator {
		return false
	}
	if len(dag.WorkerSelector) > 0 {
		return true
	}
	if defaultMode == config.ExecutionModeDistributed {
		return true
	}
	return false
}
