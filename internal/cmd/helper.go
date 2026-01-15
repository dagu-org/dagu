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
// This populates fields that are excluded from JSON serialization (env, shell, etc.)
// and must be called after LoadDotEnv() so dotenv values are available during rebuild.
//
// The function preserves Location and YamlData from the original DAG, as these
// should not be rebuilt from YAML.
func rebuildDAGFromYAML(ctx context.Context, dag *core.DAG) (*core.DAG, error) {
	if len(dag.YamlData) == 0 {
		return dag, nil
	}

	// Build env map from dag.Env (includes dotenv if LoadDotEnv was called).
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

	// Preserve fields that should NOT be rebuilt from YAML.
	fresh.Location = dag.Location
	fresh.YamlData = dag.YamlData

	core.InitializeDefaults(fresh)

	return fresh, nil
}
