package digraph

import "context"

// DB gets a result of a workflow.
type DB interface {
	GetDAG(ctx context.Context, name string) (*DAG, error)
	GetChildWorkflowStatus(ctx context.Context, workflowID string, root WorkflowRef) (*Status, error)
}

// Status is the result of a workflow.
type Status struct {
	// Name represents the name of the executed workflow.
	Name string `json:"name,omitempty"`
	// WorkflowID is the ID of the workflow.
	WorkflowID string `json:"workflowId,omitempty"`
	// Params is the parameters of the workflow
	Params string `json:"params,omitempty"`
	// Outputs is the outputs of the workflow.
	Outputs map[string]string `json:"outputs,omitempty"`
}
