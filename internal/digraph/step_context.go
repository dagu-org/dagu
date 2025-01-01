// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"context"
)

type StepContext struct{ OutputVariables *SyncMap }

func WithStepContext(ctx context.Context, stepContext *StepContext) context.Context {
	return context.WithValue(ctx, stepCtxKey{}, stepContext)
}

func GetStepContext(ctx context.Context) *StepContext {
	if v := ctx.Value(stepCtxKey{}); v != nil {
		return v.(*StepContext)
	}
	return nil
}

type stepCtxKey struct{}
