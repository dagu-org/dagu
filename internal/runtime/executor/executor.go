package executor

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// Executor is an interface for executing steps in a DAG.
type Executor interface {
	SetStdout(out io.Writer)
	SetStderr(out io.Writer)
	Kill(sig os.Signal) error
	Run(ctx context.Context) error
}

// ExecutorFactory is a function type that creates an Executor based on the step configuration.
type ExecutorFactory func(ctx context.Context, step core.Step) (Executor, error)

// NewExecutor creates a new Executor based on the step's executor type.
func NewExecutor(ctx context.Context, step core.Step) (Executor, error) {
	factory, ok := executorRegistry[step.ExecutorConfig.Type]
	if ok {
		return factory(ctx, step)
	}

	logger.Error(ctx, "Executor type is not registered",
		tag.Type(step.ExecutorConfig.Type),
		tag.Step(step.Name),
	)
	return nil, fmt.Errorf("executor type %q is not registered", step.ExecutorConfig.Type)
}

// RegisterExecutor registers a new executor type with its factory, validator, and capabilities.
func RegisterExecutor(executorType string, factory ExecutorFactory, validator core.StepValidator, caps core.ExecutorCapabilities) {
	executorRegistry[executorType] = factory
	if validator != nil {
		core.RegisterStepValidator(executorType, validator)
	}
	core.RegisterExecutorCapabilities(executorType, caps)
}

var executorRegistry = make(map[string]ExecutorFactory)

// ExitCoder is an interface for executors that can return an exit code.
type ExitCoder interface {
	ExitCode() int
}

// NodeStatusDeterminer is an interface for reporting the status of a node execution.
type NodeStatusDeterminer interface {
	DetermineNodeStatus() (core.NodeStatus, error)
}

// DAGExecutor is an interface for sub DAG executors.
type DAGExecutor interface {
	Executor

	// SetParams sets the parameters for running a sub DAG.
	SetParams(RunParams)
}

// ParallelExecutor is an interface for parallel step executors.
type ParallelExecutor interface {
	Executor

	// SetParamsList sets the parameters for running multiple sub DAGs in parallel.
	SetParamsList([]RunParams)
}

// RunParams holds the parameters for running a sub DAG.
type RunParams struct {
	RunID  string
	Params string
}

// LLMMessageHandler is an interface for executors that handle LLM conversation messages.
type LLMMessageHandler interface {
	SetInheritedMessages([]execution.LLMMessage)
	GetMessages() []execution.LLMMessage
}
