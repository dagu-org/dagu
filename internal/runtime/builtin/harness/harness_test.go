// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"testing"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderBaseArgs(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		prompt   string
		expected []string
	}{
		{"claude", &claudeProvider{}, "hello", []string{"-p", "hello"}},
		{"codex", &codexProvider{}, "hello", []string{"exec", "hello"}},
		{"copilot", &copilotProvider{}, "hello", []string{"-p", "hello"}},
		{"opencode", &opencodeProvider{}, "hello", []string{"run", "hello"}},
		{"pi", &piProvider{}, "hello", []string{"-p", "hello"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.provider.BaseArgs(tt.prompt))
			assert.Equal(t, tt.name, tt.provider.Name())
		})
	}
}

func TestConfigToFlags(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, configToFlags(nil))
	})

	t.Run("provider_skipped", func(t *testing.T) {
		flags := configToFlags(map[string]any{"provider": "claude"})
		assert.Empty(t, flags)
	})

	t.Run("string_value", func(t *testing.T) {
		flags := configToFlags(map[string]any{"model": "sonnet"})
		assert.Equal(t, []string{"--model", "sonnet"}, flags)
	})

	t.Run("empty_string_skipped", func(t *testing.T) {
		flags := configToFlags(map[string]any{"model": ""})
		assert.Empty(t, flags)
	})

	t.Run("bool_true", func(t *testing.T) {
		flags := configToFlags(map[string]any{"bare": true})
		assert.Equal(t, []string{"--bare"}, flags)
	})

	t.Run("bool_false_skipped", func(t *testing.T) {
		flags := configToFlags(map[string]any{"bare": false})
		assert.Empty(t, flags)
	})

	t.Run("integer", func(t *testing.T) {
		flags := configToFlags(map[string]any{"max-turns": float64(20)})
		assert.Equal(t, []string{"--max-turns", "20"}, flags)
	})

	t.Run("float", func(t *testing.T) {
		flags := configToFlags(map[string]any{"max-budget-usd": float64(5.5)})
		assert.Equal(t, []string{"--max-budget-usd", "5.5"}, flags)
	})

	t.Run("array_repeated", func(t *testing.T) {
		flags := configToFlags(map[string]any{
			"allow-tool": []any{"shell(git:*)", "write"},
		})
		assert.Equal(t, []string{"--allow-tool", "shell(git:*)", "--allow-tool", "write"}, flags)
	})

	t.Run("sorted_keys", func(t *testing.T) {
		flags := configToFlags(map[string]any{
			"provider": "claude",
			"model":    "sonnet",
			"bare":     true,
			"effort":   "high",
		})
		assert.Equal(t, []string{"--bare", "--effort", "high", "--model", "sonnet"}, flags)
	})

	t.Run("claude_example", func(t *testing.T) {
		flags := configToFlags(map[string]any{
			"provider":     "claude",
			"model":        "sonnet",
			"effort":       "high",
			"bare":         true,
			"allowedTools": "Bash,Read,Edit",
		})
		assert.Equal(t, []string{
			"--allowedTools", "Bash,Read,Edit",
			"--bare",
			"--effort", "high",
			"--model", "sonnet",
		}, flags)
	})

	t.Run("copilot_example", func(t *testing.T) {
		flags := configToFlags(map[string]any{
			"provider":  "copilot",
			"model":     "gpt-5.2",
			"autopilot": true,
			"yolo":      true,
			"silent":    true,
		})
		assert.Equal(t, []string{
			"--autopilot",
			"--model", "gpt-5.2",
			"--silent",
			"--yolo",
		}, flags)
	})
}

func TestValidateHarnessStep(t *testing.T) {
	t.Run("missing_commands", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "claude"}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prompt")
	})

	t.Run("empty_prompt", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: ""}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "claude"}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "prompt")
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
		assert.Contains(t, err.Error(), "provider")
	})

	t.Run("unknown_provider", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "unknown"}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
	})

	for _, p := range []string{"claude", "codex", "copilot", "opencode", "pi"} {
		t.Run("valid_"+p, func(t *testing.T) {
			err := validateHarnessStep(core.Step{
				Commands:       []core.CommandEntry{{Command: "prompt"}},
				ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": p}},
			})
			assert.NoError(t, err)
		})
	}
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
	for _, name := range []string{"claude", "codex", "copilot", "opencode", "pi"} {
		t.Run(name, func(t *testing.T) {
			p, err := getProvider(name)
			require.NoError(t, err)
			assert.Equal(t, name, p.Name())
		})
	}

	t.Run("unknown", func(t *testing.T) {
		_, err := getProvider("unknown")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
	})
}

func TestExitCodeFromError(t *testing.T) {
	assert.Equal(t, 0, exitCodeFromError(nil))
	assert.Equal(t, 1, exitCodeFromError(assert.AnError))
}
