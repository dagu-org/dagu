package execution_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
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
				return execution.NewContext(ctx, dag, "test-run", "test.log")
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
				return execution.NewContext(ctx, dag, "test-run", "test.log",
					execution.WithSecrets(secrets),
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
				return execution.NewContext(ctx, dag, "test-run", "test.log",
					execution.WithSecrets(secrets),
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
			execCtx := execution.GetContext(ctx)

			result := execCtx.UserEnvsMap()

			for key, expectedValue := range tt.expected {
				assert.Equal(t, expectedValue, result[key], "key %s should have value %s", key, expectedValue)
			}
			// Ensure OS env is not included (PATH should not be in result)
			_, hasPath := result["PATH"]
			assert.False(t, hasPath, "UserEnvsMap should not include OS environment variables like PATH")
		})
	}
}
