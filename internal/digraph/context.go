// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"context"
	"errors"
)

type Context struct {
	DAG             *DAG
	Finder          Finder
	ResultCollector ExecutionResultCollector
	AdditionalEnvs  kvPairs
}

type kvPairs []kvPair

// ListAllEnvs returns all the environment variables as a list of strings.
func (e kvPairs) ListAllEnvs() []string {
	envs := make([]string, 0, len(e))
	for _, env := range e {
		envs = append(envs, env.String())
	}
	return envs
}

type kvPair struct {
	Key   string
	Value string
}

func (e kvPair) String() string {
	return e.Key + "=" + e.Value
}

type ctxKey struct{}

func NewContext(ctx context.Context, dag *DAG, finder Finder, resultCollector ExecutionResultCollector, requestID, logFile string) context.Context {
	return context.WithValue(ctx, ctxKey{}, Context{
		DAG:             dag,
		Finder:          finder,
		ResultCollector: resultCollector,
		AdditionalEnvs: []kvPair{
			{Key: EnvKeySchedulerLogPath, Value: logFile},
			{Key: EnvKeyRequestID, Value: requestID},
			{Key: EnvKeyDAGName, Value: dag.Name},
		},
	})
}

func (c Context) WithEnv(key, value string) Context {
	c.AdditionalEnvs = append([]kvPair{{Key: key, Value: value}}, c.AdditionalEnvs...)
	return c
}

func GetContext(ctx context.Context) (Context, error) {
	dagCtx, ok := ctx.Value(ctxKey{}).(Context)
	if !ok {
		return Context{}, errors.New("context does not have a DAG context")
	}
	return dagCtx, nil
}

func WithContext(ctx context.Context, dagContext Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, dagContext)
}
