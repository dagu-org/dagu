package eval

import (
	"context"
	"os"
	"regexp"
	"strings"
)

// reVarSubstitution matches $VAR, ${VAR}, '$VAR', '${VAR}' patterns for variable substitution.
// Group 1: ${...} content, Group 2: $VAR content (without braces)
var reVarSubstitution = regexp.MustCompile(`[']{0,1}\$\{([^}]+)\}[']{0,1}|[']{0,1}\$([a-zA-Z0-9_][a-zA-Z0-9_]*)[']{0,1}`)

// reQuotedJSONRef matches quoted JSON references like "${FOO.bar}" and simple variables like "${VAR}"
var reQuotedJSONRef = regexp.MustCompile(`"\$\{([A-Za-z0-9_]\w*(?:\.[^}]+)?)\}"`)

// reJSONPathRef matches patterns like ${FOO.bar.baz} or $FOO.bar for JSON path expansion
var reJSONPathRef = regexp.MustCompile(`\$\{([A-Za-z0-9_]\w*)(\.[^}]+)\}|\$([A-Za-z0-9_]\w*)(\.[^\s]+)`)

// resolver provides unified variable resolution across multiple sources.
// It consolidates lookups from explicit variable maps, EnvScope, and OS environment.
type resolver struct {
	variables []map[string]string
	stepMap   map[string]StepInfo
	scope     *EnvScope
}

// newResolver creates a resolver from the given context and options.
func newResolver(ctx context.Context, opts *Options) *resolver {
	return &resolver{
		variables: opts.Variables,
		stepMap:   opts.StepMap,
		scope:     GetEnvScope(ctx),
	}
}

// resolve looks up a variable name from explicit variable maps and scope.
// Only user-defined scope entries are checked (OS-sourced entries are skipped).
// Returns (value, true) if found, ("", false) if not.
func (r *resolver) resolve(name string) (string, bool) {
	for _, vars := range r.variables {
		if val, ok := vars[name]; ok {
			return val, true
		}
	}
	if r.scope != nil {
		if entry, found := r.scope.GetEntry(name); found && entry.Source != EnvSourceOS {
			return entry.Value, true
		}
	}
	return "", false
}

// resolveForShell looks up a variable for shell expansion.
// Like resolve but includes OS environment as a final fallback.
func (r *resolver) resolveForShell(name string) (string, bool) {
	for _, vars := range r.variables {
		if val, ok := vars[name]; ok {
			return val, true
		}
	}
	if r.scope != nil {
		// Only use non-OS-sourced scope entries so we read live OS env below
		if entry, ok := r.scope.GetEntry(name); ok && entry.Source != EnvSourceOS {
			return entry.Value, true
		}
	}
	if val, exists := os.LookupEnv(name); exists {
		return val, true
	}
	return "", false
}

// resolveReference resolves a dotted reference (step property or JSON path).
func (r *resolver) resolveReference(ctx context.Context, varName, path string) (string, bool) {
	if r.stepMap != nil {
		if value, ok := resolveStepProperty(ctx, varName, path, r.stepMap); ok {
			return value, true
		}
	}
	jsonStr, ok := r.resolveJSONSource(varName)
	if !ok {
		return "", false
	}
	return resolveJSONPath(ctx, varName, jsonStr, path)
}

// resolveJSONSource looks up a variable's raw value for use as a JSON source document.
// Unlike resolve, this checks all sources including OS-sourced scope entries,
// because JSON path resolution needs the actual value regardless of source.
func (r *resolver) resolveJSONSource(name string) (string, bool) {
	for _, vars := range r.variables {
		if val, ok := vars[name]; ok {
			return val, true
		}
	}
	if r.scope != nil {
		if val, exists := r.scope.Get(name); exists {
			return val, true
		}
	}
	if val, exists := os.LookupEnv(name); exists {
		return val, true
	}
	return "", false
}

// extractVarKey extracts the variable key from a regex match.
// Returns the key and false if the match should be skipped (single-quoted).
func extractVarKey(match string) (string, bool) {
	if match[0] == '\'' && match[len(match)-1] == '\'' {
		return "", false // Single-quoted - skip
	}
	if strings.HasPrefix(match, "${") {
		return match[2 : len(match)-1], true
	}
	return match[1:], true
}

// replaceVars substitutes $VAR and ${VAR} patterns using all resolver sources.
// JSON path references (containing dots) are skipped — those are handled by expandReferences.
func (r *resolver) replaceVars(template string) string {
	return reVarSubstitution.ReplaceAllStringFunc(template, func(match string) string {
		key, ok := extractVarKey(match)
		if !ok {
			return match
		}
		// Skip JSON paths — handled by expandReferences
		if strings.Contains(key, ".") {
			return match
		}
		if val, found := r.resolve(key); found {
			return val
		}
		return match
	})
}

// expandReferences resolves JSON path and step property references in the input.
func (r *resolver) expandReferences(ctx context.Context, input string) string {
	return reJSONPathRef.ReplaceAllStringFunc(input, func(match string) string {
		subMatches := reJSONPathRef.FindStringSubmatch(match)
		if len(subMatches) < 3 {
			return match
		}

		var varName, path string
		if strings.HasPrefix(subMatches[0], "${") {
			varName = subMatches[1]
			path = subMatches[2]
		} else {
			varName = subMatches[3]
			path = subMatches[4]
		}

		if value, ok := r.resolveReference(ctx, varName, path); ok {
			return value
		}
		return match
	})
}
