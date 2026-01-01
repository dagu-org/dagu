package ssh

import (
	"context"
	"testing"

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
		name      string
		shell     string
		shellArgs []string
		command   string
		args      []string
		expected  string
	}{
		{
			name:     "NoShell_NoArgs",
			shell:    "",
			command:  "ls",
			args:     nil,
			expected: "ls",
		},
		{
			name:     "NoShell_WithArgs",
			shell:    "",
			command:  "ls",
			args:     []string{"-la"},
			expected: "ls -la",
		},
		{
			name:     "NoShell_ArgsWithSpaces",
			shell:    "",
			command:  "echo",
			args:     []string{"hello world"},
			expected: "echo 'hello world'",
		},
		{
			name:     "WithShell",
			shell:    "/bin/bash",
			command:  "echo",
			args:     []string{"hello"},
			expected: "/bin/bash -c 'echo hello'",
		},
		{
			name:      "WithShellAndArgs",
			shell:     "/bin/bash",
			shellArgs: []string{"-e"},
			command:   "echo",
			args:      []string{"hello"},
			expected:  "/bin/bash -e -c 'echo hello'",
		},
		{
			name:     "WithShell_PowerShell",
			shell:    "powershell",
			command:  "Write-Host",
			args:     []string{"hello"},
			expected: "powershell -Command 'Write-Host hello'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &sshExecutor{
				shell:     tt.shell,
				shellArgs: tt.shellArgs,
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
		name          string
		config        map[string]any
		expectedShell string
		expectedArgs  []string
	}{
		{
			name: "ShellFromConfig",
			config: map[string]any{
				"user":     "testuser",
				"ip":       "testip",
				"port":     22,
				"password": "testpassword",
				"shell":    "/bin/bash",
			},
			expectedShell: "/bin/bash",
		},
		{
			name: "ShellFromConfigWithArgs",
			config: map[string]any{
				"user":     "testuser",
				"ip":       "testip",
				"port":     22,
				"password": "testpassword",
				"shell":    "/bin/bash -e",
			},
			expectedShell: "/bin/bash",
			expectedArgs:  []string{"-e"},
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
			assert.Equal(t, tt.expectedArgs, sshExec.shellArgs)
		})
	}
}

func TestSSHExecutor_DAGLevelShell(t *testing.T) {
	t.Parallel()

	// This test verifies that DAG-level SSH shell is passed through the Client
	// when using DAG-level SSH configuration

	// Create a mock client with shell set
	mockClient := &Client{
		hostPort:  "localhost:22",
		Shell:     "/bin/bash",
		ShellArgs: []string{"-e"},
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
	assert.Equal(t, []string{"-e"}, sshExec.shellArgs)
}

func TestSSHExecutor_StepLevelShellOverridesDAGLevel(t *testing.T) {
	t.Parallel()

	// Create a mock client with DAG-level shell
	mockClient := &Client{
		hostPort:  "localhost:22",
		Shell:     "/bin/sh", // DAG-level shell
		ShellArgs: []string{"-e"},
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
				"shell":    "/bin/zsh -o pipefail", // Step-level shell overrides DAG-level
			},
		},
	}

	exec, err := NewSSHExecutor(ctx, step)
	require.NoError(t, err)

	sshExec, ok := exec.(*sshExecutor)
	require.True(t, ok)
	// Step-level SSH config should take priority
	assert.Equal(t, "/bin/zsh", sshExec.shell)
	assert.Equal(t, []string{"-o", "pipefail"}, sshExec.shellArgs)
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
		Name:      "ssh-step",
		Shell:     "/bin/bash", // Step-level shell as fallback
		ShellArgs: []string{"-e"},
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
	assert.Equal(t, []string{"-e"}, sshExec.shellArgs)
}

func TestSSHExecutor_SSHConfigShellTakesPriorityOverStepShell(t *testing.T) {
	t.Parallel()

	// Create a mock client with shell set
	mockClient := &Client{
		hostPort:  "localhost:22",
		Shell:     "/bin/zsh", // SSH config shell
		ShellArgs: []string{"-e"},
	}

	// Create context with the mock client
	ctx := WithSSHClient(context.Background(), mockClient)

	// Create executor with both step.Shell and SSH config shell
	step := core.Step{
		Name:      "ssh-step",
		Shell:     "/bin/bash", // Step-level shell (should be ignored)
		ShellArgs: []string{"-o", "pipefail"},
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
	assert.Equal(t, []string{"-e"}, sshExec.shellArgs)
}

func TestSSHExecutor_StepSSHConfigWithoutShellIgnoresDAGShell(t *testing.T) {
	t.Parallel()

	// DAG-level SSH config has a shell
	mockClient := &Client{
		hostPort: "localhost:22",
		Shell:    "/bin/zsh", // DAG-level shell
	}

	ctx := WithSSHClient(context.Background(), mockClient)

	// Step has its own SSH config WITHOUT shell
	// This should override DAG-level config entirely (including shell)
	step := core.Step{
		Name: "ssh-step",
		ExecutorConfig: core.ExecutorConfig{
			Type: "ssh",
			Config: map[string]any{
				"user":     "stepuser",
				"ip":       "step-host",
				"port":     22,
				"password": "steppassword",
				// No shell specified - should NOT inherit from DAG-level
			},
		},
	}

	exec, err := NewSSHExecutor(ctx, step)
	require.NoError(t, err)

	sshExec, ok := exec.(*sshExecutor)
	require.True(t, ok)
	// Step-level SSH config takes priority, and it has no shell
	// DAG-level shell should NOT be inherited
	assert.Equal(t, "", sshExec.shell)
}

func TestSSHExecutor_GetEvalOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		step            core.Step
		dagShell        string
		expectSkipShell bool
	}{
		{
			name: "StepShellSet",
			step: core.Step{
				Shell:          "/bin/bash",
				ExecutorConfig: core.ExecutorConfig{Type: "ssh"},
			},
			expectSkipShell: false,
		},
		{
			name: "StepConfigShellSet",
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Type:   "ssh",
					Config: map[string]any{"shell": "/bin/bash"},
				},
			},
			expectSkipShell: false,
		},
		{
			name: "StepConfigNoShell",
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{
					Type:   "ssh",
					Config: map[string]any{"user": "test"},
				},
			},
			expectSkipShell: true,
		},
		{
			name: "DAGShellSet",
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{Type: "ssh"},
			},
			dagShell:        "/bin/bash",
			expectSkipShell: false,
		},
		{
			name: "NoShellAnywhere",
			step: core.Step{
				ExecutorConfig: core.ExecutorConfig{Type: "ssh"},
			},
			expectSkipShell: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.dagShell != "" {
				ctx = WithSSHClient(ctx, &Client{Shell: tt.dagShell})
			}

			opts := tt.step.EvalOptions(ctx)

			if tt.expectSkipShell {
				require.Len(t, opts, 1, "expected WithoutExpandShell option")
			} else {
				require.Empty(t, opts, "expected no eval options")
			}
		})
	}
}
