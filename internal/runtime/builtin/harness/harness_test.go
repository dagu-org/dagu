// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"testing"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeProvider_BuildArgs(t *testing.T) {
	p := &claudeProvider{}

	t.Run("minimal", func(t *testing.T) {
		args := p.BuildArgs(&harnessConfig{}, "hello")
		assert.Equal(t, []string{"-p", "hello"}, args)
	})

	t.Run("full", func(t *testing.T) {
		cfg := &harnessConfig{
			Model:              "sonnet",
			Effort:             "high",
			MaxTurns:           20,
			OutputFormat:       "json",
			MaxBudgetUSD:       5.0,
			PermissionMode:     "auto",
			AllowedTools:       "Bash,Read,Edit",
			DisallowedTools:    "Write",
			SystemPrompt:       "be concise",
			AppendSystemPrompt: "extra context",
			Bare:               true,
			AddDir:             "/tmp/extra",
			Worktree:           true,
		}
		args := p.BuildArgs(cfg, "Write tests")
		assert.Contains(t, args, "-p")
		assert.Contains(t, args, "Write tests")
		assert.Contains(t, args, "--model")
		assert.Contains(t, args, "sonnet")
		assert.Contains(t, args, "--effort")
		assert.Contains(t, args, "high")
		assert.Contains(t, args, "--max-turns")
		assert.Contains(t, args, "20")
		assert.Contains(t, args, "--output-format")
		assert.Contains(t, args, "json")
		assert.Contains(t, args, "--max-budget-usd")
		assert.Contains(t, args, "5")
		assert.Contains(t, args, "--permission-mode")
		assert.Contains(t, args, "auto")
		assert.Contains(t, args, "--allowedTools")
		assert.Contains(t, args, "Bash,Read,Edit")
		assert.Contains(t, args, "--disallowedTools")
		assert.Contains(t, args, "Write")
		assert.Contains(t, args, "--system-prompt")
		assert.Contains(t, args, "be concise")
		assert.Contains(t, args, "--append-system-prompt")
		assert.Contains(t, args, "extra context")
		assert.Contains(t, args, "--bare")
		assert.Contains(t, args, "--add-dir")
		assert.Contains(t, args, "/tmp/extra")
		assert.Contains(t, args, "--worktree")
	})

	t.Run("stream-json", func(t *testing.T) {
		cfg := &harnessConfig{OutputFormat: "stream-json"}
		args := p.BuildArgs(cfg, "prompt")
		assert.Contains(t, args, "--output-format")
		assert.Contains(t, args, "stream-json")
	})

	t.Run("extra_flags", func(t *testing.T) {
		cfg := &harnessConfig{ExtraFlags: []string{"--verbose", "--no-session-persistence"}}
		args := p.BuildArgs(cfg, "prompt")
		assert.Contains(t, args, "--verbose")
		assert.Contains(t, args, "--no-session-persistence")
	})
}

func TestCodexProvider_BuildArgs(t *testing.T) {
	p := &codexProvider{}

	t.Run("minimal", func(t *testing.T) {
		args := p.BuildArgs(&harnessConfig{}, "hello")
		assert.Equal(t, []string{"exec", "hello"}, args)
	})

	t.Run("full", func(t *testing.T) {
		cfg := &harnessConfig{
			Model:        "gpt-5.4",
			OutputFormat: "json",
			FullAuto:     true,
			Sandbox:      "workspace-write",
			OutputSchema: "/tmp/schema.json",
			Ephemeral:    true,
			SkipGitCheck: true,
		}
		args := p.BuildArgs(cfg, "Fix bug")
		assert.Contains(t, args, "exec")
		assert.Contains(t, args, "Fix bug")
		assert.Contains(t, args, "--model")
		assert.Contains(t, args, "gpt-5.4")
		assert.Contains(t, args, "--json")
		assert.Contains(t, args, "--full-auto")
		assert.Contains(t, args, "--sandbox")
		assert.Contains(t, args, "workspace-write")
		assert.Contains(t, args, "--output-schema")
		assert.Contains(t, args, "/tmp/schema.json")
		assert.Contains(t, args, "--ephemeral")
		assert.Contains(t, args, "--skip-git-repo-check")
	})

	t.Run("effort_high_maps_to_full_auto", func(t *testing.T) {
		cfg := &harnessConfig{Effort: "high"}
		args := p.BuildArgs(cfg, "task")
		assert.Contains(t, args, "--full-auto")
	})

	t.Run("effort_max_maps_to_full_auto", func(t *testing.T) {
		cfg := &harnessConfig{Effort: "max"}
		args := p.BuildArgs(cfg, "task")
		assert.Contains(t, args, "--full-auto")
	})

	t.Run("effort_low_no_full_auto", func(t *testing.T) {
		cfg := &harnessConfig{Effort: "low"}
		args := p.BuildArgs(cfg, "task")
		assert.NotContains(t, args, "--full-auto")
	})
}

func TestOpenCodeProvider_BuildArgs(t *testing.T) {
	p := &opencodeProvider{}

	t.Run("minimal", func(t *testing.T) {
		args := p.BuildArgs(&harnessConfig{}, "hello")
		assert.Equal(t, []string{"run", "hello"}, args)
	})

	t.Run("full", func(t *testing.T) {
		cfg := &harnessConfig{
			Model:        "anthropic/claude-sonnet",
			OutputFormat: "json",
			File:         "context.txt",
			Agent:        "coder",
			Title:        "my session",
		}
		args := p.BuildArgs(cfg, "Refactor")
		assert.Contains(t, args, "run")
		assert.Contains(t, args, "Refactor")
		assert.Contains(t, args, "--model")
		assert.Contains(t, args, "anthropic/claude-sonnet")
		assert.Contains(t, args, "--format")
		assert.Contains(t, args, "json")
		assert.Contains(t, args, "--file")
		assert.Contains(t, args, "context.txt")
		assert.Contains(t, args, "--agent")
		assert.Contains(t, args, "coder")
		assert.Contains(t, args, "--title")
		assert.Contains(t, args, "my session")
	})
}

func TestPiProvider_BuildArgs(t *testing.T) {
	p := &piProvider{}

	t.Run("minimal", func(t *testing.T) {
		args := p.BuildArgs(&harnessConfig{}, "hello")
		assert.Equal(t, []string{"-p", "hello"}, args)
	})

	t.Run("full", func(t *testing.T) {
		cfg := &harnessConfig{
			PiProvider:   "anthropic",
			Model:        "claude-sonnet-4-20250514",
			OutputFormat: "json",
			Thinking:     "high",
			Tools:        "read,bash",
			Session:      "abc-123",
		}
		args := p.BuildArgs(cfg, "Analyze")
		assert.Contains(t, args, "-p")
		assert.Contains(t, args, "Analyze")
		assert.Contains(t, args, "--provider")
		assert.Contains(t, args, "anthropic")
		assert.Contains(t, args, "--model")
		assert.Contains(t, args, "claude-sonnet-4-20250514")
		assert.Contains(t, args, "--mode")
		assert.Contains(t, args, "json")
		assert.Contains(t, args, "--thinking")
		assert.Contains(t, args, "high")
		assert.Contains(t, args, "--tools")
		assert.Contains(t, args, "read,bash")
		assert.Contains(t, args, "--session")
		assert.Contains(t, args, "abc-123")
	})

	t.Run("no_tools_and_no_extensions", func(t *testing.T) {
		cfg := &harnessConfig{NoTools: true, NoExtensions: true}
		args := p.BuildArgs(cfg, "prompt")
		assert.Contains(t, args, "--no-tools")
		assert.Contains(t, args, "--no-extensions")
	})

	t.Run("effort_maps_to_thinking", func(t *testing.T) {
		cfg := &harnessConfig{Effort: "max"}
		args := p.BuildArgs(cfg, "prompt")
		assert.Contains(t, args, "--thinking")
		assert.Contains(t, args, "xhigh")
	})

	t.Run("explicit_thinking_overrides_effort", func(t *testing.T) {
		cfg := &harnessConfig{Effort: "max", Thinking: "low"}
		args := p.BuildArgs(cfg, "prompt")
		assert.Contains(t, args, "--thinking")
		assert.Contains(t, args, "low")
		assert.NotContains(t, args, "xhigh")
	})
}

func TestMapEffortToThinking(t *testing.T) {
	tests := []struct {
		effort   string
		expected string
	}{
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"max", "xhigh"},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.effort, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapEffortToThinking(tt.effort))
		})
	}
}

func TestDecodeConfig(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		data := map[string]any{
			"provider":      "claude",
			"model":         "sonnet",
			"max_turns":     20,
			"output_format": "json",
			"bare":          true,
		}
		var cfg harnessConfig
		err := decodeConfig(data, &cfg)
		require.NoError(t, err)
		assert.Equal(t, "claude", cfg.Provider)
		assert.Equal(t, "sonnet", cfg.Model)
		assert.Equal(t, 20, cfg.MaxTurns)
		assert.Equal(t, "json", cfg.OutputFormat)
		assert.True(t, cfg.Bare)
	})

	t.Run("weakly_typed_max_turns", func(t *testing.T) {
		data := map[string]any{
			"provider":  "codex",
			"max_turns": "10",
		}
		var cfg harnessConfig
		err := decodeConfig(data, &cfg)
		require.NoError(t, err)
		assert.Equal(t, 10, cfg.MaxTurns)
	})

	t.Run("extra_flags", func(t *testing.T) {
		data := map[string]any{
			"provider":    "claude",
			"extra_flags": []any{"--verbose", "--no-session-persistence"},
		}
		var cfg harnessConfig
		err := decodeConfig(data, &cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"--verbose", "--no-session-persistence"}, cfg.ExtraFlags)
	})
}

func TestValidateHarnessStep(t *testing.T) {
	t.Run("missing_commands", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "claude"}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command field (prompt) is required")
	})

	t.Run("missing_config", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands: []core.CommandEntry{{Command: "prompt"}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config is required")
	})

	t.Run("missing_provider", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider is required")
	})

	t.Run("unknown_provider", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "unknown"}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
	})

	t.Run("valid_claude", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "claude"}},
		})
		assert.NoError(t, err)
	})

	t.Run("valid_codex", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "codex"}},
		})
		assert.NoError(t, err)
	})

	t.Run("valid_opencode", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "opencode"}},
		})
		assert.NoError(t, err)
	})

	t.Run("valid_pi", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "pi"}},
		})
		assert.NoError(t, err)
	})
}

func TestExtractPrompt(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Equal(t, "", extractPrompt(core.Step{}))
	})

	t.Run("cmd_with_args", func(t *testing.T) {
		step := core.Step{
			Commands: []core.CommandEntry{{CmdWithArgs: "Write tests for auth"}},
		}
		assert.Equal(t, "Write tests for auth", extractPrompt(step))
	})

	t.Run("command_only", func(t *testing.T) {
		step := core.Step{
			Commands: []core.CommandEntry{{Command: "Refactor"}},
		}
		assert.Equal(t, "Refactor", extractPrompt(step))
	})

	t.Run("command_with_args", func(t *testing.T) {
		step := core.Step{
			Commands: []core.CommandEntry{{Command: "analyze", Args: []string{"--deep", "src/"}}},
		}
		assert.Equal(t, "analyze --deep src/", extractPrompt(step))
	})
}

func TestGetProvider(t *testing.T) {
	t.Run("claude", func(t *testing.T) {
		p, err := getProvider("claude")
		require.NoError(t, err)
		assert.Equal(t, "claude", p.Name())
		assert.Equal(t, "claude", p.BinaryName())
	})

	t.Run("codex", func(t *testing.T) {
		p, err := getProvider("codex")
		require.NoError(t, err)
		assert.Equal(t, "codex", p.Name())
		assert.Equal(t, "codex", p.BinaryName())
	})

	t.Run("opencode", func(t *testing.T) {
		p, err := getProvider("opencode")
		require.NoError(t, err)
		assert.Equal(t, "opencode", p.Name())
		assert.Equal(t, "opencode", p.BinaryName())
	})

	t.Run("pi", func(t *testing.T) {
		p, err := getProvider("pi")
		require.NoError(t, err)
		assert.Equal(t, "pi", p.Name())
		assert.Equal(t, "pi", p.BinaryName())
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := getProvider("unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
	})
}

func TestExitCodeFromError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Equal(t, 0, exitCodeFromError(nil))
	})

	t.Run("generic_error", func(t *testing.T) {
		assert.Equal(t, 1, exitCodeFromError(assert.AnError))
	})
}
