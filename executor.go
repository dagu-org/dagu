// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package dagu

import (
	"context"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/spec"
	runtimeexec "github.com/dagucloud/dagu/internal/runtime/executor"
)

type Step = core.Step
type Executor = runtimeexec.Executor
type ExecutorFactory func(context.Context, Step) (Executor, error)
type StepValidator = core.StepValidator
type ExecutorCapabilities = core.ExecutorCapabilities

type ExecutorOption func(*executorRegistration)

type executorRegistration struct {
	validator StepValidator
	caps      ExecutorCapabilities
}

func RegisterExecutor(name string, factory ExecutorFactory, opts ...ExecutorOption) {
	var registration executorRegistration
	for _, opt := range opts {
		opt(&registration)
	}
	runtimeexec.RegisterExecutor(
		name,
		runtimeexec.ExecutorFactory(factory),
		registration.validator,
		registration.caps,
	)
	spec.RegisterExecutorTypeName(name)
}

func WithStepValidator(validator StepValidator) ExecutorOption {
	return func(r *executorRegistration) {
		r.validator = validator
	}
}

func WithExecutorCapabilities(caps ExecutorCapabilities) ExecutorOption {
	return func(r *executorRegistration) {
		r.caps = caps
	}
}
