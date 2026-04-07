// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"testing"

	"github.com/dagucloud/dagu/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestGenerateSystemPrompt(t *testing.T) {
	t.Parallel()

	t.Run("includes environment info", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{
			DAGsDir:        "/dags",
			DocsDir:        "/dags/docs",
			LogDir:         "/logs",
			WorkingDir:     "/work",
			BaseConfigFile: "/config/base.yaml",
		}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleDeveloper})

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "/dags")
		assert.Contains(t, result, "/dags/docs")
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

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, CurrentDAG: dag, Role: auth.RoleAdmin})

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "test-dag")
		assert.Contains(t, result, "Authenticated role: admin")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		result := GenerateSystemPrompt(SystemPromptParams{Role: auth.RoleViewer})

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Authenticated role: viewer")
	})

	t.Run("no memory omits memory section", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleViewer})

		assert.NotContains(t, result, "<global_memory>")
		assert.NotContains(t, result, "<dag_memory")
		assert.NotContains(t, result, "<automata_memory")
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

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Memory: mem, Role: auth.RoleViewer})

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

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Memory: mem, Role: auth.RoleViewer})

		assert.Contains(t, result, "<global_memory>")
		assert.Contains(t, result, "Global info.")
		assert.Contains(t, result, `<dag_memory dag="my-etl">`)
		assert.Contains(t, result, "DAG-specific info.")
	})

	t.Run("includes automata memory", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			GlobalMemory:   "Global info.",
			AutomataMemory: "Automata-specific operating rules.",
			AutomataName:   "queue_worker",
			MemoryDir:      "/dags/memory",
		}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Memory: mem, Role: auth.RoleViewer})

		assert.Contains(t, result, "<global_memory>")
		assert.Contains(t, result, `<automata_memory automata="queue_worker">`)
		assert.Contains(t, result, "Automata-specific operating rules.")
	})

	t.Run("memory paths appear in output", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			MemoryDir: "/dags/memory",
			DAGName:   "test-dag",
		}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Memory: mem, Role: auth.RoleViewer})

		assert.Contains(t, result, "/dags/memory/MEMORY.md")
		assert.Contains(t, result, "/dags/memory/dags/test-dag/MEMORY.md")
	})

	t.Run("automata memory paths appear in output", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			MemoryDir:    "/dags/memory",
			AutomataName: "queue_worker",
		}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Memory: mem, Role: auth.RoleViewer})

		assert.Contains(t, result, "/dags/memory/MEMORY.md")
		assert.Contains(t, result, "/dags/memory/automata/queue_worker/MEMORY.md")
	})

	t.Run("memory management enforces DAG-first policy", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			MemoryDir: "/dags/memory",
			DAGName:   "new-etl",
		}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Memory: mem, Role: auth.RoleViewer})

		assert.Contains(t, result, "If DAG context is available, save memory to Per-DAG by default (not Global)")
		assert.Contains(t, result, "If Automata context is available, save memory to Per-Automata by default (not Global)")
		assert.Contains(t, result, "After creating or updating a DAG, if anything should be remembered, create/update that DAG's memory file")
		assert.Contains(t, result, "After updating an Automata's long-lived behavior, operating rules, or learned procedures, create/update that Automata's memory file")
		assert.Contains(t, result, "Global memory is only for cross-DAG or user-wide stable preferences/policies")
	})

	t.Run("memory management requires confirmation before global write without DAG context", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		mem := MemoryContent{
			MemoryDir: "/dags/memory",
		}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Memory: mem, Role: auth.RoleViewer})

		assert.Contains(t, result, "If no DAG or Automata context is available, ask the user before writing to Global memory")
	})

	t.Run("lists skills individually when under threshold", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		skills := []SkillSummary{
			{ID: "sql-optimizer", Name: "SQL Optimizer", Description: "Optimizes SQL queries"},
			{ID: "docker-deploy", Name: "Docker Deployment", Description: "Container best practices"},
		}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleViewer, AvailableSkills: skills, SkillCount: 2})

		assert.Contains(t, result, "<available_skills>")
		assert.Contains(t, result, "sql-optimizer")
		assert.Contains(t, result, "docker-deploy")
		assert.Contains(t, result, "Use `use_skill`")
		assert.NotContains(t, result, "You have access to")
	})

	t.Run("shows count only when above threshold", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleViewer, SkillCount: 100})

		assert.Contains(t, result, "<available_skills>")
		assert.Contains(t, result, "You have access to 100 skills")
		assert.Contains(t, result, "search_skills")
		assert.NotContains(t, result, "Use `use_skill` with the skill ID")
	})

	t.Run("omits skills section when no skills", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleViewer})

		assert.NotContains(t, result, "<available_skills>")
		assert.NotContains(t, result, "<skill_delegation>")
	})

	t.Run("includes skill delegation guidance when skills available", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		skills := []SkillSummary{{ID: "test", Name: "Test Skill"}}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleViewer, AvailableSkills: skills, SkillCount: 1})

		assert.Contains(t, result, "<skill_delegation>")
		assert.Contains(t, result, "delegate")
		assert.Contains(t, result, "use_skill")
	})

	t.Run("includes skill delegation guidance when only skill count", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleViewer, SkillCount: 50})

		assert.Contains(t, result, "<skill_delegation>")
	})

	t.Run("includes soul content when provided", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		soul := &Soul{Content: "test soul identity"}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleViewer, Soul: soul})

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "test soul identity")
	})

	t.Run("template-like syntax in soul content is not evaluated", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}
		soul := &Soul{Content: "You are {{.Name}} and use {{template}} things"}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleViewer, Soul: soul})

		assert.NotEmpty(t, result)
		// The literal template syntax must appear in output, not be evaluated.
		assert.Contains(t, result, "You are {{.Name}} and use {{template}} things")
		// The identity tag must be present (soul content is rendered).
		assert.Contains(t, result, "<identity>")
		// Fallback prompt must NOT be used.
		assert.NotContains(t, result, "You are Dagu Assistant, an AI assistant")
	})

	t.Run("execution guidance prefers enqueue without preflight checks", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleDeveloper})

		assert.Contains(t, result, "Default to queue-based execution: `dagu enqueue <dag-name>`")
		assert.Contains(t, result, "Do not check running jobs, queued jobs")
		assert.Contains(t, result, "pass user parameters with `-p`")
		assert.Contains(t, result, `dagu enqueue my-dag -p 'topic="OpenAI new model released March 2026"'`)
		assert.Contains(t, result, "Avoid passing spaced values after `--`")
		assert.NotContains(t, result, "2. Start: `dagu start <dag-name>`")
	})

	t.Run("includes active progress reporting guidance", func(t *testing.T) {
		t.Parallel()
		env := EnvironmentInfo{DAGsDir: "/dags"}

		result := GenerateSystemPrompt(SystemPromptParams{Env: env, Role: auth.RoleDeveloper})

		assert.Contains(t, result, "<communication>")
		assert.Contains(t, result, "Actively report your progress during multi-step work")
		assert.Contains(t, result, "Before using tools or starting a long-running action")
		assert.Contains(t, result, "Do not stay silent until the final answer")
		assert.Contains(t, result, "what you did, what you found, and what you will do next")
	})
}

func TestFallbackPrompt(t *testing.T) {
	t.Parallel()

	t.Run("includes DAGs directory", func(t *testing.T) {
		t.Parallel()

		result := fallbackPrompt(EnvironmentInfo{DAGsDir: "/my/dags"})

		assert.Contains(t, result, "/my/dags")
		assert.Contains(t, result, "Dagu Assistant")
	})

	t.Run("works with empty environment", func(t *testing.T) {
		t.Parallel()

		result := fallbackPrompt(EnvironmentInfo{})

		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Dagu Assistant")
	})
}
