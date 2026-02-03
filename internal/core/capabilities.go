package core

import (
	"context"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
)

// ExecutorCapabilities defines what an executor can do.
type ExecutorCapabilities struct {
	// Command indicates whether the executor supports the command field.
	Command bool
	// MultipleCommands indicates whether the executor supports multiple commands.
	MultipleCommands bool
	// Script indicates whether the executor supports the script field.
	Script bool
	// Shell indicates whether the executor uses shell/shellArgs/shellPackages.
	Shell bool
	// Container indicates whether the executor supports step-level container config.
	Container bool
	// SubDAG indicates whether the executor can execute sub-DAGs.
	SubDAG bool
	// WorkerSelector indicates whether the executor supports worker selection.
	WorkerSelector bool
	// LLM indicates whether the executor supports the llm field.
	LLM bool
	// GetEvalOptions returns eval options for command argument evaluation.
	// If nil, default evaluation is used.
	GetEvalOptions func(ctx context.Context, step Step) []cmdutil.EvalOption
}

// executorCapabilitiesRegistry is a typed registry of executor capabilities.
type executorCapabilitiesRegistry struct {
	caps map[string]ExecutorCapabilities
}

var executorCapabilities = executorCapabilitiesRegistry{
	caps: make(map[string]ExecutorCapabilities),
}

// Register registers capabilities for an executor type.
func (r *executorCapabilitiesRegistry) Register(executorType string, caps ExecutorCapabilities) {
	r.caps[executorType] = caps
}

// Get returns capabilities for an executor type.
// Returns an empty ExecutorCapabilities if not registered.
func (r *executorCapabilitiesRegistry) Get(executorType string) ExecutorCapabilities {
	if caps, ok := r.caps[executorType]; ok {
		return caps
	}
	// Default: return all false (strict mode)
	return ExecutorCapabilities{}
}

// RegisterExecutorCapabilities registers capabilities for an executor type.
func RegisterExecutorCapabilities(executorType string, caps ExecutorCapabilities) {
	executorCapabilities.Register(executorType, caps)
}

// SupportsCommand returns whether the executor type supports the command field.
func SupportsCommand(executorType string) bool {
	return executorCapabilities.Get(executorType).Command
}

// SupportsMultipleCommands returns whether the executor type supports multiple commands.
func SupportsMultipleCommands(executorType string) bool {
	return executorCapabilities.Get(executorType).MultipleCommands
}

// SupportsScript returns whether the executor type supports the script field.
func SupportsScript(executorType string) bool {
	return executorCapabilities.Get(executorType).Script
}

// SupportsShell returns whether the executor type uses shell configuration.
func SupportsShell(executorType string) bool {
	return executorCapabilities.Get(executorType).Shell
}

// SupportsContainer returns whether the executor type supports step-level container config.
func SupportsContainer(executorType string) bool {
	return executorCapabilities.Get(executorType).Container
}

// SupportsSubDAG returns whether the executor type can execute sub-DAGs.
func SupportsSubDAG(executorType string) bool {
	return executorCapabilities.Get(executorType).SubDAG
}

// SupportsWorkerSelector returns whether the executor type supports worker selection.
func SupportsWorkerSelector(executorType string) bool {
	return executorCapabilities.Get(executorType).WorkerSelector
}

// SupportsLLM returns whether the executor type supports the llm field.
func SupportsLLM(executorType string) bool {
	return executorCapabilities.Get(executorType).LLM
}

func (s Step) EvalOptions(ctx context.Context) []cmdutil.EvalOption {
	caps := executorCapabilities.Get(s.ExecutorConfig.Type)
	if caps.GetEvalOptions != nil {
		return caps.GetEvalOptions(ctx, s)
	}
	return nil
}
