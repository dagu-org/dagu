package digraph

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Executor is an interface for executing steps in a DAG.
type Executor interface {
	SetStdout(out io.Writer)
	SetStderr(out io.Writer)
	Kill(sig os.Signal) error
	Run(ctx context.Context) error
}

// ExecutorFactory is a function type that creates an Executor based on the step configuration.
type ExecutorFactory func(ctx context.Context, step Step) (Executor, error)

// NewExecutor creates a new Executor based on the step's executor type.
func NewExecutor(ctx context.Context, step Step) (Executor, error) {
	factory, ok := executorRegistry[step.ExecutorConfig.Type]
	if ok {
		return factory(ctx, step)
	}
	return nil, fmt.Errorf("executor type %q is not registered", step.ExecutorConfig.Type)
}

// RegisterExecutor registers a new executor type with its corresponding Creator function.
func RegisterExecutor(executorType string, factory ExecutorFactory) {
	executorRegistry[executorType] = factory
}

var executorRegistry = make(map[string]ExecutorFactory)
