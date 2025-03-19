package digraph

import (
	"context"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/logger"
)

type Context struct {
	rootRequestID string
	requestID     string
	dag           *DAG
	client        DBClient
	envs          map[string]string
}

func GetDAGByName(ctx context.Context, name string) (*DAG, error) {
	c := GetContext(ctx)
	return c.client.GetDAG(ctx, name)
}

func GetResult(ctx context.Context, name, requestID string) (*Status, error) {
	c := GetContext(ctx)
	return c.client.GetStatus(ctx, name, requestID)
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

func (c Context) RootRequestID() string {
	if c.rootRequestID != "" {
		return c.rootRequestID
	}
	return c.requestID
}

func NewContext(ctx context.Context, dag *DAG, client DBClient, rootRequestID, requestID, logFile string, params []string) context.Context {
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
		rootRequestID: rootRequestID,
		requestID:     requestID,
		dag:           dag,
		client:        client,
		envs:          envs,
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
