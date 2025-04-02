package digraph

import "context"

// DBClient gets a result of a DAG run.
type DBClient interface {
	GetDAG(ctx context.Context, name string) (*DAG, error)
	GetSubStatus(ctx context.Context, requestID string, rootDAG RootDAG) (*Status, error)
}

// Status is the result of a DAG run.
type Status struct {
	// Name represents the name of the executed DAG.
	Name string `json:"name,omitempty"`
	// Params is the parameters of the DAG run
	Params string `json:"params,omitempty"`
	// Outputs is the outputs of the DAG run.
	Outputs map[string]string `json:"outputs,omitempty"`
}
