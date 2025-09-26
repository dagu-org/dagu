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
		name         string
		shell        string
		cmdStr       string
		expectedArgs []string
	}{
		{
			name:         "BashShell",
			shell:        "/bin/bash",
			cmdStr:       "echo hello",
			expectedArgs: []string{"-c", "echo hello"},
		},
		{
			name:         "ShShell",
			shell:        "/bin/sh",
			cmdStr:       "echo hello",
			expectedArgs: []string{"-c", "echo hello"},
		},
		{
			name:         "ZshShell",
			shell:        "/bin/zsh",
			cmdStr:       "echo hello",
			expectedArgs: []string{"-c", "echo hello"},
		},
		{
			name:         "EmptyShellFallback",
			shell:        "",
			cmdStr:       "echo hello",
			expectedArgs: []string{"-c", "echo hello"},
		},
		{
			name:         "PowershellDetectionEvenOnUnix",
			shell:        "/usr/local/bin/powershell",
			cmdStr:       "echo hello",
			expectedArgs: []string{"-Command", "echo hello"},
		},
		{
			name:         "PwshDetection",
			shell:        "/usr/local/bin/pwsh",
			cmdStr:       "echo hello",
			expectedArgs: []string{"-Command", "echo hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildShellCommand(tt.shell, tt.cmdStr)
			require.NotNil(t, cmd)

			// For shells with explicit paths, just verify the command uses that shell
			if tt.shell != "" {
				assert.Equal(t, tt.shell, cmd.Path)
			} else {
				// For empty shell, just verify we got something
				assert.NotEmpty(t, cmd.Path)
			}

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
			name:   "CommandWithPipes",
			shell:  "/bin/bash",
			cmdStr: "echo hello | tr a-z A-Z",
		},
		{
			name:   "CommandWithQuotes",
			shell:  "/bin/bash",
			cmdStr: `echo "hello world"`,
		},
		{
			name:   "CommandWithVariables",
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
