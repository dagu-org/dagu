package digraph

import (
	"context"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/logger"
)

type Context struct {
	RootRun RootRun
	DAG     *DAG
	DB      DB
	Envs    map[string]string
	ReqID   string
}

type RootRun struct {
	Name  string
	ReqID string
}

func NewRootRun(name, reqID string) RootRun {
	return RootRun{
		Name:  name,
		ReqID: reqID,
	}
}

func GetDAGByName(ctx context.Context, name string) (*DAG, error) {
	c := GetExecContext(ctx)
	return c.DB.GetDAG(ctx, name)
}

func GetSubResult(ctx context.Context, requestID string) (*Status, error) {
	c := GetContext(ctx)
	return c.DB.GetSubStatus(ctx, requestID, c.RootRun)
}

func ApplyEnvs(ctx context.Context) {
	c := GetContext(ctx)
	for k, v := range c.Envs {
		if err := os.Setenv(k, v); err != nil {
			logger.Error(ctx, "failed to set environment variable %q: %v", k, err)
		}
	}
}

func (c Context) AllEnvs() []string {
	envs := os.Environ()
	envs = append(envs, c.DAG.Env...)
	for k, v := range c.Envs {
		envs = append(envs, k+"="+v)
	}
	return envs
}

func (c Context) EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	opts = append(opts, cmdutil.WithVariables(c.Envs))
	return cmdutil.EvalString(ctx, s, opts...)
}

func NewContext(ctx context.Context, d *DAG, c DB, r RootRun, reqID, logFile string, params []string) context.Context {
	var envs = map[string]string{
		EnvKeySchedulerLogPath: logFile,
		EnvKeyReqID:            reqID,
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
		RootRun: r,
		DAG:     d,
		DB:      c,
		Envs:    envs,
		ReqID:   reqID,
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
