package digraph

import (
	"context"
	"encoding/json"

	"github.com/dagu-org/dagu/internal/digraph/status"
)

// Database is the interface for accessing the database to retrieve DAGs and dag-run statuses.
// This interface abstracts the underlying storage mechanism, allowing for different implementations (e.g., SQL, NoSQL, in-memory).
type Database interface {
	// GetDAG retrieves a DAG by its name.
	GetDAG(ctx context.Context, name string) (*DAG, error)
	// GetChildDAGRunStatus retrieves the status of a child dag-run by its ID and the root dag-run reference.
	GetChildDAGRunStatus(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) (*RunStatus, error)
	// IsChildDAGRunCompleted checks if a child dag-run has completed.
	IsChildDAGRunCompleted(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) (bool, error)
	// RequestChildCancel requests cancellation of a child dag-run.
	RequestChildCancel(ctx context.Context, dagRunID string, rootDAGRun DAGRunRef) error
}

// ChildDAGRunStatus is an interface that represents the status of a child dag-run.
type RunStatus struct {
	// Name represents the name of the executed DAG.
	Name string
	// DAGRunID is the ID of the dag-run.
	DAGRunID string
	// Params is the parameters of the DAG.
	Params string
	// Outputs is the outputs of the dag-run.
	Outputs map[string]string
	// Status is the execution status of the dag-run.
	Status status.Status
}

// MarshalJSON implements the json.Marshaler interface for RunStatus.
func (r *RunStatus) MarshalJSON() ([]byte, error) {
	return json.MarshalIndent(struct {
		Name     string            `json:"name,omitempty"`
		DAGRunID string            `json:"dagRunId,omitempty"`
		Params   string            `json:"params,omitempty"`
		Outputs  map[string]string `json:"outputs,omitzero"`
		Status   string            `json:"status"`
	}{
		Name:     r.Name,
		DAGRunID: r.DAGRunID,
		Params:   r.Params,
		Outputs:  r.Outputs,
		Status:   r.Status.String(),
	}, "", "  ")
}
