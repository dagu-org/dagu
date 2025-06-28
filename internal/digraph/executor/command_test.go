package executor

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellCommandBuilder_Build(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name             string
		builder          shellCommandBuilder
		expectedInArgs   []string
		shouldSkipOnMac  bool
	}{
		{
			name: "basic shell command",
			builder: shellCommandBuilder{
				ShellCommand:     "/bin/sh",
				ShellCommandArgs: "echo hello",
			},
			expectedInArgs: []string{"-c", "echo hello"},
		},
		{
			name: "bash command",
			builder: shellCommandBuilder{
				ShellCommand:     "/bin/bash",
				ShellCommandArgs: "echo hello",
			},
			expectedInArgs: []string{"-c", "echo hello"},
		},
		{
			name: "command with script",
			builder: shellCommandBuilder{
				Command: "python",
				Script:  "script.py",
				Args:    []string{"-u"},
				ShellCommand: "python", // Need to set this too
			},
			expectedInArgs: []string{"-u", "script.py"},
		},
		{
			name: "powershell command",
			builder: shellCommandBuilder{
				ShellCommand:     "powershell",
				ShellCommandArgs: "Write-Host hello",
			},
			expectedInArgs:  []string{"-Command", "Write-Host hello"},
			shouldSkipOnMac: true,
		},
		{
			name: "cmd.exe command",
			builder: shellCommandBuilder{
				ShellCommand:     "cmd.exe",
				ShellCommandArgs: "echo hello",
			},
			expectedInArgs:  []string{"/c", "echo hello"},
			shouldSkipOnMac: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldSkipOnMac && runtime.GOOS == "darwin" {
				t.Skip("Skipping Windows-specific test on macOS")
			}
			
			cmd, err := tt.builder.Build(ctx)
			require.NoError(t, err)
			require.NotNil(t, cmd)
			
			// Check that expected arguments are in the command
			for _, expectedArg := range tt.expectedInArgs {
				assert.Contains(t, cmd.Args, expectedArg)
			}
		})
	}
}

func TestBuildPowerShellCommand(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name     string
		cmd      string
		args     []string
		builder  shellCommandBuilder
		expected []string
	}{
		{
			name: "basic powershell command",
			cmd:  "powershell",
			args: []string{},
			builder: shellCommandBuilder{
				ShellCommandArgs: "Get-Date",
			},
			expected: []string{"powershell", "-Command", "Get-Date"},
		},
		{
			name: "powershell with existing -Command",
			cmd:  "powershell",
			args: []string{"-Command"},
			builder: shellCommandBuilder{
				ShellCommandArgs: "Get-Date",
			},
			expected: []string{"powershell", "-Command", "Get-Date"},
		},
		{
			name: "powershell with script",
			cmd:  "powershell",
			args: []string{},
			builder: shellCommandBuilder{
				Command: "python",
				Script:  "test.py",
				Args:    []string{"-u"},
			},
			expected: []string{"python", "-u", "test.py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.builder.buildPowerShellCommand(ctx, tt.cmd, tt.args)
			require.NoError(t, err)
			require.NotNil(t, cmd)
			
			assert.Equal(t, tt.expected, cmd.Args)
		})
	}
}

func TestBuildCmdCommand(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name     string
		cmd      string
		args     []string
		builder  shellCommandBuilder
		expected []string
	}{
		{
			name: "basic cmd command",
			cmd:  "cmd",
			args: []string{},
			builder: shellCommandBuilder{
				ShellCommandArgs: "dir",
			},
			expected: []string{"cmd", "/c", "dir"},
		},
		{
			name: "cmd with existing /c",
			cmd:  "cmd",
			args: []string{"/c"},
			builder: shellCommandBuilder{
				ShellCommandArgs: "dir",
			},
			expected: []string{"cmd", "/c", "dir"},
		},
		{
			name: "cmd with script",
			cmd:  "cmd",
			args: []string{},
			builder: shellCommandBuilder{
				Command: "python",
				Script:  "test.py",
				Args:    []string{"-u"},
			},
			expected: []string{"python", "-u", "test.py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.builder.buildCmdCommand(ctx, tt.cmd, tt.args)
			require.NoError(t, err)
			require.NotNil(t, cmd)
			
			assert.Equal(t, tt.expected, cmd.Args)
		})
	}
}

func TestCommandExecutor_Kill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}
	
	ctx := context.Background()
	env := NewEnv(ctx, digraph.Step{})
	env.WorkingDir = t.TempDir()
	ctx = WithEnv(ctx, env)
	
	// Create a command that will run for a while
	step := digraph.Step{
		Name:    "test",
		Command: "sleep 10",
		Shell:   "/bin/sh",
	}
	
	executor, err := newCommand(ctx, step)
	require.NoError(t, err)
	
	cmdExecutor := executor.(*commandExecutor)
	
	// Start the command in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- cmdExecutor.Run(ctx)
	}()
	
	// Give it a moment to start
	// Note: This is a bit flaky but necessary for the test
	require.Eventually(t, func() bool {
		return cmdExecutor.cmd != nil && cmdExecutor.cmd.Process != nil
	}, 1000, 10, "Command should start")
	
	// Kill the process
	err = cmdExecutor.Kill(os.Interrupt)
	assert.NoError(t, err)
	
	// Wait for it to finish
	select {
	case <-done:
		// Process was killed successfully
	case <-ctx.Done():
		t.Fatal("Context cancelled before process finished")
	}
}

func TestSetupCommand(t *testing.T) {
	// This test verifies that setupCommand is called and sets up the command correctly
	cmd := exec.Command("echo", "test")
	
	// Call setupCommand (this is platform-specific)
	setupCommand(cmd)
	
	// On Unix, it should set process group attributes
	if runtime.GOOS != "windows" {
		require.NotNil(t, cmd.SysProcAttr)
	}
}

func TestCommandConfig_NewCmd(t *testing.T) {
	ctx := context.Background()
	env := NewEnv(ctx, digraph.Step{})
	env.WorkingDir = t.TempDir()
	ctx = WithEnv(ctx, env)
	
	tests := []struct {
		name       string
		config     commandConfig
		scriptFile string
		checkPath  string
	}{
		{
			name: "simple command",
			config: commandConfig{
				Ctx:     ctx,
				Command: "echo",
				Args:    []string{"hello"},
			},
			checkPath: "echo",
		},
		{
			name: "shell command",
			config: commandConfig{
				Ctx:              ctx,
				ShellCommand:     "/bin/sh",
				ShellCommandArgs: "echo hello",
			},
			checkPath: "/bin/sh",
		},
		{
			name: "script file",
			config: commandConfig{
				Ctx:          ctx,
				ShellCommand: "/bin/sh",
			},
			scriptFile: "/tmp/test.sh",
			checkPath:  "/bin/sh",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.config.newCmd(ctx, tt.scriptFile)
			require.NoError(t, err)
			require.NotNil(t, cmd)
			
			// Check the command path
			assert.Contains(t, cmd.Path, tt.checkPath)
			
			// Check working directory
			if tt.config.Dir != "" {
				assert.Equal(t, tt.config.Dir, cmd.Dir)
			}
		})
	}
}

func TestCommandExecutor_ExitCode(t *testing.T) {
	ctx := context.Background()
	env := NewEnv(ctx, digraph.Step{})
	env.WorkingDir = t.TempDir()
	ctx = WithEnv(ctx, env)
	
	tests := []struct {
		name         string
		step         digraph.Step
		expectedCode int
		shouldError  bool
	}{
		{
			name: "successful command",
			step: digraph.Step{
				Name:    "test",
				Command: "true",
			},
			expectedCode: 0,
			shouldError:  false,
		},
		{
			name: "failing command",
			step: digraph.Step{
				Name:    "test",
				Command: "false",
			},
			expectedCode: 1,
			shouldError:  true,
		},
		{
			name: "exit with specific code",
			step: digraph.Step{
				Name:    "test", 
				Command: "exit 42",
				Shell:   "/bin/sh",
			},
			expectedCode: 42,
			shouldError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor, err := newCommand(ctx, tt.step)
			require.NoError(t, err)
			
			err = executor.Run(ctx)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			
			// Check exit code
			if exitCoder, ok := executor.(ExitCoder); ok {
				assert.Equal(t, tt.expectedCode, exitCoder.ExitCode())
			}
		})
	}
}