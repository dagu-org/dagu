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

		result := GenerateSystemPrompt(env, nil, MemoryContent{})

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

		result := GenerateSystemPrompt(env, dag, MemoryContent{})

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "test-dag")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		result := GenerateSystemPrompt(EnvironmentInfo{}, nil, MemoryContent{})

		assert.NotEmpty(t, result)
	})

	t.Run("no memory omits memory section", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(env, nil, MemoryContent{})

		assert.NotContains(t, result, "<global_memory>")
		assert.NotContains(t, result, "<dag_memory")
	})

	t.Run("includes global memory only", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			GlobalMemory: "User prefers concise output.",
			MemoryDir:    "/dags/memory",
		}

		result := GenerateSystemPrompt(env, nil, mem)

		assert.Contains(t, result, "<global_memory>")
		assert.Contains(t, result, "User prefers concise output.")
		assert.NotContains(t, result, "<dag_memory")
	})

	t.Run("includes both global and DAG memory", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			GlobalMemory: "Global info.",
			DAGMemory:    "DAG-specific info.",
			DAGName:      "my-etl",
			MemoryDir:    "/dags/memory",
		}

		result := GenerateSystemPrompt(env, nil, mem)

		assert.Contains(t, result, "<global_memory>")
		assert.Contains(t, result, "Global info.")
		assert.Contains(t, result, `<dag_memory dag="my-etl">`)
		assert.Contains(t, result, "DAG-specific info.")
	})

	t.Run("memory paths appear in output", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			MemoryDir: "/dags/memory",
			DAGName:   "test-dag",
		}

		result := GenerateSystemPrompt(env, nil, mem)

		assert.Contains(t, result, "/dags/memory/MEMORY.md")
		assert.Contains(t, result, "/dags/memory/dags/test-dag/MEMORY.md")
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
