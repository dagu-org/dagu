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
	EnvSourceOS      EnvSource = "os"       // From os.Environ()
	EnvSourceDAGEnv  EnvSource = "dag_env"  // From DAG env: field
	EnvSourceDotEnv  EnvSource = "dotenv"   // From .env file
	EnvSourceParam   EnvSource = "param"    // From params
	EnvSourceOutput  EnvSource = "output"   // From step output (renamed from EnvSourceStep)
	EnvSourceSecret  EnvSource = "secret"   // From secrets
	EnvSourceStepEnv EnvSource = "step_env" // From step.env field
)

// EnvSourceStep is deprecated: use EnvSourceOutput instead
const EnvSourceStep = EnvSourceOutput

// EnvEntry represents a single environment variable with metadata
type EnvEntry struct {
	Key    string
	Value  string
	Source EnvSource
	Origin string // stepID for outputs, filepath for dotenv (optional metadata)
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

// WithEntry returns a new EnvScope with the given entry added.
// The original scope is not modified (immutable).
func (e *EnvScope) WithEntry(key, value string, source EnvSource) *EnvScope {
	return e.WithEntryOrigin(key, value, source, "")
}

// WithEntryOrigin returns a new EnvScope with the given entry and origin metadata.
// The original scope is not modified (immutable).
func (e *EnvScope) WithEntryOrigin(key, value string, source EnvSource, origin string) *EnvScope {
	newScope := &EnvScope{
		entries: make(map[string]EnvEntry, 1),
		parent:  e,
	}
	newScope.entries[key] = EnvEntry{Key: key, Value: value, Source: source, Origin: origin}
	return newScope
}

// WithEntries returns a new EnvScope with the given entries added.
// The original scope is not modified (immutable).
func (e *EnvScope) WithEntries(entries map[string]string, source EnvSource) *EnvScope {
	if len(entries) == 0 {
		return e
	}
	newScope := &EnvScope{
		entries: make(map[string]EnvEntry, len(entries)),
		parent:  e,
	}
	for k, v := range entries {
		newScope.entries[k] = EnvEntry{Key: k, Value: v, Source: source}
	}
	return newScope
}

// WithStepOutputs returns a new EnvScope with step output variables added.
// The stepID is recorded as the origin for debugging.
func (e *EnvScope) WithStepOutputs(outputs map[string]string, stepID string) *EnvScope {
	if len(outputs) == 0 {
		return e
	}
	newScope := &EnvScope{
		entries: make(map[string]EnvEntry, len(outputs)),
		parent:  e,
	}
	for k, v := range outputs {
		newScope.entries[k] = EnvEntry{Key: k, Value: v, Source: EnvSourceOutput, Origin: stepID}
	}
	return newScope
}

// Get retrieves a variable, checking this scope then parent scopes
func (e *EnvScope) Get(key string) (string, bool) {
	if e == nil {
		return "", false
	}
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
	if e == nil {
		return EnvEntry{}, false
	}
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
	if e == nil {
		return nil
	}
	all := e.ToMap()
	result := make([]string, 0, len(all))
	for k, v := range all {
		result = append(result, k+"="+v)
	}
	return result
}

// ToMap returns all variables as a map
func (e *EnvScope) ToMap() map[string]string {
	if e == nil {
		return make(map[string]string)
	}
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

// expandWithLookup expands $VAR and ${VAR} using the provided lookup function.
// Single-quoted variables ('$VAR' or '${VAR}') and unknown variables are preserved.
func expandWithLookup(s string, lookup func(key string) (string, bool)) string {
	return reVarSubstitution.ReplaceAllStringFunc(s, func(match string) string {
		key, ok := extractVarKey(match)
		if !ok {
			return match // Single-quoted - preserve
		}
		if val, found := lookup(key); found {
			return val
		}
		return match // Not found - preserve original
	})
}

// Expand expands ${VAR} and $VAR in s using this scope.
// Variables not found in the scope are preserved in their original form.
func (e *EnvScope) Expand(s string) string {
	if e == nil {
		return s
	}
	return expandWithLookup(s, e.Get)
}

// ExpandSkipOS expands ${VAR} and $VAR in s using this scope,
// but skips variables with EnvSourceOS. Used for command/script evaluation
// where OS vars should be left for shell expansion at runtime.
func (e *EnvScope) ExpandSkipOS(s string) string {
	if e == nil {
		return s
	}
	return expandWithLookup(s, func(key string) (string, bool) {
		if entry, ok := e.GetEntry(key); ok && entry.Source != EnvSourceOS {
			return entry.Value, true
		}
		return "", false
	})
}

// Debug returns a string representation for debugging
func (e *EnvScope) Debug() string {
	if e == nil {
		return "EnvScope{nil}"
	}
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

// AllBySource returns all entries with the given source.
// This is useful for getting all secrets for masking, all params, etc.
func (e *EnvScope) AllBySource(source EnvSource) map[string]string {
	if e == nil {
		return make(map[string]string)
	}
	result := make(map[string]string)
	e.collectBySource(source, result)
	return result
}

func (e *EnvScope) collectBySource(source EnvSource, result map[string]string) {
	if e == nil {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	// First collect from parent (so child entries override)
	if e.parent != nil {
		e.parent.collectBySource(source, result)
	}
	// Then add entries from this scope
	for k, entry := range e.entries {
		if entry.Source == source {
			result[k] = entry.Value
		}
	}
}

// AllSecrets returns all entries with EnvSourceSecret.
// This is a convenience method for output masking.
func (e *EnvScope) AllSecrets() map[string]string {
	return e.AllBySource(EnvSourceSecret)
}

// AllUserEnvs returns all entries excluding OS environment.
// This replaces exec.Context.UserEnvsMap().
func (e *EnvScope) AllUserEnvs() map[string]string {
	result := make(map[string]string)
	e.collectUserEnvs(result)
	return result
}

func (e *EnvScope) collectUserEnvs(result map[string]string) {
	if e == nil {
		return
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	// First collect from parent (so child entries override)
	if e.parent != nil {
		e.parent.collectUserEnvs(result)
	}
	// Then add entries from this scope (excluding OS source)
	for k, entry := range e.entries {
		if entry.Source != EnvSourceOS {
			result[k] = entry.Value
		}
	}
}

// Provenance returns a human-readable description of where a variable came from.
// Returns empty string if the variable is not found.
func (e *EnvScope) Provenance(key string) string {
	if e == nil {
		return ""
	}
	entry, ok := e.GetEntry(key)
	if !ok {
		return ""
	}
	if entry.Origin != "" {
		return fmt.Sprintf("%s (from %s)", entry.Source, entry.Origin)
	}
	return string(entry.Source)
}
