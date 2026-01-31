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
			DAGsDir:    "/dags",
			LogDir:     "/logs",
			WorkingDir: "/work",
		}

		prompt := GenerateSystemPrompt(env, nil)

		assert.NotEmpty(t, prompt)
		assert.Contains(t, prompt, "/dags")
	})

	t.Run("includes current DAG context", func(t *testing.T) {
		t.Parallel()

		env := EnvironmentInfo{DAGsDir: "/dags"}
		dag := &CurrentDAG{
			Name:     "test-dag",
			FilePath: "/dags/test-dag.yaml",
		}

		prompt := GenerateSystemPrompt(env, dag)

		assert.NotEmpty(t, prompt)
		// The exact content depends on the template, but it should include the DAG info
		assert.Contains(t, prompt, "test-dag")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		env := EnvironmentInfo{}
		prompt := GenerateSystemPrompt(env, nil)

		assert.NotEmpty(t, prompt)
	})
}

func TestFallbackPrompt(t *testing.T) {
	t.Parallel()

	t.Run("includes DAGs directory", func(t *testing.T) {
		t.Parallel()

		env := EnvironmentInfo{DAGsDir: "/my/dags"}
		result := fallbackPrompt(env)

		assert.Contains(t, result, "/my/dags")
		assert.Contains(t, result, "Hermio")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		env := EnvironmentInfo{}
		result := fallbackPrompt(env)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Hermio")
	})
}
