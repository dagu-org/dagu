package digraph

import (
	"context"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/logger"
)

func EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	if c, ok := ctx.Value(stepCtxKey{}).(StepContext); ok {
		c.ctx = ctx
		return c.EvalString(s, opts...)
	}
	if c, ok := ctx.Value(ctxKey{}).(Context); ok {
		c.ctx = ctx
		return c.EvalString(s, opts...)
	}
	return cmdutil.EvalString(ctx, s, opts...)
}

type Context struct {
	ctx    context.Context
	dag    *DAG
	client DBClient
	envs   map[string]string
}

func (c Context) GetDAGByName(name string) (*DAG, error) {
	return c.client.GetDAG(c.ctx, name)
}

func (c Context) GetResult(name, requestID string) (*Status, error) {
	return c.client.GetStatus(c.ctx, name, requestID)
}

func (c Context) AllEnvs() []string {
	envs := os.Environ()
	envs = append(envs, c.dag.Env...)
	for k, v := range c.envs {
		envs = append(envs, k+"="+v)
	}
	return envs
}

func (c Context) ApplyEnvs() {
	for k, v := range c.envs {
		if err := os.Setenv(k, v); err != nil {
			logger.Error(c.ctx, "failed to set environment variable %q: %v", k, err)
		}
	}
}

func (c Context) WithEnv(key, value string) Context {
	c.envs[key] = value
	return c
}

func (c Context) EvalString(s string, opts ...cmdutil.EvalOption) (string, error) {
	opts = append(opts, cmdutil.WithVariables(c.envs))
	return cmdutil.EvalString(c.ctx, s, opts...)
}

func NewContext(ctx context.Context, dag *DAG, client DBClient, requestID, logFile string, params []string) context.Context {
	var envs = map[string]string{
		EnvKeySchedulerLogPath: logFile,
		EnvKeyRequestID:        requestID,
		EnvKeyDAGName:          dag.Name,
	}
	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			logger.Error(ctx, "invalid parameter: %s", param)
			continue
		}
		envs[parts[0]] = parts[1]
	}

	return context.WithValue(ctx, ctxKey{}, Context{
		ctx:    ctx,
		dag:    dag,
		client: client,
		envs:   envs,
	})
}

func GetContext(ctx context.Context) Context {
	value := ctx.Value(ctxKey{})
	if value == nil {
		logger.Error(ctx, "failed to get the DAG context")
		return Context{
			ctx: ctx,
		}
	}
	contextValue, ok := value.(Context)
	if !ok {
		logger.Error(ctx, "failed to get the DAG context")
		return Context{
			ctx: ctx,
		}
	}
	return contextValue
}

func WithContext(ctx context.Context, dagContext Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, dagContext)
}

func IsContext(ctx context.Context) bool {
	_, ok := ctx.Value(ctxKey{}).(Context)
	return ok
}

type ctxKey struct{}
