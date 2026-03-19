// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
)

// EvalString evaluates the given string with the variables within the execution context.
func EvalString(ctx context.Context, s string, opts ...eval.Option) (string, error) {
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
	return eval.Object(ctx, obj, GetEnv(ctx).UserEnvsMap())
}

// GenerateSubDAGRunID generates a unique run ID based on the current DAG run ID, step name, and parameters.
func GenerateSubDAGRunID(ctx context.Context, params string, repeated bool) string {
	return GenerateSubDAGRunIDForTarget(ctx, "", params, repeated)
}

// GenerateSubDAGRunIDForTarget generates a unique run ID for a sub-DAG target.
// Including the target keeps deterministic IDs stable for retries while avoiding
// collisions when one parent step dispatches different child DAGs with identical params.
func GenerateSubDAGRunIDForTarget(ctx context.Context, dagName, params string, repeated bool) string {
	identity := params
	if dagName != "" {
		identity = dagName + "\x00" + params
	}

	if repeated {
		// If this is a repeated sub DAG run, we need to generate a unique ID with randomness
		// to avoid collisions with previous runs.
		randomBytes := make([]byte, 8)
		if _, err := rand.Read(randomBytes); err != nil {
			randomBytes = fmt.Appendf(nil, "%d", time.Now().UnixNano())
		}
		return stringutil.Base58EncodeSHA256(
			fmt.Sprintf("%s:%s:%s:%x", GetEnv(ctx).DAGRunID, GetEnv(ctx).Step.Name, identity, randomBytes),
		)
	}
	env := GetEnv(ctx)
	return stringutil.Base58EncodeSHA256(
		fmt.Sprintf("%s:%s:%s", env.DAGRunID, env.Step.Name, identity),
	)
}
