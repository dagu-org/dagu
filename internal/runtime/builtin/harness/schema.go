// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"github.com/dagucloud/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

var configSchema = &jsonschema.Schema{
	Type:     "object",
	Required: []string{"provider"},
	Properties: map[string]*jsonschema.Schema{
		// Common
		"provider":      {Type: "string", Enum: []any{"claude", "codex", "opencode", "pi"}, Description: "Coding agent CLI provider"},
		"model":         {Type: "string", Description: "Provider-specific model name"},
		"effort":        {Type: "string", Enum: []any{"low", "medium", "high", "max"}, Description: "Effort level (mapped per provider)"},
		"max_turns":     {Type: "integer", Description: "Max agentic iterations"},
		"output_format": {Type: "string", Enum: []any{"text", "json", "stream-json"}, Description: "Output format"},
		// Claude-specific
		"allowed_tools":        {Type: "string", Description: "Claude: comma-separated allowed tools"},
		"disallowed_tools":     {Type: "string", Description: "Claude: comma-separated disallowed tools"},
		"permission_mode":      {Type: "string", Description: "Claude: permission mode"},
		"system_prompt":        {Type: "string", Description: "Claude: system prompt override"},
		"append_system_prompt": {Type: "string", Description: "Claude: text appended to system prompt"},
		"max_budget_usd":       {Type: "number", Description: "Claude: max budget in USD"},
		"bare":                 {Type: "boolean", Description: "Claude: skip auto-discovery for fast startup"},
		"add_dir":              {Type: "string", Description: "Claude: grant access to additional directory"},
		"worktree":             {Type: "boolean", Description: "Claude: run in isolated git worktree"},
		// Codex-specific
		"sandbox":             {Type: "string", Description: "Codex: sandbox mode (read-only, workspace-write, danger-full-access)"},
		"full_auto":           {Type: "boolean", Description: "Codex: full auto mode"},
		"output_schema":       {Type: "string", Description: "Codex: JSON schema for structured output"},
		"ephemeral":           {Type: "boolean", Description: "Codex: ephemeral mode"},
		"skip_git_repo_check": {Type: "boolean", Description: "Codex: skip git repo check"},
		// OpenCode-specific
		"agent": {Type: "string", Description: "OpenCode: agent name"},
		"file":  {Type: "string", Description: "OpenCode: input file"},
		"title": {Type: "string", Description: "OpenCode: session title"},
		// Pi-specific
		"thinking":      {Type: "string", Description: "Pi: thinking level (off, minimal, low, medium, high, xhigh)"},
		"pi_provider":   {Type: "string", Description: "Pi: LLM provider name (anthropic, openai, etc.)"},
		"tools":         {Type: "string", Description: "Pi: comma-separated tool list"},
		"no_tools":      {Type: "boolean", Description: "Pi: disable all tools"},
		"no_extensions": {Type: "boolean", Description: "Pi: disable extension auto-discovery"},
		"session":       {Type: "string", Description: "Pi: session ID"},
		// Escape hatch
		"extra_flags": {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Additional CLI flags passed directly"},
	},
}

func init() {
	core.RegisterExecutorConfigSchema("harness", configSchema)
}
