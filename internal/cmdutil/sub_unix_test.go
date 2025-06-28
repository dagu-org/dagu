//go:build !windows
// +build !windows

package cmdutil

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildShellCommand(t *testing.T) {
	tests := []struct {
		name        string
		shell       string
		cmdStr      string
		expectedCmd string
		expectedArgs []string
	}{
		{
			name:        "bash shell",
			shell:       "/bin/bash",
			cmdStr:      "echo hello",
			expectedCmd: "/bin/bash",
			expectedArgs: []string{"-c", "echo hello"},
		},
		{
			name:        "sh shell",
			shell:       "/bin/sh",
			cmdStr:      "echo hello",
			expectedCmd: "/bin/sh",
			expectedArgs: []string{"-c", "echo hello"},
		},
		{
			name:        "zsh shell",
			shell:       "/bin/zsh",
			cmdStr:      "echo hello",
			expectedCmd: "/bin/zsh",
			expectedArgs: []string{"-c", "echo hello"},
		},
		{
			name:        "empty shell fallback",
			shell:       "",
			cmdStr:      "echo hello",
			expectedCmd: "/bin/sh",
			expectedArgs: []string{"-c", "echo hello"},
		},
		{
			name:        "powershell detection (even on unix)",
			shell:       "/usr/local/bin/powershell",
			cmdStr:      "echo hello",
			expectedCmd: "/usr/local/bin/powershell",
			expectedArgs: []string{"-Command", "echo hello"},
		},
		{
			name:        "pwsh detection",
			shell:       "/usr/local/bin/pwsh",
			cmdStr:      "echo hello",
			expectedCmd: "/usr/local/bin/pwsh",
			expectedArgs: []string{"-Command", "echo hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildShellCommand(tt.shell, tt.cmdStr)
			require.NotNil(t, cmd)
			
			assert.Equal(t, tt.expectedCmd, cmd.Path)
			assert.Equal(t, len(tt.expectedArgs), len(cmd.Args)-1) // -1 because Args[0] is the command itself
			
			// Check args (skip first arg which is the command path)
			for i, expectedArg := range tt.expectedArgs {
				assert.Equal(t, expectedArg, cmd.Args[i+1])
			}
		})
	}
}

func TestRunCommand_WithBuildShellCommand(t *testing.T) {
	// This test verifies that runCommand works correctly with the new buildShellCommand
	output, err := runCommand("echo test123")
	require.NoError(t, err)
	assert.Equal(t, "test123", output)
}

func TestBuildShellCommand_ComplexCommands(t *testing.T) {
	tests := []struct {
		name   string
		shell  string
		cmdStr string
	}{
		{
			name:   "command with pipes",
			shell:  "/bin/bash",
			cmdStr: "echo hello | tr a-z A-Z",
		},
		{
			name:   "command with quotes",
			shell:  "/bin/bash",
			cmdStr: `echo "hello world"`,
		},
		{
			name:   "command with variables",
			shell:  "/bin/bash",
			cmdStr: "VAR=test; echo $VAR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildShellCommand(tt.shell, tt.cmdStr)
			require.NotNil(t, cmd)
			
			// Verify it's a valid command that can be created
			_, err := exec.LookPath(cmd.Path)
			// Don't fail if the shell isn't found, just verify the command was built
			if err == nil {
				assert.Contains(t, cmd.Args, "-c")
				assert.Contains(t, cmd.Args, tt.cmdStr)
			}
		})
	}
}