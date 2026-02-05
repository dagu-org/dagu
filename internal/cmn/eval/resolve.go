package eval

import (
	"context"
	"os"
	"regexp"
	"strings"
)

// reVarSubstitution matches $VAR, ${VAR}, '$VAR', '${VAR}' patterns for variable substitution.
var reVarSubstitution = regexp.MustCompile(`[']{0,1}\$\{([^}]+)\}[']{0,1}|[']{0,1}\$([a-zA-Z0-9_][a-zA-Z0-9_]*)[']{0,1}`)

// reQuotedJSONRef matches quoted JSON references like "${FOO.bar}" and simple variables like "${VAR}"
var reQuotedJSONRef = regexp.MustCompile(`"\$\{([A-Za-z0-9_]\w*(?:\.[^}]+)?)\}"`)

// reJSONPathRef matches patterns like ${FOO.bar.baz} or $FOO.bar for JSON path expansion
var reJSONPathRef = regexp.MustCompile(`\$\{([A-Za-z0-9_]\w*)(\.[^}]+)\}|\$([A-Za-z0-9_]\w*)(\.[^\s]+)`)

// resolver provides unified variable resolution across explicit variable maps,
// EnvScope, and OS environment.
type resolver struct {
	variables []map[string]string
	stepMap   map[string]StepInfo
	scope     *EnvScope
	expandOS  bool
}

// newResolver creates a resolver from the given context and options.
func newResolver(ctx context.Context, opts *Options) *resolver {
	return &resolver{
		variables: opts.Variables,
		stepMap:   opts.StepMap,
		scope:     GetEnvScope(ctx),
		expandOS:  opts.ExpandOS,
	}
}

// lookupVariable searches the explicit variable maps for the given name.
func (r *resolver) lookupVariable(name string) (string, bool) {
	for _, vars := range r.variables {
		if val, ok := vars[name]; ok {
			return val, true
		}
	}
	return "", false
}

// lookupScopeNonOS searches the scope for a non-OS-sourced entry.
func (r *resolver) lookupScopeNonOS(name string) (string, bool) {
	if r.scope == nil {
		return "", false
	}
	entry, ok := r.scope.GetEntry(name)
	if ok && entry.Source != EnvSourceOS {
		return entry.Value, true
	}
	return "", false
}

// resolve looks up a variable from explicit variable maps and scope.
// Only user-defined scope entries are checked (OS-sourced entries are skipped).
func (r *resolver) resolve(name string) (string, bool) {
	if val, ok := r.lookupVariable(name); ok {
		return val, true
	}
	return r.lookupScopeNonOS(name)
}

// resolveForShell looks up a variable for shell expansion.
// Like resolve but includes OS environment as a final fallback when expandOS is true.
func (r *resolver) resolveForShell(name string) (string, bool) {
	if val, ok := r.resolve(name); ok {
		return val, true
	}
	if r.expandOS {
		return os.LookupEnv(name)
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

// resolveJSONSource looks up a variable's raw value for JSON path resolution.
// Unlike resolve, this includes OS-sourced scope entries when expandOS is true,
// because JSON path resolution needs the actual value regardless of source.
func (r *resolver) resolveJSONSource(name string) (string, bool) {
	if val, ok := r.lookupVariable(name); ok {
		return val, true
	}
	if r.expandOS {
		if r.scope != nil {
			if val, ok := r.scope.Get(name); ok {
				return val, true
			}
		}
		return os.LookupEnv(name)
	}
	return r.lookupScopeNonOS(name)
}

// extractVarKey extracts the variable key from a regex match.
// Returns the key and false if the match is single-quoted.
func extractVarKey(match string) (string, bool) {
	if match[0] == '\'' && match[len(match)-1] == '\'' {
		return "", false
	}
	if strings.HasPrefix(match, "${") {
		return match[2 : len(match)-1], true
	}
	return match[1:], true
}

// replaceVars substitutes $VAR and ${VAR} patterns using all resolver sources.
// JSON path references (containing dots) are skipped; those are handled by expandReferences.
func (r *resolver) replaceVars(template string) string {
	return reVarSubstitution.ReplaceAllStringFunc(template, func(match string) string {
		key, ok := extractVarKey(match)
		if !ok {
			return match
		}
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
