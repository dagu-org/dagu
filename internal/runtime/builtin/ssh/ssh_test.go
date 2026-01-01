package ssh

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
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

func TestSSHExecutor_BuildCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		shell    string
		command  string
		args     []string
		expected string
	}{
		{
			name:     "NoShell_DirectExecution",
			shell:    "",
			command:  "ls",
			args:     []string{"-la"},
			expected: "ls -la", // Simple args don't need quoting
		},
		{
			name:     "NoShell_SimpleCommand",
			shell:    "",
			command:  "echo",
			args:     []string{"hello"},
			expected: "echo hello", // Simple args don't need quoting
		},
		{
			name:     "NoShell_ArgsWithSpaces",
			shell:    "",
			command:  "echo",
			args:     []string{"hello world"},
			expected: "echo 'hello world'", // Args with spaces need quoting
		},
		{
			name:     "BashShell_Wrap",
			shell:    "/bin/bash",
			command:  "echo",
			args:     []string{"hello"},
			expected: "/bin/bash -c 'echo hello'", // Full command quoted
		},
		{
			name:     "ShShell_Wrap",
			shell:    "/bin/sh",
			command:  "ls",
			args:     nil,
			expected: "/bin/sh -c ls",
		},
		{
			name:     "PowerShell_CommandFlag",
			shell:    "powershell",
			command:  "Write-Host",
			args:     []string{"hello"},
			expected: "powershell -Command 'Write-Host hello'",
		},
		{
			name:     "CommandWithSpecialChars",
			shell:    "/bin/bash",
			command:  "echo",
			args:     []string{"$HOME", "it's"},
			expected: "/bin/bash -c 'echo '\\''$HOME'\\'' '\\''it'\\''\\'\\'''\\''s'\\'''",
		},
		{
			name:     "CommandWithSpaces",
			shell:    "/bin/bash",
			command:  "echo",
			args:     []string{"hello world"},
			expected: "/bin/bash -c 'echo '\\''hello world'\\'''",
		},
		{
			name:     "ShellExpansion_CommandSubstitution",
			shell:    "/bin/sh",
			command:  "echo $(pwd)",
			args:     nil,
			expected: "/bin/sh -c 'echo $(pwd)'", // Shell should interpret $(pwd)
		},
		{
			name:     "ShellExpansion_VariableExpansion",
			shell:    "/bin/bash",
			command:  "echo $HOME",
			args:     nil,
			expected: "/bin/bash -c 'echo $HOME'", // Shell should expand $HOME
		},
		{
			name:     "ShellExpansion_PipeCommand",
			shell:    "/bin/sh",
			command:  "ls | grep test",
			args:     nil,
			expected: "/bin/sh -c 'ls | grep test'", // Pipe should work
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &sshExecutor{
				shell: tt.shell,
			}
			result := e.buildCommand(core.CommandEntry{
				Command:     tt.command,
				Args:        tt.args,
				CmdWithArgs: buildCmdWithArgs(tt.command, tt.args),
			})
			assert.Equal(t, tt.expected, result)
		})
	}
}

// buildCmdWithArgs constructs the CmdWithArgs field for testing
func buildCmdWithArgs(cmd string, args []string) string {
	if len(args) == 0 {
		return cmd
	}
	return cmd + " " + cmdutil.ShellQuoteArgs(args)
}

func TestNewSSHExecutor_WithShellConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		config        map[string]any
		expectedShell string
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
			expectedShell: "/bin/bash",
		},
		{
			name: "NoShellInConfig",
			config: map[string]any{
				"user":     "testuser",
				"ip":       "testip",
				"port":     22,
				"password": "testpassword",
			},
			expectedShell: "",
		},
		{
			name: "ZshShell",
			config: map[string]any{
				"user":     "testuser",
				"ip":       "testip",
				"port":     22,
				"password": "testpassword",
				"shell":    "/usr/bin/zsh",
			},
			expectedShell: "/usr/bin/zsh",
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
			}
			ctx := context.Background()
			exec, err := NewSSHExecutor(ctx, step)
			require.NoError(t, err)

			sshExec, ok := exec.(*sshExecutor)
			require.True(t, ok)
			assert.Equal(t, tt.expectedShell, sshExec.shell)
		})
	}
}

func TestSSHExecutor_DAGLevelShell(t *testing.T) {
	t.Parallel()

	// This test verifies that DAG-level SSH shell is passed through the Client
	// when using DAG-level SSH configuration

	// Create a mock client with shell set
	mockClient := &Client{
		hostPort: "localhost:22",
		Shell:    "/bin/bash",
	}

	// Create context with the mock client
	ctx := WithSSHClient(context.Background(), mockClient)

	// Create executor without step-level config (should use DAG-level)
	step := core.Step{
		Name: "ssh-step",
		ExecutorConfig: core.ExecutorConfig{
			Type:   "ssh",
			Config: nil, // No step-level config
		},
	}

	exec, err := NewSSHExecutor(ctx, step)
	require.NoError(t, err)

	sshExec, ok := exec.(*sshExecutor)
	require.True(t, ok)
	assert.Equal(t, "/bin/bash", sshExec.shell)
	assert.Equal(t, "/bin/bash", sshExec.getEffectiveShell())
}

func TestSSHExecutor_StepLevelShellOverridesDAGLevel(t *testing.T) {
	t.Parallel()

	// Create a mock client with DAG-level shell
	mockClient := &Client{
		hostPort: "localhost:22",
		Shell:    "/bin/sh", // DAG-level shell
	}

	// Create context with the mock client
	ctx := WithSSHClient(context.Background(), mockClient)

	// Create executor with step-level config that has different shell
	step := core.Step{
		Name: "ssh-step",
		ExecutorConfig: core.ExecutorConfig{
			Type: "ssh",
			Config: map[string]any{
				"user":     "testuser",
				"ip":       "testip",
				"port":     22,
				"password": "testpassword",
				"shell":    "/bin/zsh", // Step-level shell overrides DAG-level
			},
		},
	}

	exec, err := NewSSHExecutor(ctx, step)
	require.NoError(t, err)

	sshExec, ok := exec.(*sshExecutor)
	require.True(t, ok)
	// Step-level SSH config should take priority
	assert.Equal(t, "/bin/zsh", sshExec.shell)
}

func TestSSHExecutor_StepShellFallback(t *testing.T) {
	t.Parallel()

	// Create a mock client WITHOUT shell set
	mockClient := &Client{
		hostPort: "localhost:22",
		Shell:    "", // No SSH config shell
	}

	// Create context with the mock client
	ctx := WithSSHClient(context.Background(), mockClient)

	// Create executor with step.Shell set (fallback for UX)
	step := core.Step{
		Name:  "ssh-step",
		Shell: "/bin/bash", // Step-level shell as fallback
		ExecutorConfig: core.ExecutorConfig{
			Type:   "ssh",
			Config: nil, // No step-level SSH config
		},
	}

	exec, err := NewSSHExecutor(ctx, step)
	require.NoError(t, err)

	sshExec, ok := exec.(*sshExecutor)
	require.True(t, ok)
	// step.Shell should be used as fallback
	assert.Equal(t, "/bin/bash", sshExec.shell)
}

func TestSSHExecutor_SSHConfigShellTakesPriorityOverStepShell(t *testing.T) {
	t.Parallel()

	// Create a mock client with shell set
	mockClient := &Client{
		hostPort: "localhost:22",
		Shell:    "/bin/zsh", // SSH config shell
	}

	// Create context with the mock client
	ctx := WithSSHClient(context.Background(), mockClient)

	// Create executor with both step.Shell and SSH config shell
	step := core.Step{
		Name:  "ssh-step",
		Shell: "/bin/bash", // Step-level shell (should be ignored)
		ExecutorConfig: core.ExecutorConfig{
			Type:   "ssh",
			Config: nil, // No step-level SSH config, uses DAG-level
		},
	}

	exec, err := NewSSHExecutor(ctx, step)
	require.NoError(t, err)

	sshExec, ok := exec.(*sshExecutor)
	require.True(t, ok)
	// SSH config shell takes priority over step.Shell
	assert.Equal(t, "/bin/zsh", sshExec.shell)
}
