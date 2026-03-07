package exec_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
)

func TestDAGContext_UserEnvsMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(ctx context.Context) context.Context
		expected map[string]string
	}{
		{
			name: "ExcludesOSEnvironment",
			setup: func(ctx context.Context) context.Context {
				dag := &core.DAG{
					Env: []string{"USER_VAR=user_value"},
				}
				return exec.NewContext(ctx, dag, "test-run", "test.log")
			},
			expected: map[string]string{
				"USER_VAR": "user_value",
			},
		},
		{
			name: "SecretOverridesEnvs",
			setup: func(ctx context.Context) context.Context {
				dag := &core.DAG{
					Env: []string{"KEY=from_dag"},
				}
				secrets := []string{"KEY=from_secret"}
				return exec.NewContext(ctx, dag, "test-run", "test.log",
					exec.WithSecrets(secrets),
				)
			},
			expected: map[string]string{
				"KEY": "from_secret",
			},
		},
		{
			name: "CombinesAllSources",
			setup: func(ctx context.Context) context.Context {
				dag := &core.DAG{
					Env: []string{"DAG_VAR=dag_value"},
				}
				secrets := []string{"SECRET_VAR=secret_value"}
				return exec.NewContext(ctx, dag, "test-run", "test.log",
					exec.WithSecrets(secrets),
				)
			},
			expected: map[string]string{
				"DAG_VAR":    "dag_value",
				"SECRET_VAR": "secret_value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			ctx = tt.setup(ctx)
			rCtx := exec.GetContext(ctx)

			result := rCtx.UserEnvsMap()

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "key %s should have value %s", key, expectedValue)
			}
			// Ensure OS env is not included (PATH should not be in result)
			_, hasPath := result["PATH"]
			assert.False(t, hasPath, "UserEnvsMap should not include OS environment variables like PATH")
		})
	}
}

func TestNewContext_DAGDocsDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		docsDir   string
		dagName   string
		expected  string
		expectSet bool
	}{
		{
			name:      "ConfigHasDocsDir",
			docsDir:   "/var/dagu/docs",
			dagName:   "my-workflow",
			expected:  filepath.Join("/var/dagu/docs", "my-workflow"),
			expectSet: true,
		},
		{
			name:      "DocsDirEmpty",
			docsDir:   "",
			dagName:   "my-workflow",
			expectSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tt.docsDir != "" {
				cfg := &config.Config{}
				cfg.Paths.DocsDir = tt.docsDir
				ctx = config.WithConfig(ctx, cfg)
			}

			dag := &core.DAG{Name: tt.dagName}
			ctx = exec.NewContext(ctx, dag, "run-1", "test.log")
			rCtx := exec.GetContext(ctx)
			result := rCtx.UserEnvsMap()

			if tt.expectSet {
				assert.Equal(t, tt.expected, result[exec.EnvKeyDAGDocsDir])
			} else {
				_, ok := result[exec.EnvKeyDAGDocsDir]
				assert.False(t, ok, "DAG_DOCS_DIR should not be set")
			}
		})
	}

	t.Run("NoConfigInContext", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		dag := &core.DAG{Name: "my-workflow"}
		ctx = exec.NewContext(ctx, dag, "run-1", "test.log")
		rCtx := exec.GetContext(ctx)
		result := rCtx.UserEnvsMap()

		_, ok := result[exec.EnvKeyDAGDocsDir]
		assert.False(t, ok, "DAG_DOCS_DIR should not be set when no config in context")
	})
}

func TestNewContext_DAGParamsJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		paramsJSON string
		expectSet  bool
	}{
		{
			name:       "JSONPresent",
			paramsJSON: `{"key":"value"}`,
			expectSet:  true,
		},
		{
			name:       "JSONEmpty",
			paramsJSON: "",
			expectSet:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			dag := &core.DAG{Name: "test-dag", ParamsJSON: tt.paramsJSON}
			ctx = exec.NewContext(ctx, dag, "run-1", "test.log")
			rCtx := exec.GetContext(ctx)
			result := rCtx.UserEnvsMap()

			if tt.expectSet {
				assert.Equal(t, tt.paramsJSON, result[exec.EnvKeyDAGParamsJSONCompat])
				assert.Equal(t, tt.paramsJSON, result[exec.EnvKeyDAGParamsJSON])
			} else {
				_, ok1 := result[exec.EnvKeyDAGParamsJSONCompat]
				_, ok2 := result[exec.EnvKeyDAGParamsJSON]
				assert.False(t, ok1, "DAG_PARAMS_JSON should not be set")
				assert.False(t, ok2, "DAGU_PARAMS_JSON should not be set")
			}
		})
	}
}
