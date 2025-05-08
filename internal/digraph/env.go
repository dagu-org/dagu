package digraph

import (
	"context"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/logger"
)

// Env contains the execution metadata for a DAG run.
type Env struct {
	RootRun RootRun
	DAG     *DAG
	DB      DB
	Envs    map[string]string
	ReqID   string
}

func (e Env) AllEnvs() []string {
	envs := os.Environ()
	envs = append(envs, e.DAG.Env...)
	for k, v := range e.Envs {
		envs = append(envs, k+"="+v)
	}
	return envs
}

func (e Env) EvalString(ctx context.Context, s string, opts ...cmdutil.EvalOption) (string, error) {
	opts = append(opts, cmdutil.WithVariables(e.Envs))
	return cmdutil.EvalString(ctx, s, opts...)
}

func (e Env) ApplyEnvs(ctx context.Context) {
	for k, v := range e.Envs {
		if err := os.Setenv(k, v); err != nil {
			logger.Error(ctx, "failed to set environment variable %q: %v", k, err)
		}
	}
}

// SetupEnv sets up the execution context for a DAG run.
// It initializes the environment variables and the DAG metadata.
func SetupEnv(ctx context.Context, d *DAG, c DB, r RootRun, reqID, logFile string, params []string) context.Context {
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

	return context.WithValue(ctx, envCtxKey{}, Env{
		RootRun: r,
		DAG:     d,
		DB:      c,
		Envs:    envs,
		ReqID:   reqID,
	})
}

func GetEnv(ctx context.Context) Env {
	value := ctx.Value(envCtxKey{})
	if value == nil {
		logger.Error(ctx, "failed to get the DAG context")
		return Env{}
	}
	execEnv, ok := value.(Env)
	if !ok {
		logger.Error(ctx, "failed to get the DAG context")
		return Env{}
	}
	return execEnv
}

type envCtxKey struct{}

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
