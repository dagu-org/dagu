package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateSystemPrompt(t *testing.T) {
	t.Parallel()

	t.Run("includes environment info", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{
			DAGsDir:        "/dags",
			LogDir:         "/logs",
			WorkingDir:     "/work",
			BaseConfigFile: "/config/base.yaml",
		}

		result := GenerateSystemPrompt(env, nil)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "/dags")
		assert.Contains(t, result, "/config/base.yaml")
	})

	t.Run("includes current DAG context", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		dag := &CurrentDAG{
			Name:     "test-dag",
			FilePath: "/dags/test-dag.yaml",
		}

		result := GenerateSystemPrompt(env, dag)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "test-dag")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		result := GenerateSystemPrompt(EnvironmentInfo{}, nil)

		assert.NotEmpty(t, result)
	})
}

func TestFallbackPrompt(t *testing.T) {
	t.Parallel()

	t.Run("includes DAGs directory", func(t *testing.T) {
		t.Parallel()

		result := fallbackPrompt(EnvironmentInfo{DAGsDir: "/my/dags"})

		assert.Contains(t, result, "/my/dags")
		assert.Contains(t, result, "Hermio")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		result := fallbackPrompt(EnvironmentInfo{})

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Hermio")
	})
}
