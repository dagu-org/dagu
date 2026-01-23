package cmdutil

import (
	"context"
	"os"
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
