package spec

import (
	"fmt"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	digraph "github.com/dagu-org/dagu/internal/core"
)

// loadVariables loads the environment variables from the map.
// Case 1: env is a map.
// Case 2: env is an array of maps.
// Case 3: is recommended because the order of the environment variables is
// preserved.
func loadVariables(ctx BuildContext, strVariables any) (
	map[string]string, error,
) {
	var pairs []pair
	switch a := strVariables.(type) {
	case map[string]any:
		// Case 1. env is a map.
		if err := parseKeyValue(a, &pairs); err != nil {
			return nil, digraph.NewValidationError("env", a, err)
		}

	case []any:
		// Case 2. env is an array of maps.
		for _, v := range a {
			switch vv := v.(type) {
			case map[string]any:
				if err := parseKeyValue(vv, &pairs); err != nil {
					return nil, digraph.NewValidationError("env", v, err)
				}
			case string:
				// parse key=value string
				parts := strings.SplitN(vv, "=", 2)
				if len(parts) != 2 {
					return nil, digraph.NewValidationError("env", &pairs, fmt.Errorf("%w: %s", ErrInvalidEnvValue, v))
				}
				pairs = append(pairs, pair{key: parts[0], val: parts[1]})
			default:
				return nil, digraph.NewValidationError("env", &pairs, fmt.Errorf("%w: %s", ErrInvalidEnvValue, v))
			}
			if aa, ok := v.(map[string]any); ok {
				if err := parseKeyValue(aa, &pairs); err != nil {
					return nil, digraph.NewValidationError("env", v, err)
				}
			}
		}
	}

	// Parse each key-value pair and set the environment variable.
	vars := map[string]string{}
	for _, pair := range pairs {
		value := pair.val

		if !ctx.opts.NoEval {
			// Evaluate the value of the environment variable.
			// This also executes command substitution.
			// Pass accumulated vars so ${VAR} can reference previously defined vars
			var err error

			value, err = cmdutil.EvalString(ctx.ctx, value, cmdutil.WithVariables(vars))
			if err != nil {
				return nil, digraph.NewValidationError("env", pair.val, fmt.Errorf("%w: %s", ErrInvalidEnvValue, pair.val))
			}

			// Set the environment variable.
			if err := os.Setenv(pair.key, value); err != nil {
				return nil, digraph.NewValidationError("env", pair.key, fmt.Errorf("%w: %s", err, pair.key))
			}
		}

		vars[pair.key] = value
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
