package digraph

import (
	"context"
	"strings"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// Env contains the execution metadata for a dag-run.
type Env struct {
	DAGRunID       string
	RootDAGRun     DAGRunRef
	DAG            *DAG
	DB             Database
	BaseEnv        *config.BaseEnv
	Envs           map[string]string
	CoordinatorCli Dispatcher
}

func (e Env) AllEnvs() []string {
	envs := e.BaseEnv.AsSlice()
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

// Dispatcher defines the interface for coordinator operations
type Dispatcher interface {
	// Dispatch sends a task to the coordinator
	Dispatch(ctx context.Context, task *coordinatorv1.Task) error

	// Cleanup cleans up any resources used by the coordinator client
	Cleanup(ctx context.Context) error
}

// SetupEnv sets up the execution context for a dag-run.
// It initializes the environment variables and the DAG metadata.
func SetupEnv(ctx context.Context, dag *DAG, db Database, rootDAGRun DAGRunRef, dagRunID, logFile string, params []string, coordinatorCli Dispatcher) context.Context {
	var envs = map[string]string{
		EnvKeyDAGRunLogFile: logFile,
		EnvKeyDAGRunID:      dagRunID,
		EnvKeyDAGName:       dag.Name,
	}
	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) != 2 {
			logger.Error(ctx, "invalid parameter: %s", param)
			continue
		}
		envs[parts[0]] = parts[1]
	}

	for _, kv := range dag.Env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			envs[parts[0]] = parts[1]
		}
	}

	return context.WithValue(ctx, envCtxKey{}, Env{
		RootDAGRun:     rootDAGRun,
		DAG:            dag,
		DB:             db,
		Envs:           envs,
		DAGRunID:       dagRunID,
		BaseEnv:        config.GetBaseEnv(ctx),
		CoordinatorCli: coordinatorCli,
	})
}

// SetupEnvForTest is a convenience function for tests that don't need coordinator client
func SetupEnvForTest(ctx context.Context, dag *DAG, db Database, rootDAGRun DAGRunRef, dagRunID, logFile string, params []string) context.Context {
	return SetupEnv(ctx, dag, db, rootDAGRun, dagRunID, logFile, params, nil)
}

func GetEnv(ctx context.Context) Env {
	value := ctx.Value(envCtxKey{})
	if value == nil {
		logger.Error(ctx, "failed to get the env")
		return Env{}
	}
	execEnv, ok := value.(Env)
	if !ok {
		logger.Error(ctx, "failed to get the env")
		return Env{}
	}
	return execEnv
}

type envCtxKey struct{}
