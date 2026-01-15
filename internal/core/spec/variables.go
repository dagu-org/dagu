package spec

import (
	"context"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec/types"
)

// loadVariables loads environment variables from strVariables and returns the
// resulting map of key→value without modifying the global OS environment.
//
// strVariables may be either a map[string]any or a []any containing maps and/or
// "key=value" strings; entries are collected in input order. For each pair, the
// value is optionally evaluated and expanded (including command substitution and
// references to previously defined variables) unless the BuildFlagNoEval option
// is set on ctx. The environment is passed via context to ensure thread-safety
// during concurrent DAG loading. The function returns a validation error if the
// input is malformed or a value fails to evaluate.
func loadVariables(ctx BuildContext, strVariables any) (
	map[string]string, error,
) {
	var pairs []pair
	switch a := strVariables.(type) {
	case map[string]any:
		// Case 1. env is a map.
		if err := parseKeyValue(a, &pairs); err != nil {
			return nil, core.NewValidationError("env", a, err)
		}

	case []any:
		// Case 2. env is an array of maps.
		for _, v := range a {
			switch vv := v.(type) {
			case map[string]any:
				if err := parseKeyValue(vv, &pairs); err != nil {
					return nil, core.NewValidationError("env", v, err)
				}
			case string:
				// parse key=value string
				key, val, found := strings.Cut(vv, "=")
				if !found {
					return nil, core.NewValidationError("env", &pairs, fmt.Errorf("%w: %s", ErrInvalidEnvValue, v))
				}
				pairs = append(pairs, pair{key: key, val: val})
			default:
				return nil, core.NewValidationError("env", &pairs, fmt.Errorf("%w: %s", ErrInvalidEnvValue, v))
			}
			if aa, ok := v.(map[string]any); ok {
				if err := parseKeyValue(aa, &pairs); err != nil {
					return nil, core.NewValidationError("env", v, err)
				}
			}
		}
	}

	// Parse each key-value pair using context-based environment.
	vars := map[string]string{}
	for _, pair := range pairs {
		value := pair.val

		if !ctx.opts.Has(BuildFlagNoEval) {
			// Build an EnvScope with OS env + accumulated vars for evaluation.
			// This ensures command substitution and variable expansion work correctly
			// without mutating the global OS environment.
			scope := cmdutil.NewEnvScope(nil, true)
			for k, v := range vars {
				scope.Set(k, v, cmdutil.EnvSourceDAGEnv)
			}

			// Create evaluation context - handle nil parent context
			evalCtx := ctx.ctx
			if evalCtx == nil {
				evalCtx = context.Background()
			}
			evalCtx = cmdutil.WithEnvScope(evalCtx, scope)

			// Evaluate the value of the environment variable.
			// This also executes command substitution.
			// Pass accumulated vars so ${VAR} can reference previously defined vars
			var err error
			value, err = cmdutil.EvalString(evalCtx, value, cmdutil.WithVariables(vars))
			if err != nil {
				return nil, core.NewValidationError("env", pair.val, fmt.Errorf("%w: %s", ErrInvalidEnvValue, pair.val))
			}
			// Note: We no longer call os.Setenv to avoid race conditions
			// when multiple DAGs are loaded concurrently.
		}

		vars[pair.key] = value
	}

	return vars, nil
}

// loadVariablesFromEnvValue loads environment variables from a types.EnvValue.
// This function converts the typed EnvValue entries to the expected format
// and processes them using the same logic as loadVariables without modifying
// the global OS environment.
func loadVariablesFromEnvValue(ctx BuildContext, env types.EnvValue) (
	map[string]string, error,
) {
	if env.IsZero() {
		return nil, nil
	}

	vars := map[string]string{}
	for _, entry := range env.Entries() {
		value := entry.Value

		if !ctx.opts.Has(BuildFlagNoEval) {
			// Build an EnvScope with OS env + accumulated vars for evaluation.
			scope := cmdutil.NewEnvScope(nil, true)
			for k, v := range vars {
				scope.Set(k, v, cmdutil.EnvSourceDAGEnv)
			}

			// Create evaluation context - handle nil parent context
			evalCtx := ctx.ctx
			if evalCtx == nil {
				evalCtx = context.Background()
			}
			evalCtx = cmdutil.WithEnvScope(evalCtx, scope)

			// Evaluate the value of the environment variable.
			// This also executes command substitution.
			// Pass accumulated vars so ${VAR} can reference previously defined vars
			var err error
			value, err = cmdutil.EvalString(evalCtx, value, cmdutil.WithVariables(vars))
			if err != nil {
				return nil, core.NewValidationError("env", entry.Value, fmt.Errorf("%w: %s", ErrInvalidEnvValue, entry.Value))
			}
			// Note: We no longer call os.Setenv to avoid race conditions
			// when multiple DAGs are loaded concurrently.
		}

		vars[entry.Key] = value
	}

	return vars, nil
}

// pair represents a key-value pair.
type pair struct {
	key string
	val string
}

// parseKeyValue parse a key-value pair from a map and appends it to the pairs
// slice. Each entry in the map must have a string key and a string value.
func parseKeyValue(m map[string]any, pairs *[]pair) error {
	for key, v := range m {
		var val string
		switch v := v.(type) {
		case string:
			val = v
		default:
			val = fmt.Sprintf("%v", v)
		}

		*pairs = append(*pairs, pair{key: key, val: val})
	}
	return nil
}
