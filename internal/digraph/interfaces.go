package digraph

import (
	"context"
)

// Database is the interface for accessing the database to retrieve DAGs and dag-run statuses.
type Database interface {
	// GetDAG retrieves a DAG by its name.
	GetDAG(ctx context.Context, name string) (*DAG, error)
	// GetChildDAGRunStatus retrieves the status of a child dag-run by its ID and the root dag-run reference.
	GetChildDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) (RunStatus, error)
}

// ChildDAGRunStatus is an interface that represents the status of a child dag-run.
type RunStatus interface {
	// Outputs returns the outputs of the dag-run.
	Outputs() map[string]string
	// Success returns whether the dag-run was successful.
	Success() bool
	// StatusLabel returns the label of the dag-run status.
	StatusLabel() string
}
