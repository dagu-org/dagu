// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"context"
	"os"

	"github.com/dagu-org/dagu/internal/logger"
)

type Context struct {
	Context         context.Context
	DAG             *DAG
	Finder          Finder
	ResultCollector ExecutionResultCollector
	envs            kvPairs
}

func (c Context) ListEnvs() []string {
	envs := append([]string{}, os.Environ()...)
	envs = append(envs, c.DAG.Env...)
	for _, env := range c.envs {
		envs = append(envs, env.String())
	}
	return envs
}

type kvPairs []kvPair

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
		Context:         ctx,
		DAG:             dag,
		Finder:          finder,
		ResultCollector: resultCollector,
		envs: []kvPair{
			{Key: EnvKeySchedulerLogPath, Value: logFile},
			{Key: EnvKeyRequestID, Value: requestID},
			{Key: EnvKeyDAGName, Value: dag.Name},
		},
	})
}

func (c Context) ApplyEnvs() {
	for _, env := range c.envs {
		if err := os.Setenv(env.Key, env.Value); err != nil {
			logger.Error(c.Context, "failed to set environment variable %q: %v", env.Key, err)
		}
	}
}

func (c Context) WithEnv(key, value string) Context {
	c.envs = append([]kvPair{{Key: key, Value: value}}, c.envs...)
	return c
}

func GetContext(ctx context.Context) Context {
	contextValue, ok := ctx.Value(ctxKey{}).(Context)
	if !ok {
		logger.Error(ctx, "failed to get the DAG context")
		return Context{}
	}
	return contextValue
}

func WithContext(ctx context.Context, dagContext Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, dagContext)
}
