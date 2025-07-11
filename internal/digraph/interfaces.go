package digraph

import (
	"context"
)

// Database is the interface for accessing the database to retrieve DAGs and dag-run statuses.
type Database interface {
	// GetDAG retrieves a DAG by its name.
	GetDAG(ctx context.Context, name string) (*DAG, error)
	// GetChildDAGRunStatus retrieves the status of a child dag-run by its ID and the root dag-run reference.
	GetChildDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) (*Status, error)
	// IsChildDAGRunCompleted checks if a child dag-run has completed.
	IsChildDAGRunCompleted(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) (bool, error)
}

// Status is the result of a dag-run.
type Status struct {
	// Name represents the name of the executed DAG.
	Name string `json:"name,omitempty"`
	// DAGRunID is the ID of the dag-run.
	DAGRunID string `json:"dagRunId,omitempty"`
	// Params is the parameters of the DAG.
	Params string `json:"params,omitempty"`
	// Outputs is the outputs of the dag-run.
	Outputs map[string]string `json:"outputs,omitempty"`
	// Success indicates if the dag-run completed successfully.
	// True means the DAG finished with success status, false means it failed, was canceled, or had partial success.
	Success bool `json:"success"`
}
