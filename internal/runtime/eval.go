package runtime

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
)

// EvalString evaluates the given string with the variables within the execution context.
func EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	return GetEnv(ctx).EvalString(ctx, s, opts...)
}

// EvalBool evaluates the given value with the variables within the execution context
// and parses it as a boolean.
func EvalBool(ctx context.Context, value any) (bool, error) {
	return GetEnv(ctx).EvalBool(ctx, value)
}

// EvalObject recursively evaluates the string fields of the given object
// with the variables within the execution context.
func EvalObject[T any](ctx context.Context, obj T) (T, error) {
	return cmdutil.EvalObject(ctx, obj, AllEnvsMap(ctx))
}

// GenerateSubDAGRunID generates a unique run ID based on the current DAG run ID, step name, and parameters.
func GenerateSubDAGRunID(ctx context.Context, params string, repeated bool) string {
	if repeated {
		// If this is a repeated sub DAG run, we need to generate a unique ID with randomness
		// to avoid collisions with previous runs.
		randomBytes := make([]byte, 8)
		if _, err := rand.Read(randomBytes); err != nil {
			randomBytes = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
		}
		return stringutil.Base58EncodeSHA256(
			fmt.Sprintf("%s:%s:%s:%x", GetEnv(ctx).DAGRunID, GetEnv(ctx).Step.Name, params, randomBytes),
		)
	}
	env := GetEnv(ctx)
	return stringutil.Base58EncodeSHA256(
		fmt.Sprintf("%s:%s:%s", env.DAGRunID, env.Step.Name, params),
	)
}
