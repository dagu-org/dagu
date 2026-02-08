package scheduler

import (
	"context"

	"github.com/dagu-org/dagu/internal/core"
)

// DAGStateStore is the interface for per-DAG state persistence used by the scheduler.
type DAGStateStore interface {
	Load(ctx context.Context, dag *core.DAG) (core.DAGState, error)
	Save(ctx context.Context, dag *core.DAG, state core.DAGState) error
	LoadAll(ctx context.Context, dags map[string]*core.DAG) (map[*core.DAG]core.DAGState, error)
}
