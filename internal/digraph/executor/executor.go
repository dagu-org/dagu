// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/digraph"
)

type Executor interface {
	SetStdout(out io.Writer)
	SetStderr(out io.Writer)
	Kill(sig os.Signal) error
	Run() error
}

type Creator func(ctx context.Context, step digraph.Step) (Executor, error)

var (
	executors          = make(map[string]Creator)
	errInvalidExecutor = errors.New("invalid executor")
)

func NewExecutor(ctx context.Context, step digraph.Step) (Executor, error) {
	f, ok := executors[step.ExecutorConfig.Type]
	if ok {
		return f(ctx, step)
	}
	return nil, fmt.Errorf("%w: %s", errInvalidExecutor, step.ExecutorConfig)
}

func Register(name string, register Creator) {
	executors[name] = register
}
