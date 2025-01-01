// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"context"
	"strings"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/mailer"
)

type StepContext struct {
	Context

	outputVariables *SyncMap
	step            Step
}

func NewStepContext(ctx context.Context, step Step) StepContext {
	return StepContext{
		Context: GetContext(ctx),

		outputVariables: &SyncMap{},
		step:            step,
	}
}

func (s StepContext) AllEnvs() []string {
	envs := s.Context.AllEnvs()
	s.outputVariables.Range(func(_, value any) bool {
		envs = append(envs, value.(string))
		return true
	})
	return envs
}

func (s StepContext) LoadOutputVariables(vars *SyncMap) {
	vars.Range(func(key, value any) bool {
		// Skip if the key already exists
		if _, ok := s.outputVariables.Load(key); ok {
			return true
		}
		s.outputVariables.Store(key, value)
		return true
	})
}

func (c StepContext) MailerConfig() (mailer.Config, error) {
	return EvalStringFields(c, mailer.Config{
		Host:     c.dag.SMTP.Host,
		Port:     c.dag.SMTP.Port,
		Username: c.dag.SMTP.Username,
		Password: c.dag.SMTP.Password,
	})
}

func WithStepContext(ctx context.Context, stepContext StepContext) context.Context {
	return context.WithValue(ctx, stepCtxKey{}, stepContext)
}

func GetStepContext(ctx context.Context) StepContext {
	contextValue, ok := ctx.Value(stepCtxKey{}).(StepContext)
	if !ok {
		return NewStepContext(ctx, Step{})
	}
	return contextValue
}

type stepCtxKey struct{}

func EvalStringFields[T any](stepContext StepContext, obj T) (T, error) {
	vars := make(map[string]string)
	stepContext.outputVariables.Range(func(_, value any) bool {
		splits := strings.SplitN(value.(string), "=", 2)
		vars[splits[0]] = splits[1]
		return true
	})
	return cmdutil.SubstituteStringFields(obj, cmdutil.WithVariables(vars))
}
