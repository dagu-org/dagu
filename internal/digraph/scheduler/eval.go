package scheduler

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/stringutil"
)

// EvalString evaluates the given string with the variables within the execution context.
func EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	return digraph.GetEnv(ctx).EvalString(ctx, s, opts...)
}

// EvalBool evaluates the given value with the variables within the execution context
// and parses it as a boolean.
func EvalBool(ctx context.Context, value any) (bool, error) {
	return digraph.GetEnv(ctx).EvalBool(ctx, value)
}

// EvalObject recursively evaluates the string fields of the given object
// with the variables within the execution context.
func EvalObject[T any](ctx context.Context, obj T) (T, error) {
	vars := digraph.GetEnv(ctx).VariablesMap()
	return cmdutil.EvalObject(ctx, obj, vars)
}

// GenerateChildDAGRunID generates a unique run ID based on the current DAG run ID, step name, and parameters.
func GenerateChildDAGRunID(ctx context.Context, params string, repeated bool) string {
	if repeated {
		// If this is a repeated child DAG run, we need to generate a unique ID with randomness
		// to avoid collisions with previous runs.
		randomBytes := make([]byte, 8)
		if _, err := rand.Read(randomBytes); err != nil {
			randomBytes = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
		}
		return stringutil.Base58EncodeSHA256(
			fmt.Sprintf("%s:%s:%s:%x", digraph.GetEnv(ctx).DAGRunID, digraph.GetEnv(ctx).Step.Name, params, randomBytes),
		)
	}
	env := digraph.GetEnv(ctx)
	return stringutil.Base58EncodeSHA256(
		fmt.Sprintf("%s:%s:%s", env.DAGRunID, env.Step.Name, params),
	)
}
