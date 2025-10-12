package scheduler

import "sync"

// ConfigSet is a thread-safe set of configurations.
type ConfigSet struct {
	mu      sync.RWMutex
	configs map[string]Config
}

// Config represents the configuration shared for all dag-runs with the same DAG name.
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
func (cs *ConfigSet) Get(name string) Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.configs[name]
}

// Set sets the configuration for a given DAG name.
func (cs *ConfigSet) Set(name string, config Config) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.configs[name] = config
}

// DefaultConfig is the default configuration used when no specific configuration is set.
var DefaultConfig = Config{
	ConcurrencyLimit: 1,
}
