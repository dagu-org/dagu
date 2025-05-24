package scheduler

import "sync"

// WorkflowConfigSet is a thread-safe set of workflow configurations.
type WorkflowConfigSet struct {
	mu      sync.RWMutex
	configs map[string]WorkflowConfig
}

// WorkflowConfig represents the configuration shared for all workflows with the same name.
type WorkflowConfig struct {
	ConcurrencyLimit int
}

// NewWorkflowConfigSet creates a new WorkflowConfigSet.
func NewWorkflowConfigSet() *WorkflowConfigSet {
	return &WorkflowConfigSet{
		configs: make(map[string]WorkflowConfig),
	}
}

// Get retrieves the configuration for a given workflow name.
func (w *WorkflowConfigSet) Get(name string) WorkflowConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.configs[name]
}

// Set sets the configuration for a given workflow name.
func (w *WorkflowConfigSet) Set(name string, config WorkflowConfig) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.configs[name] = config
}

// DefaultWorkflowConfig is the default configuration for workflows.
var DefaultWorkflowConfig = WorkflowConfig{
	ConcurrencyLimit: 1,
}
