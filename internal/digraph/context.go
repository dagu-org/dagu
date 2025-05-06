package digraph

import (
	"context"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/logger"
)

type Context struct {
	rootDAG   RootDAG
	requestID string
	dag       *DAG
	client    DBClient
	envs      map[string]string
}

type RootDAG struct {
	Name      string
	RequestID string
}

func NewRootDAG(name, requestID string) RootDAG {
	return RootDAG{
		Name:      name,
		RequestID: requestID,
	}
}

func GetDAGByName(ctx context.Context, name string) (*DAG, error) {
	c := GetExecContext(ctx)
	return c.client.GetDAG(ctx, name)
}

func GetSubResult(ctx context.Context, requestID string) (*Status, error) {
	c := GetContext(ctx)
	return c.client.GetSubStatus(ctx, requestID, c.rootDAG)
}

func ApplyEnvs(ctx context.Context) {
	c := GetContext(ctx)
	for k, v := range c.envs {
		if err := os.Setenv(k, v); err != nil {
			logger.Error(ctx, "failed to set environment variable %q: %v", k, err)
		}
	}
}

func (c Context) AllEnvs() []string {
	envs := os.Environ()
	envs = append(envs, c.dag.Env...)
	for k, v := range c.envs {
		envs = append(envs, k+"="+v)
	}
	return envs
}

func (c Context) WithEnv(key, value string) Context {
	c.envs[key] = value
	return c
}

func (c Context) EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	opts = append(opts, cmdutil.WithVariables(c.envs))
	return cmdutil.EvalString(ctx, s, opts...)
}

func (c Context) RootDAG() RootDAG {
	return c.rootDAG
}

func NewContext(ctx context.Context, d *DAG, c DBClient, rd RootDAG, reqID, logFile string, params []string) context.Context {
	var envs = map[string]string{
		EnvKeySchedulerLogPath: logFile,
		EnvKeyRequestID:        reqID,
		EnvKeyDAGName:          d.Name,
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
		rootDAG:   rd,
		requestID: reqID,
		dag:       d,
		client:    c,
		envs:      envs,
	})
}

func GetContext(ctx context.Context) Context {
	value := ctx.Value(ctxKey{})
	if value == nil {
		logger.Error(ctx, "failed to get the DAG context")
		return Context{}
	}
	contextValue, ok := value.(Context)
	if !ok {
		logger.Error(ctx, "failed to get the DAG context")
		return Context{}
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
