// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/daguflow/dagu/internal/dag"
)

type Executor interface {
	SetStdout(out io.Writer)
	SetStderr(out io.Writer)
	Kill(sig os.Signal) error
	Run() error
}

type Creator func(ctx context.Context, step dag.Step) (Executor, error)

var (
	executors          = make(map[string]Creator)
	errInvalidExecutor = errors.New("invalid executor")
)

func NewExecutor(ctx context.Context, step dag.Step) (Executor, error) {
	f, ok := executors[step.ExecutorConfig.Type]
	if ok {
		return f(ctx, step)
	}
	return nil, fmt.Errorf("%w: %s", errInvalidExecutor, step.ExecutorConfig)
}

func Register(name string, register Creator) {
	executors[name] = register
}
