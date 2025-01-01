// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"context"
)

type StepContext struct {
	Context
	OutputVariables *SyncMap
}

func NewStepContext(ctx context.Context) *StepContext {
	return &StepContext{
		Context:         GetContext(ctx),
		OutputVariables: &SyncMap{},
	}
}

func (s *StepContext) AllEnvs() []string {
	return s.Context.AllEnvs()
}

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
