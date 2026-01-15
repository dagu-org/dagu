package cmdutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// EnvSource tracks where an environment variable came from (for debugging)
type EnvSource string

const (
	EnvSourceOS     EnvSource = "os"      // From os.Environ()
	EnvSourceDAGEnv EnvSource = "dag_env" // From DAG env: field
	EnvSourceDotEnv EnvSource = "dotenv"  // From .env file
	EnvSourceParam  EnvSource = "param"   // From params
	EnvSourceStep   EnvSource = "step"    // From step output
	EnvSourceSecret EnvSource = "secret"  // From secrets
)

// EnvEntry represents a single environment variable with metadata
type EnvEntry struct {
	Key    string
	Value  string
	Source EnvSource
}

// EnvScope is an isolated, immutable environment scope for DAG loading/execution.
// It does NOT modify the global os.Env.
type EnvScope struct {
	mu      sync.RWMutex
	entries map[string]EnvEntry // key -> entry with metadata
	parent  *EnvScope           // optional parent scope for layering
}

// NewEnvScope creates a new EnvScope, optionally inheriting from parent.
// If includeOS is true, it includes os.Environ() as the base layer.
func NewEnvScope(parent *EnvScope, includeOS bool) *EnvScope {
	e := &EnvScope{
		entries: make(map[string]EnvEntry),
		parent:  parent,
	}
	if includeOS {
		for _, env := range os.Environ() {
			if k, v, ok := strings.Cut(env, "="); ok {
				e.entries[k] = EnvEntry{Key: k, Value: v, Source: EnvSourceOS}
			}
		}
	}
	return e
}

// Set adds or updates a variable in this scope
func (e *EnvScope) Set(key, value string, source EnvSource) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.entries[key] = EnvEntry{Key: key, Value: value, Source: source}
}

// Get retrieves a variable, checking this scope then parent scopes
func (e *EnvScope) Get(key string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if entry, ok := e.entries[key]; ok {
		return entry.Value, true
	}
	if e.parent != nil {
		return e.parent.Get(key)
	}
	return "", false
}

// GetEntry retrieves the full entry with metadata
func (e *EnvScope) GetEntry(key string) (EnvEntry, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if entry, ok := e.entries[key]; ok {
		return entry, true
	}
	if e.parent != nil {
		return e.parent.GetEntry(key)
	}
	return EnvEntry{}, false
}

// ToSlice returns all variables as KEY=value strings (for cmd.Env)
func (e *EnvScope) ToSlice() []string {
	all := e.ToMap()
	result := make([]string, 0, len(all))
	for k, v := range all {
		result = append(result, k+"="+v)
	}
	return result
}

// ToMap returns all variables as a map
func (e *EnvScope) ToMap() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	all := make(map[string]string)
	if e.parent != nil {
		for k, v := range e.parent.ToMap() {
			all[k] = v
		}
	}
	for k, entry := range e.entries {
		all[k] = entry.Value
	}
	return all
}

// Expand expands ${VAR} and $VAR in s using this scope
func (e *EnvScope) Expand(s string) string {
	return os.Expand(s, func(key string) string {
		if v, ok := e.Get(key); ok {
			return v
		}
		return "" // Not found - return empty (consistent with os.ExpandEnv)
	})
}

// Debug returns a string representation for debugging
func (e *EnvScope) Debug() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var b strings.Builder
	b.WriteString("EnvScope{\n")
	for k, entry := range e.entries {
		fmt.Fprintf(&b, "  %s=%q (source: %s)\n", k, entry.Value, entry.Source)
	}
	if e.parent != nil {
		b.WriteString("  parent: <yes>\n")
	}
	b.WriteString("}")
	return b.String()
}

// Context key for EnvScope
type envScopeKey struct{}

// WithEnvScope returns a context with the given EnvScope
func WithEnvScope(ctx context.Context, scope *EnvScope) context.Context {
	return context.WithValue(ctx, envScopeKey{}, scope)
}

// GetEnvScope retrieves the EnvScope from context.
// Returns nil if context is nil or no EnvScope is set.
func GetEnvScope(ctx context.Context) *EnvScope {
	if ctx == nil {
		return nil
	}
	if scope, ok := ctx.Value(envScopeKey{}).(*EnvScope); ok {
		return scope
	}
	return nil
}

// GetEnvScopeOrOS returns the EnvScope from context, or creates one with OS env.
// If context is nil, returns a new EnvScope with OS environment.
func GetEnvScopeOrOS(ctx context.Context) *EnvScope {
	if scope := GetEnvScope(ctx); scope != nil {
		return scope
	}
	return NewEnvScope(nil, true)
}
