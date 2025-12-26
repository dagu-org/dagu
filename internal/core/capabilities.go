package core

// ExecutorCapabilities defines what an executor can do.
type ExecutorCapabilities struct {
	// MultipleCommands indicates whether the executor supports multiple commands.
	// If false, validation will reject steps with more than one command.
	MultipleCommands bool
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

// SupportsMultipleCommands returns whether the executor type supports multiple commands.
func (r *executorCapabilitiesRegistry) SupportsMultipleCommands(executorType string) bool {
	return r.Get(executorType).MultipleCommands
}

// RegisterExecutorCapabilities registers capabilities for an executor type.
func RegisterExecutorCapabilities(executorType string, caps ExecutorCapabilities) {
	executorCapabilities.Register(executorType, caps)
}

// SupportsMultipleCommands returns whether the executor type supports multiple commands.
func SupportsMultipleCommands(executorType string) bool {
	return executorCapabilities.SupportsMultipleCommands(executorType)
}
