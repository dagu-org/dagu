package ssh

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSSHExecutor(t *testing.T) {
	t.Parallel()

	step := core.Step{
		Name: "ssh-exec",
		ExecutorConfig: core.ExecutorConfig{
			Type: "ssh",
			Config: map[string]any{
				"User":     "testuser",
				"IP":       "testip",
				"Port":     25,
				"Password": "testpassword",
			},
		},
	}
	ctx := context.Background()
	_, err := NewSSHExecutor(ctx, step)
	require.NoError(t, err)
}

func TestSSHCommandEscaping(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		expected string
	}{
		{
			name:     "Simple command",
			command:  "ls",
			args:     nil,
			expected: "ls",
		},
		{
			name:     "Command with space",
			command:  "echo",
			args:     []string{"hello world"},
			expected: "echo 'hello world'",
		},
		{
			name:     "Command with special characters",
			command:  "echo",
			args:     []string{"$HOME", "quote'quote"},
			expected: "echo '$HOME' 'quote'\\''quote'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := cmdutil.ShellQuote(tt.command)
			if len(tt.args) > 0 {
				actual += " " + cmdutil.ShellQuoteArgs(tt.args)
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestSSHExecutor_GetEffectiveShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		stepShell     string
		stepShellArgs []string
		configShell   string
		wantShell     string
		wantArgs      []string
	}{
		{
			name:          "StepShellTakesPriority",
			stepShell:     "/bin/zsh",
			stepShellArgs: []string{"-e"},
			configShell:   "/bin/bash",
			wantShell:     "/bin/zsh",
			wantArgs:      []string{"-e"},
		},
		{
			name:        "ConfigShellFallback",
			stepShell:   "",
			configShell: "/bin/bash",
			wantShell:   "/bin/bash",
			wantArgs:    nil,
		},
		{
			name:      "NoShellConfigured",
			wantShell: "",
			wantArgs:  nil,
		},
		{
			name:          "StepShellWithNoArgs",
			stepShell:     "/bin/sh",
			stepShellArgs: nil,
			configShell:   "",
			wantShell:     "/bin/sh",
			wantArgs:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &sshExecutor{
				step: core.Step{
					Shell:     tt.stepShell,
					ShellArgs: tt.stepShellArgs,
				},
				configShell: tt.configShell,
			}
			shell, args := e.getEffectiveShell()
			assert.Equal(t, tt.wantShell, shell)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestSSHExecutor_GetEffectiveShell_DAGLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		dagShell      string
		dagShellArgs  []string
		stepShell     string
		stepShellArgs []string
		configShell   string
		wantShell     string
		wantArgs      []string
	}{
		{
			name:      "DAGShellFallback",
			dagShell:  "/bin/bash",
			wantShell: "/bin/bash",
			wantArgs:  nil,
		},
		{
			name:         "DAGShellWithArgs",
			dagShell:     "/bin/zsh",
			dagShellArgs: []string{"-e", "-x"},
			wantShell:    "/bin/zsh",
			wantArgs:     []string{"-e", "-x"},
		},
		{
			name:        "StepShellOverridesDAGShell",
			dagShell:    "/bin/sh",
			stepShell:   "/bin/bash",
			wantShell:   "/bin/bash",
			wantArgs:    nil,
		},
		{
			name:        "ConfigShellOverridesDAGShell",
			dagShell:    "/bin/sh",
			configShell: "/bin/zsh",
			wantShell:   "/bin/zsh",
			wantArgs:    nil,
		},
		{
			name:          "StepShellTakesPriorityOverAll",
			dagShell:      "/bin/sh",
			configShell:   "/bin/zsh",
			stepShell:     "/bin/bash",
			stepShellArgs: []string{"-e"},
			wantShell:     "/bin/bash",
			wantArgs:      []string{"-e"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create DAG with shell configuration
			dag := &core.DAG{
				Name:      "test-dag",
				Shell:     tt.dagShell,
				ShellArgs: tt.dagShellArgs,
			}

			// Create context with DAG
			ctx := runtime.NewContextForTest(context.Background(), dag, "test-run-id", "")
			// Create Env and store it in context
			env := runtime.NewEnv(ctx, core.Step{})
			ctx = runtime.WithEnv(ctx, env)

			e := &sshExecutor{
				ctx: ctx,
				step: core.Step{
					Shell:     tt.stepShell,
					ShellArgs: tt.stepShellArgs,
				},
				configShell: tt.configShell,
			}
			shell, args := e.getEffectiveShell()
			assert.Equal(t, tt.wantShell, shell)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestSSHExecutor_BuildCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		configShell string
		stepShell   string
		stepArgs    []string
		command     string
		args        []string
		expected    string
	}{
		{
			name:     "NoShell_DirectExecution",
			command:  "ls",
			args:     []string{"-la"},
			expected: "ls -la", // Simple args don't need quoting
		},
		{
			name:     "NoShell_SimpleCommand",
			command:  "echo",
			args:     []string{"hello"},
			expected: "echo hello", // Simple args don't need quoting
		},
		{
			name:     "NoShell_ArgsWithSpaces",
			command:  "echo",
			args:     []string{"hello world"},
			expected: "echo 'hello world'", // Args with spaces need quoting
		},
		{
			name:        "ConfigShell_BashWrap",
			configShell: "/bin/bash",
			command:     "echo",
			args:        []string{"hello"},
			expected:    "/bin/bash -c 'echo hello'", // Full command quoted
		},
		{
			name:      "StepShell_ShWrap",
			stepShell: "/bin/sh",
			command:   "ls",
			args:      nil,
			expected:  "/bin/sh -c ls",
		},
		{
			name:        "StepShell_OverridesConfig",
			configShell: "/bin/sh",
			stepShell:   "/bin/bash",
			stepArgs:    []string{"-e"},
			command:     "echo",
			args:        []string{"test"},
			expected:    "/bin/bash -e -c 'echo test'",
		},
		{
			name:        "PowerShell_CommandFlag",
			configShell: "powershell",
			command:     "Write-Host",
			args:        []string{"hello"},
			expected:    "powershell -Command 'Write-Host hello'",
		},
		{
			name:        "CommandWithSpecialChars",
			configShell: "/bin/bash",
			command:     "echo",
			args:        []string{"$HOME", "it's"},
			expected:    "/bin/bash -c 'echo '\\''$HOME'\\'' '\\''it'\\''\\'\\'''\\''s'\\'''",
		},
		{
			name:        "CommandWithSpaces",
			configShell: "/bin/bash",
			command:     "echo",
			args:        []string{"hello world"},
			expected:    "/bin/bash -c 'echo '\\''hello world'\\'''",
		},
		{
			name:        "ShellExpansion_CommandSubstitution",
			configShell: "/bin/sh",
			command:     "echo $(pwd)",
			args:        nil,
			expected:    "/bin/sh -c 'echo $(pwd)'", // Shell should interpret $(pwd)
		},
		{
			name:        "ShellExpansion_VariableExpansion",
			configShell: "/bin/bash",
			command:     "echo $HOME",
			args:        nil,
			expected:    "/bin/bash -c 'echo $HOME'", // Shell should expand $HOME
		},
		{
			name:        "ShellExpansion_PipeCommand",
			configShell: "/bin/sh",
			command:     "ls | grep test",
			args:        nil,
			expected:    "/bin/sh -c 'ls | grep test'", // Pipe should work
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &sshExecutor{
				step: core.Step{
					Shell:     tt.stepShell,
					ShellArgs: tt.stepArgs,
				},
				configShell: tt.configShell,
			}
			result := e.buildCommand(core.CommandEntry{
				Command: tt.command,
				Args:    tt.args,
			})
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewSSHExecutor_WithShellConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		config         map[string]any
		stepShell      string
		expectedConfig string
	}{
		{
			name: "ShellFromConfig",
			config: map[string]any{
				"user":     "testuser",
				"ip":       "testip",
				"port":     22,
				"password": "testpassword",
				"shell":    "/bin/bash", // lowercase - matches YAML behavior
			},
			expectedConfig: "/bin/bash",
		},
		{
			name: "NoShellInConfig",
			config: map[string]any{
				"user":     "testuser",
				"ip":       "testip",
				"port":     22,
				"password": "testpassword",
			},
			expectedConfig: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := core.Step{
				Name: "ssh-exec",
				ExecutorConfig: core.ExecutorConfig{
					Type:   "ssh",
					Config: tt.config,
				},
				Shell: tt.stepShell,
			}
			ctx := context.Background()
			exec, err := NewSSHExecutor(ctx, step)
			require.NoError(t, err)

			sshExec, ok := exec.(*sshExecutor)
			require.True(t, ok)
			assert.Equal(t, tt.expectedConfig, sshExec.configShell)
		})
	}
}
