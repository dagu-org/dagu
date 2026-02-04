package eval

import (
	"context"
	"errors"
	"os"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

// ExpandEnvContext expands ${VAR} and $VAR in s using EnvScope from context,
// falling back to os.LookupEnv if no scope in context.
// Variables not found are preserved in their original form.
func ExpandEnvContext(ctx context.Context, s string) string {
	scope := GetEnvScope(ctx)
	if scope == nil {
		// No scope - use OS lookup but preserve unknown vars
		return expandWithLookup(s, os.LookupEnv)
	}
	return scope.Expand(s)
}

// expandWithShellContext performs POSIX shell-style variable expansion using mvdan.cc/sh.
// Falls back to ExpandEnvContext on parse errors or unexpected command substitutions.
func expandWithShellContext(ctx context.Context, input string, opts *Options) (string, error) {
	if !opts.ExpandShell {
		if !opts.ExpandEnv {
			return input, nil
		}
		return ExpandEnvContext(ctx, input), nil
	}

	parser := syntax.NewParser()
	word, err := parser.Document(strings.NewReader(input))
	if err != nil {
		return "", err
	}
	if word == nil {
		return "", nil
	}

	r := newResolver(ctx, opts)
	cfg := &expand.Config{
		Env: expand.FuncEnviron(func(name string) string {
			if val, ok := r.resolveForShell(name); ok {
				return val
			}
			return ""
		}),
	}

	result, err := expand.Literal(cfg, word)
	if err != nil {
		var unexpected expand.UnexpectedCommandError
		if errors.As(err, &unexpected) {
			return ExpandEnvContext(ctx, input), nil
		}
		return "", err
	}
	return result, nil
}
