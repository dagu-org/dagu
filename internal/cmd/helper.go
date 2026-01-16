package cmd

import (
	"context"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
)

// restoreDAGFromStatus restores a DAG from a previous run's status and YAML.
// It restores params from the status, loads dotenv, and rebuilds fields excluded
// from JSON serialization (env, shell, workingDir, registryAuths, etc.).
func restoreDAGFromStatus(ctx context.Context, dag *core.DAG, status *exec.DAGRunStatus) (*core.DAG, error) {
	dag.Params = status.ParamsList
	dag.LoadDotEnv(ctx)
	return rebuildDAGFromYAML(ctx, dag)
}

// rebuildDAGFromYAML rebuilds a DAG from its YamlData using the spec loader.
// This populates fields excluded from JSON serialization (json:"-") and must be
// called after LoadDotEnv() so dotenv values are available during rebuild.
//
// The function preserves all JSON-serialized fields from the original DAG and
// only copies JSON-excluded fields (Env, Params, ParamsJSON, SMTP, SSH,
// RegistryAuths) from the rebuilt DAG.
func rebuildDAGFromYAML(ctx context.Context, dag *core.DAG) (*core.DAG, error) {
	if len(dag.YamlData) == 0 {
		return dag, nil
	}

	// Build env map from dag.Env (includes dotenv values if LoadDotEnv was called).
	buildEnv := make(map[string]string, len(dag.Env))
	for _, env := range dag.Env {
		if k, v, ok := strings.Cut(env, "="); ok {
			buildEnv[k] = v
		}
	}

	loadOpts := []spec.LoadOption{
		spec.WithParams(dag.Params),
		spec.WithBuildEnv(buildEnv),
		spec.SkipSchemaValidation(),
	}

	if dag.Name != "" {
		loadOpts = append(loadOpts, spec.WithName(dag.Name))
	}

	fresh, err := spec.LoadYAML(ctx, dag.YamlData, loadOpts...)
	if err != nil {
		return nil, err
	}

	// Copy only fields excluded from JSON serialization (json:"-").
	// All other fields (Queue, WorkerSelector, HandlerOn, Steps, Tags, etc.)
	// are already correctly stored in dag.json and must be preserved.
	dag.Env = fresh.Env
	dag.Params = fresh.Params
	dag.ParamsJSON = fresh.ParamsJSON
	dag.SMTP = fresh.SMTP
	dag.SSH = fresh.SSH
	dag.RegistryAuths = fresh.RegistryAuths

	core.InitializeDefaults(dag)

	return dag, nil
}
