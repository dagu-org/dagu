package client

import (
	"context"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/persistence"
)

// RunClient is an interface for managing the execution of DAGs based on their DAG name and request ID.
// Note that DAG name may not be the same as the file name.
type RunClient interface {
	StopDAG(ctx context.Context, dag *digraph.DAG) error
	StartDAG(ctx context.Context, dag *digraph.DAG, opts StartOptions) error
	RestartDAG(ctx context.Context, dag *digraph.DAG, opts RestartOptions) error
	RetryDAG(ctx context.Context, dag *digraph.DAG, requestID string) error
	IsRunning(ctx context.Context, dag *digraph.DAG, requestID string) bool
	GetCurrentStatus(ctx context.Context, dag *digraph.DAG, requestId string) (*persistence.Status, error)
	GetStatusByRequestID(ctx context.Context, dag *digraph.DAG, requestID string) (*persistence.Status, error)
	GetLatestStatus(ctx context.Context, dag *digraph.DAG) (persistence.Status, error)
	GetRecentHistory(ctx context.Context, name string, n int) []persistence.Run
	GetStatus(ctx context.Context, name string, requestID string) (*persistence.Status, error)
	UpdateStatus(ctx context.Context, name string, status persistence.Status) error
	LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error)
	Rename(ctx context.Context, oldName, newName string) error
}

type StartOptions struct {
	Params string
	Quiet  bool
}

type RestartOptions struct {
	Quiet bool
}

type DAGStatus struct {
	File      string
	DAG       *digraph.DAG
	Status    persistence.Status
	Suspended bool
	Error     error
}

// ErrorAsString converts the error to a string if it exists, otherwise returns an empty string.
func (s DAGStatus) ErrorAsString() string {
	if s.Error == nil {
		return ""
	}
	return s.Error.Error()
}

func newDAGStatus(
	dag *digraph.DAG, status persistence.Status, suspended bool, err error,
) DAGStatus {
	var file string
	if dag.Location != "" {
		file = dag.Location
	}
	return DAGStatus{
		File:      file,
		DAG:       dag,
		Status:    status,
		Suspended: suspended,
		Error:     err,
	}
}
