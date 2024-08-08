// Copyright (C) 2024 The Daguflow/Dagu Authors
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

package dag

import (
	"context"
	"errors"
)

// Finder finds a DAG by name.
// This is used to find the DAG when a node references another DAG.
type Finder interface {
	Find(name string) (*DAG, error)
}

// Context contains the current DAG and Finder.
type Context struct {
	DAG                  *DAG
	Finder               Finder
	DaguSchedulerLogPath string
	DaguExecutionLogPath string
	DaguRequestID        string
}

// ctxKey is used as the key for storing the DAG in the context.
type ctxKey struct{}

// NewContext creates a new context with the DAG and Finder and RequestIDEnvKey.
func NewContext(ctx context.Context, dagCtx Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, dagCtx)
}

var (
	errFailedCtxAssertion = errors.New("failed to assert DAG context")
)

const (
	ExecutionLogPathEnvKey = "DAGU_EXECUTION_LOG_PATH"
	SchedulerLogPathEnvKey = "DAGU_SCHEDULER_LOG_PATH"
	RequestIDEnvKey        = "DAGU_REQUEST_ID"
)

// GetContext returns the DAG Context from the context.
// It returns an error if the context does not contain a DAG Context.
func GetContext(ctx context.Context) (Context, error) {
	dagCtx, ok := ctx.Value(ctxKey{}).(Context)
	if !ok {
		return Context{}, errFailedCtxAssertion
	}
	return dagCtx, nil
}
