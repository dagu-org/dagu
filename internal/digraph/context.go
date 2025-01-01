// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"context"
	"os"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/mailer"
)

type Context struct {
	ctx    context.Context
	dag    *DAG
	client DBClient
	envs   kvPairs
}

func (c Context) GetDAGByName(name string) (*DAG, error) {
	return c.client.GetDAG(c.ctx, name)
}

func (c Context) GetResult(name, requestID string) (*Status, error) {
	return c.client.GetStatus(c.ctx, name, requestID)
}

func (c Context) AllEnvs() []string {
	envs := append([]string{}, os.Environ()...)
	envs = append(envs, c.dag.Env...)
	for _, env := range c.envs {
		envs = append(envs, env.String())
	}
	return envs
}

func (c Context) MailerConfig() (mailer.Config, error) {
	return EvalStringFields(c, mailer.Config{
		Host:     c.dag.SMTP.Host,
		Port:     c.dag.SMTP.Port,
		Username: c.dag.SMTP.Username,
		Password: c.dag.SMTP.Password,
	})
}

func EvalStringFields[T any](dagContext Context, obj T) (T, error) {
	return cmdutil.SubstituteStringFields(obj)
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

func NewContext(ctx context.Context, dag *DAG, client DBClient, requestID, logFile string) context.Context {
	return context.WithValue(ctx, ctxKey{}, Context{
		ctx:    ctx,
		dag:    dag,
		client: client,
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
			logger.Error(c.ctx, "failed to set environment variable %q: %v", env.Key, err)
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
