package digraph

import (
	"fmt"
	"os"

	"github.com/dagu-org/dagu/internal/cmdutil"
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
	case map[any]any:
		// Case 1. env is a map.
		if err := parseKeyValue(a, &pairs); err != nil {
			return nil, wrapError("env", a, err)
		}

	case []any:
		// Case 2. env is an array of maps.
		for _, v := range a {
			if aa, ok := v.(map[any]any); ok {
				if err := parseKeyValue(aa, &pairs); err != nil {
					return nil, wrapError("env", v, err)
				}
			}
		}
	}

	// Parse each key-value pair and set the environment variable.
	vars := map[string]string{}
	for _, pair := range pairs {
		value := pair.val

		if !ctx.opts.noEval {
			// Evaluate the value of the environment variable.
			// This also executes command substitution.
			var err error

			value, err = cmdutil.EvalString(ctx.ctx, value)
			if err != nil {
				return nil, wrapError("env", pair.val, fmt.Errorf("%w: %s", errInvalidEnvValue, pair.val))
			}

			if err := os.Setenv(pair.key, value); err != nil {
				return nil, wrapError("env", pair.key, err)
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
func parseKeyValue(m map[any]any, pairs *[]pair) error {
	for k, v := range m {
		key, ok := k.(string)
		if !ok {
			return wrapError("env", k, errInvalidKeyType)
		}

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
