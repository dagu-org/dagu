package core

// ExecutorCapabilities defines what an executor can do.
type ExecutorCapabilities struct {
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
// Returns default capabilities (MultipleCommands: true) if not registered.
func (r *executorCapabilitiesRegistry) Get(executorType string) ExecutorCapabilities {
	if caps, ok := r.caps[executorType]; ok {
		return caps
	}
	// Default: allow multiple commands (shell, command, docker, ssh)
	return ExecutorCapabilities{MultipleCommands: true}
}

// RegisterExecutorCapabilities registers capabilities for an executor type.
func RegisterExecutorCapabilities(executorType string, caps ExecutorCapabilities) {
	executorCapabilities.Register(executorType, caps)
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
