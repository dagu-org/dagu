// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

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
	Find(ctx context.Context, name string) (*DAG, error)
}

// ResultCollector gets a result of a DAG execution.
// This is used for subworkflow executor to get the output from the subworkflow.
type ResultCollector interface {
	CollectResult(ctx context.Context, name string, requestID string) (*Result, error)
}

type Result struct {
	Name    string            `json:"name,omitempty"`
	Params  string            `json:"params,omitempty"`
	Outputs map[string]string `json:"outputs,omitempty"`
}

// Context contains the current DAG and Finder.
type Context struct {
	DAG             *DAG
	Finder          Finder
	ResultCollector ResultCollector
	Envs            Envs
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
func NewContext(ctx context.Context, dag *DAG, finder Finder, resultCollector ResultCollector, requestID, logFile string) context.Context {
	return context.WithValue(ctx, ctxKey{}, Context{
		DAG:             dag,
		Finder:          finder,
		ResultCollector: resultCollector,
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

// StepContext contains the information needed to execute a step.
type StepContext struct{ OutputVariables *SyncMap }

type stepContextKey struct{}

func WithStepContext(ctx context.Context, stepContext *StepContext) context.Context {
	return context.WithValue(ctx, stepContextKey{}, stepContext)
}

// GetStepContext returns the StepContext from the context.
func GetStepContext(ctx context.Context) *StepContext {
	if v := ctx.Value(stepContextKey{}); v != nil {
		return v.(*StepContext)
	}
	return nil
}
