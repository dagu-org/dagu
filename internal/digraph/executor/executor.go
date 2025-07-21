package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
)

// Executor is an interface for executing steps in a DAG.
type Executor interface {
	SetStdout(out io.Writer)
	SetStderr(out io.Writer)
	Kill(sig os.Signal) error
	Run(ctx context.Context) error
}

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
	Executor

	// SetParams sets the parameters for running a child DAG.
	SetParams(RunParams)
}

// ParallelExecutor is an interface for parallel step executors.
type ParallelExecutor interface {
	Executor

	// SetParamsList sets the parameters for running multiple child DAGs in parallel.
	SetParamsList([]RunParams)
}

// RunParams holds the parameters for running a child DAG.
type RunParams struct {
	RunID  string
	Params string
}

// Creator is a function type that creates an Executor based on the step configuration.
type Creator func(ctx context.Context, step digraph.Step) (Executor, error)

var (
	executors          = make(map[string]Creator)
	errInvalidExecutor = errors.New("invalid executor")
)

func NewExecutor(ctx context.Context, step digraph.Step) (Executor, error) {
	f, ok := executors[step.ExecutorConfig.Type]
	if ok {
		return f(ctx, step)
	}
	return nil, fmt.Errorf("%w: %s", errInvalidExecutor, step.ExecutorConfig)
}

func Register(name string, register Creator) {
	executors[name] = register
}
