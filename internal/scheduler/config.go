package scheduler

import "sync"

// ConfigSet is a thread-safe set of configurations.
type ConfigSet struct {
	mu      sync.RWMutex
	configs map[string]Config
}

// Config represents the configuration shared for all workflows with the same name.
type Config struct {
	ConcurrencyLimit int
}

// NewConfigSet creates a new ConfigSet instance.
func NewConfigSet() *ConfigSet {
	return &ConfigSet{
		configs: make(map[string]Config),
	}
}

// Get retrieves the configuration for a given DAG name.
func (w *ConfigSet) Get(name string) Config {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.configs[name]
}

// Set sets the configuration for a given DAG name.
func (w *ConfigSet) Set(name string, config Config) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.configs[name] = config
}

// DefaultConfig is the default configuration used when no specific configuration is set.
var DefaultConfig = Config{
	ConcurrencyLimit: 1,
}
