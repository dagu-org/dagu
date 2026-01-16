package cmdutil

import (
	"context"
	"os"
)

// ExpandEnvContext expands ${VAR} and $VAR in s using EnvScope from context,
// falling back to os.ExpandEnv if no scope in context.
func ExpandEnvContext(ctx context.Context, s string) string {
	scope := GetEnvScope(ctx)
	if scope == nil {
		return os.ExpandEnv(s)
	}
	return scope.Expand(s)
}
