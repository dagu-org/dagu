// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

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
	t.Run("reserved_keys_skipped", func(t *testing.T) {
		flags := configToFlags(map[string]any{
			"provider":    "claude",
			"binary":      "gemini",
			"prompt_args": []any{"-p"},
			"fallback": []any{
				map[string]any{"provider": "codex"},
			},
		})
		assert.Empty(t, flags)
	})

	t.Run("bool_number_and_array", func(t *testing.T) {
		flags := configToFlags(map[string]any{
			"bare":           true,
			"max-turns":      20,
			"max-budget-usd": 5.5,
			"allow-tool":     []any{"shell(git:*)", "write"},
		})
		assert.Equal(t, []string{
			"--allow-tool", "shell(git:*)",
			"--allow-tool", "write",
			"--bare",
			"--max-budget-usd", "5.5",
			"--max-turns", "20",
		}, flags)
	})
}

func TestNormalizeConfigMap(t *testing.T) {
	cfg := normalizeConfigMap(map[string]any{
		"provider":    "${PROVIDER}",
		"bare":        "true",
		"max-turns":   "10",
		"temperature": "5.5",
		"model":       "sonnet",
		"fallback": []any{
			map[string]any{
				"provider":  "${FALLBACK_PROVIDER}",
				"full-auto": "true",
			},
		},
	})

	assert.Equal(t, "${PROVIDER}", cfg["provider"])
	assert.Equal(t, true, cfg["bare"])
	assert.EqualValues(t, 10, cfg["max-turns"])
	assert.EqualValues(t, 5.5, cfg["temperature"])
	assert.Equal(t, "sonnet", cfg["model"])

	fallback := mustFallback(t, cfg["fallback"])
	require.Len(t, fallback, 1)
	assert.Equal(t, "${FALLBACK_PROVIDER}", fallback[0]["provider"])
	assert.Equal(t, true, fallback[0]["full-auto"])
}

func TestExtractFallbackConfigs(t *testing.T) {
	primary, fallback, err := extractFallbackConfigs(map[string]any{
		"provider": "claude",
		"model":    "sonnet",
		"fallback": []any{
			map[string]any{"provider": "codex", "full-auto": true},
			map[string]any{"provider": "copilot", "silent": true},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"provider": "claude",
		"model":    "sonnet",
	}, primary)
	require.Len(t, fallback, 2)
	assert.Equal(t, "codex", fallback[0]["provider"])
	assert.Equal(t, "copilot", fallback[1]["provider"])
}

func TestValidateHarnessStep(t *testing.T) {
	t.Run("missing_prompt", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
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

	t.Run("missing_provider_and_binary", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider or config.binary")
	})

	t.Run("both_provider_and_binary", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands: []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{
				"provider": "claude",
				"binary":   "gemini",
			}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "either provider or binary")
	})

	t.Run("unknown_literal_provider", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "unknown"}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown provider")
	})

	t.Run("templated_provider_allowed", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands:       []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{"provider": "${PROVIDER}"}},
		})
		assert.NoError(t, err)
	})

	t.Run("templated_fallback_provider_allowed", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands: []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{
				"provider": "claude",
				"fallback": []any{
					map[string]any{"provider": "${FALLBACK_PROVIDER}"},
				},
			}},
		})
		assert.NoError(t, err)
	})

	t.Run("invalid_fallback_shape", func(t *testing.T) {
		err := validateHarnessStep(core.Step{
			Commands: []core.CommandEntry{{Command: "prompt"}},
			ExecutorConfig: core.ExecutorConfig{Config: map[string]any{
				"provider": "claude",
				"fallback": []any{"codex"},
			}},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "fallback[0]")
	})
}

func TestResolveProvider(t *testing.T) {
	t.Run("builtin", func(t *testing.T) {
		p, err := resolveProvider(map[string]any{"provider": "claude"})
		require.NoError(t, err)
		assert.Equal(t, "claude", p.BinaryName())
		assert.Equal(t, []string{"-p", "hello"}, p.BaseArgs("hello"))
	})

	t.Run("custom_binary_default_prompt_args", func(t *testing.T) {
		p, err := resolveProvider(map[string]any{"binary": "gemini"})
		require.NoError(t, err)
		assert.Equal(t, "gemini", p.BinaryName())
		assert.Equal(t, []string{"-p", "hello"}, p.BaseArgs("hello"))
	})

	t.Run("custom_binary_with_prompt_args", func(t *testing.T) {
		p, err := resolveProvider(map[string]any{
			"binary":      "aider",
			"prompt_args": []any{"-m"},
		})
		require.NoError(t, err)
		assert.Equal(t, "aider", p.BinaryName())
		assert.Equal(t, []string{"-m", "hello"}, p.BaseArgs("hello"))
	})

	t.Run("templated_provider_runtime_error", func(t *testing.T) {
		_, err := resolveProvider(map[string]any{"provider": "${PROVIDER}"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unresolved provider template")
	})
}

func TestBuildProviderConfigs(t *testing.T) {
	t.Run("primary_and_fallback", func(t *testing.T) {
		if goruntime.GOOS == "windows" {
			t.Skip("Skipping shell-based test on Windows")
		}

		primary := writeHarnessTestBinary(t, "primary", "#!/bin/sh\nexit 0\n")
		fallback := writeHarnessTestBinary(t, "fallback", "#!/bin/sh\nexit 0\n")

		configs, err := buildProviderConfigs(map[string]any{
			"binary": primary,
			"fallback": []any{
				map[string]any{"binary": fallback},
			},
		})
		require.NoError(t, err)
		require.Len(t, configs, 2)
		assert.Equal(t, primary, configs[0].provider.BinaryName())
		assert.Equal(t, fallback, configs[1].provider.BinaryName())
	})
}

func TestHarnessExecutorRun_FallbackBuffersStdout(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Skipping shell-based test on Windows")
	}

	primary := writeHarnessTestBinary(t, "primary", `#!/bin/sh
echo "primary stdout"
echo "primary stderr" >&2
exit 1
`)
	fallback := writeHarnessTestBinary(t, "fallback", `#!/bin/sh
echo "fallback stdout"
echo "fallback stderr" >&2
exit 0
`)

	exec := &harnessExecutor{
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
		configs: []providerConfig{
			{provider: &customProvider{binary: primary, promptArgs: []string{}}, flags: map[string]any{}},
			{provider: &customProvider{binary: fallback, promptArgs: []string{}}, flags: map[string]any{}},
		},
		prompt: "hello",
	}

	stdout := exec.stdout.(*strings.Builder)
	stderr := exec.stderr.(*strings.Builder)
	err := exec.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "fallback stdout\n", stdout.String())
	assert.Contains(t, stderr.String(), "primary stderr")
	assert.Contains(t, stderr.String(), "fallback stderr")
	assert.Contains(t, stderr.String(), "trying fallback")
	assert.Equal(t, 0, exec.ExitCode())
}

func TestHarnessExecutorRun_ContextCancellationSkipsFallback(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Skipping shell-based test on Windows")
	}

	marker := filepath.Join(t.TempDir(), "fallback-ran")
	primary := writeHarnessTestBinary(t, "primary", `#!/bin/sh
sleep 1
echo "primary stderr" >&2
exit 1
`)
	fallback := writeHarnessTestBinary(t, "fallback", "#!/bin/sh\ntouch \""+marker+"\"\nexit 0\n")

	var stdout strings.Builder
	var stderr strings.Builder
	exec := &harnessExecutor{
		stdout: &stdout,
		stderr: &stderr,
		configs: []providerConfig{
			{provider: &customProvider{binary: primary, promptArgs: []string{}}, flags: map[string]any{}},
			{provider: &customProvider{binary: fallback, promptArgs: []string{}}, flags: map[string]any{}},
		},
		prompt: "hello",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := exec.Run(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NoFileExists(t, marker)
	assert.NotContains(t, stderr.String(), "trying fallback")
	assert.Equal(t, 124, exec.ExitCode())
}

func TestHarnessExecutorRun_CreatesWorkingDir(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Skipping shell-based test on Windows")
	}

	workDir := filepath.Join(t.TempDir(), "nested", "workdir")
	bin := writeHarnessTestBinary(t, "pwd", "#!/bin/sh\npwd\n")

	var stdout strings.Builder
	var stderr strings.Builder
	exec := &harnessExecutor{
		stdout: &stdout,
		stderr: &stderr,
		configs: []providerConfig{
			{provider: &customProvider{binary: bin, promptArgs: []string{}}, flags: map[string]any{}},
		},
		prompt:  "hello",
		workDir: workDir,
	}

	err := exec.Run(context.Background())
	require.NoError(t, err)
	assert.DirExists(t, workDir)
	assert.Contains(t, stdout.String(), workDir)
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
}

func TestExitCodeFromError(t *testing.T) {
	assert.Equal(t, 0, exitCodeFromError(nil))
	assert.Equal(t, 1, exitCodeFromError(assert.AnError))
}

func writeHarnessTestBinary(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

func mustFallback(t *testing.T, value any) []map[string]any {
	t.Helper()

	switch v := value.(type) {
	case []map[string]any:
		return v
	case []any:
		ret := make([]map[string]any, len(v))
		for i := range v {
			item, ok := v[i].(map[string]any)
			require.True(t, ok, "fallback[%d] should be a map[string]any", i)
			ret[i] = item
		}
		return ret
	default:
		t.Fatalf("unexpected fallback type %T", value)
		return nil
	}
}
