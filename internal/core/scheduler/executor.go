package scheduler

import (
	"context"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/status"
)

// ExitCoder is an interface for executors that can return an exit code.
type ExitCoder interface {
	ExitCode() int
}

// NodeStatusDeterminer is an interface for executors that can determine the status of a node.
type NodeStatusDeterminer interface {
	DetermineNodeStatus(ctx context.Context) (status.NodeStatus, error)
}

// DAGExecutor is an interface for child DAG executors.
type DAGExecutor interface {
	core.Executor

	// SetParams sets the parameters for running a child DAG.
	SetParams(RunParams)
}

// ParallelExecutor is an interface for parallel step executors.
type ParallelExecutor interface {
	core.Executor

	// SetParamsList sets the parameters for running multiple child DAGs in parallel.
	SetParamsList([]RunParams)
}

// RunParams holds the parameters for running a child DAG.
type RunParams struct {
	RunID  string
	Params string
}
