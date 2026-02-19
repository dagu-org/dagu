package agent

import (
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
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

		result := GenerateSystemPrompt(env, nil, MemoryContent{}, auth.RoleDeveloper, nil, 0)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "/dags")
		assert.Contains(t, result, "/config/base.yaml")
		assert.Contains(t, result, "Authenticated role: developer")
	})

	t.Run("includes current DAG context", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		dag := &CurrentDAG{
			Name:     "test-dag",
			FilePath: "/dags/test-dag.yaml",
		}

		result := GenerateSystemPrompt(env, dag, MemoryContent{}, auth.RoleAdmin, nil, 0)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "test-dag")
		assert.Contains(t, result, "Authenticated role: admin")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		result := GenerateSystemPrompt(EnvironmentInfo{}, nil, MemoryContent{}, auth.RoleViewer, nil, 0)

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Authenticated role: viewer")
	})

	t.Run("no memory omits memory section", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(env, nil, MemoryContent{}, auth.RoleViewer, nil, 0)

		assert.NotContains(t, result, "<global_memory>")
		assert.NotContains(t, result, "<dag_memory")
		assert.NotContains(t, result, "<memory_paths>")
		assert.NotContains(t, result, "<memory_management>")
	})

	t.Run("includes global memory only", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			GlobalMemory: "User prefers concise output.",
			MemoryDir:    "/dags/memory",
		}

		result := GenerateSystemPrompt(env, nil, mem, auth.RoleViewer, nil, 0)

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

		result := GenerateSystemPrompt(env, nil, mem, auth.RoleViewer, nil, 0)

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

		result := GenerateSystemPrompt(env, nil, mem, auth.RoleViewer, nil, 0)

		assert.Contains(t, result, "/dags/memory/MEMORY.md")
		assert.Contains(t, result, "/dags/memory/dags/test-dag/MEMORY.md")
	})

	t.Run("memory management enforces DAG-first policy", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			MemoryDir: "/dags/memory",
			DAGName:   "new-etl",
		}

		result := GenerateSystemPrompt(env, nil, mem, auth.RoleViewer, nil, 0)

		assert.Contains(t, result, "If DAG context is available, save memory to Per-DAG by default (not Global)")
		assert.Contains(t, result, "After creating or updating a DAG, if anything should be remembered, create/update that DAG's memory file")
		assert.Contains(t, result, "Global memory is only for cross-DAG or user-wide stable preferences/policies")
	})

	t.Run("memory management requires confirmation before global write without DAG context", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			MemoryDir: "/dags/memory",
		}

		result := GenerateSystemPrompt(env, nil, mem, auth.RoleViewer, nil, 0)

		assert.Contains(t, result, "If no DAG context is available, ask the user before writing to Global memory")
	})

	t.Run("lists skills individually when under threshold", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		skills := []SkillSummary{
			{ID: "sql-optimizer", Name: "SQL Optimizer", Description: "Optimizes SQL queries"},
			{ID: "docker-deploy", Name: "Docker Deployment", Description: "Container best practices"},
		}

		result := GenerateSystemPrompt(env, nil, MemoryContent{}, auth.RoleViewer, skills, 2)

		assert.Contains(t, result, "<available_skills>")
		assert.Contains(t, result, "sql-optimizer")
		assert.Contains(t, result, "docker-deploy")
		assert.Contains(t, result, "Use `use_skill`")
		assert.NotContains(t, result, "You have access to")
	})

	t.Run("shows count only when above threshold", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(env, nil, MemoryContent{}, auth.RoleViewer, nil, 100)

		assert.Contains(t, result, "<available_skills>")
		assert.Contains(t, result, "You have access to 100 skills")
		assert.Contains(t, result, "search_skills")
		assert.NotContains(t, result, "use_skill")
	})

	t.Run("omits skills section when no skills", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(env, nil, MemoryContent{}, auth.RoleViewer, nil, 0)

		assert.NotContains(t, result, "<available_skills>")
	})
}

func TestFallbackPrompt(t *testing.T) {
	t.Parallel()

	t.Run("includes DAGs directory", func(t *testing.T) {
		t.Parallel()

		result := fallbackPrompt(EnvironmentInfo{DAGsDir: "/my/dags"})

		assert.Contains(t, result, "/my/dags")
		assert.Contains(t, result, "Tsumugi")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		result := fallbackPrompt(EnvironmentInfo{})

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Tsumugi")
	})
}
