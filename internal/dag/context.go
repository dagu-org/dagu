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

package dag

import (
	"context"
	"errors"
)

// Special environment variables.
const (
	EnvKeyLogPath          = "DAG_EXECUTION_LOG_PATH"
	EnvKeySchedulerLogPath = "DAG_SCHEDULER_LOG_PATH"
	EnvKeyRequestID        = "DAG_REQUEST_ID"
)

// Finder finds a DAG by name.
// This is used to find the DAG when a node references another DAG.
type Finder interface {
	Find(name string) (*DAG, error)
}

// Context contains the current DAG and Finder.
type Context struct {
	DAG    *DAG
	Finder Finder
	Envs   Envs
}

// Envs is a list of environment variables.
type Envs []Env

// All returns all the environment variables as a list of strings.
func (e Envs) All() []string {
	envs := make([]string, 0, len(e))
	for _, env := range e {
		envs = append(envs, env.String())
	}
	return envs
}

// Env is an environment variable.
type Env struct {
	Key   string
	Value string
}

// String returns the environment variable as a string.
func (e Env) String() string {
	return e.Key + "=" + e.Value
}

// ctxKey is used as the key for storing the DAG in the context.
type ctxKey struct{}

// NewContext creates a new context with the DAG and Finder.
func NewContext(ctx context.Context, dag *DAG, finder Finder, requestID, logFile string) context.Context {
	return context.WithValue(ctx, ctxKey{}, Context{
		DAG:    dag,
		Finder: finder,
		Envs: []Env{
			{Key: EnvKeySchedulerLogPath, Value: logFile},
			{Key: EnvKeyRequestID, Value: requestID},
		},
	})
}

func (c Context) WithEnv(env Env) Context {
	c.Envs = append([]Env{env}, c.Envs...)
	return c
}

var (
	errFailedCtxAssertion = errors.New("failed to assert DAG context")
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

func WithDagContext(ctx context.Context, dagContext Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, dagContext)
}
