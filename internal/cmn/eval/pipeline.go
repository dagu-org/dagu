package eval

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
)

// phase represents a single step in the evaluation pipeline.
type phase struct {
	name    string
	execute func(ctx context.Context, input string, opts *Options) (string, error)
	enabled func(opts *Options) bool // nil = always run
}

// pipeline is an ordered sequence of evaluation phases.
type pipeline struct {
	phases []phase
}

// execute runs all enabled phases in order on the input string.
func (p *pipeline) execute(ctx context.Context, input string, opts *Options) (string, error) {
	value := input
	for _, ph := range p.phases {
		if ph.enabled != nil && !ph.enabled(opts) {
			continue
		}
		var err error
		value, err = ph.execute(ctx, value, opts)
		if err != nil {
			return "", fmt.Errorf("phase %s: %w", ph.name, err)
		}
	}
	return value, nil
}

// defaultPipeline is the standard evaluation pipeline used by String().
// Phase order: quoted-refs → variables → substitute → shell-expand
var defaultPipeline = &pipeline{
	phases: []phase{
		{
			name:    "quoted-refs",
			execute: expandQuotedRefs,
		},
		{
			name:    "variables",
			execute: expandAllVariables,
		},
		{
			name:    "substitute",
			execute: substitutePhase,
			enabled: func(opts *Options) bool { return opts.Substitute },
		},
		{
			name:    "shell-expand",
			execute: shellExpandPhase,
			enabled: func(opts *Options) bool { return opts.ExpandEnv },
		},
	},
}

// expandQuotedRefs handles quoted JSON references like "${FOO.bar}" and simple variables
// like "${VAR}" that appear within double quotes. These are resolved and re-quoted so
// that the JSON remains valid after expansion.
func expandQuotedRefs(ctx context.Context, input string, opts *Options) (string, error) {
	r := newResolver(ctx, opts)
	result := reQuotedJSONRef.ReplaceAllStringFunc(input, func(match string) string {
		// Extract the reference (VAR or VAR.path)
		ref := match[3 : len(match)-2] // Remove "$ and }"

		if strings.Contains(ref, ".") {
			dotIdx := strings.Index(ref, ".")
			varName := ref[:dotIdx]
			path := ref[dotIdx:]
			if resolved, ok := r.resolveReference(ctx, varName, path); ok {
				return strconv.Quote(resolved)
			}
		} else {
			if val, ok := r.resolve(ref); ok {
				return strconv.Quote(val)
			}
		}
		return match
	})
	return result, nil
}

// expandAllVariables resolves JSON path references, step property references,
// and simple $VAR/${VAR} patterns from all variable sources.
func expandAllVariables(ctx context.Context, input string, opts *Options) (string, error) {
	return expandVariables(ctx, input, opts), nil
}

// substitutePhase runs backtick command substitution.
func substitutePhase(ctx context.Context, input string, _ *Options) (string, error) {
	return substituteCommandsWithContext(ctx, input)
}

// shellExpandPhase performs shell-style variable expansion using mvdan.cc/sh.
// Falls back to simple regex-based expansion on failure.
// When ExpandOS is false, bypasses shell expansion entirely and uses scope-only expansion.
func shellExpandPhase(ctx context.Context, input string, opts *Options) (string, error) {
	if !opts.ExpandOS {
		return expandEnvScopeOnly(ctx, input), nil
	}
	expanded, err := expandWithShellContext(ctx, input, opts)
	if err != nil {
		logger.Debug(ctx, "Shell expansion failed, falling back to ExpandEnvContext",
			tag.Error(err))
		return ExpandEnvContext(ctx, input), nil
	}
	return expanded, nil
}

// evalStringValue applies variable expansion, substitution, and env expansion to a string.
// Used by StringFields and Object for struct/map field processing.
// Uses simple env expansion (ExpandEnvContext) rather than full shell expansion,
// which preserves undefined variables as-is.
func evalStringValue(ctx context.Context, value string, opts *Options) (string, error) {
	value = expandVariables(ctx, value, opts)
	if opts.Substitute {
		var err error
		value, err = substituteCommandsWithContext(ctx, value)
		if err != nil {
			return "", err
		}
	}
	if opts.ExpandEnv {
		if opts.ExpandOS {
			value = ExpandEnvContext(ctx, value)
		} else {
			value = expandEnvScopeOnly(ctx, value)
		}
	}
	return value, nil
}
