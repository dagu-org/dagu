package cmd

import (
	"context"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
)

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
	// This allows YAML to reference env vars via ${VAR} during rebuild.
	buildEnv := make(map[string]string)
	for _, env := range dag.Env {
		if k, v, ok := strings.Cut(env, "="); ok {
			buildEnv[k] = v
		}
	}

	// Build load options for rebuild.
	loadOpts := []spec.LoadOption{
		spec.WithParams(dag.Params),
		spec.WithBuildEnv(buildEnv),
		spec.SkipSchemaValidation(),
	}

	// Preserve Name if it was set (YAML may not have name field).
	if dag.Name != "" {
		loadOpts = append(loadOpts, spec.WithName(dag.Name))
	}

	// Rebuild using spec.LoadYAML with the params and build env.
	fresh, err := spec.LoadYAML(ctx, dag.YamlData, loadOpts...)
	if err != nil {
		return nil, err
	}

	// Preserve fields that should NOT be rebuilt from YAML.
	// Location is the file path, which doesn't exist in YAML.
	// YamlData is the original YAML content for future rebuilds.
	fresh.Location = dag.Location
	fresh.YamlData = dag.YamlData

	// Initialize defaults (HistRetentionDays, MaxCleanUpTime, etc.)
	// Note: spec.LoadYAML doesn't call InitializeDefaults, unlike spec.Load
	core.InitializeDefaults(fresh)

	return fresh, nil
}
