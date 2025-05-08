package digraph

import "context"

// DB gets a result of a DAG execution.
type DB interface {
	GetDAG(ctx context.Context, name string) (*DAG, error)
	GetChildExecStatus(ctx context.Context, requestID string, root ExecRef) (*Status, error)
}

// Status is the result of a DAG execution.
type Status struct {
	// Name represents the name of the executed DAG.
	Name string `json:"name,omitempty"`
	// Params is the parameters of the DAG run
	Params string `json:"params,omitempty"`
	// Outputs is the outputs of the DAG execution.
	Outputs map[string]string `json:"outputs,omitempty"`
}
