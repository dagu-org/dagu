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
		name            string
		builder         shellCommandBuilder
		expectedInArgs  []string
		shouldSkipOnMac bool
	}{
		{
			name: "BasicShellCommand",
			builder: shellCommandBuilder{
				ShellCommand:     "/bin/sh",
				ShellCommandArgs: "echo hello",
			},
			expectedInArgs: []string{"-c", "echo hello"},
		},
		{
			name: "BashCommand",
			builder: shellCommandBuilder{
				ShellCommand:     "/bin/bash",
				ShellCommandArgs: "echo hello",
			},
			expectedInArgs: []string{"-c", "echo hello"},
		},
		{
			name: "CommandWithScript",
			builder: shellCommandBuilder{
				Command:      "python",
				Script:       "script.py",
				Args:         []string{"-u"},
				ShellCommand: "python", // Need to set this too
			},
			expectedInArgs: []string{"-u", "script.py"},
		},
		{
			name: "PowershellCommand",
			builder: shellCommandBuilder{
				ShellCommand:     "powershell",
				ShellCommandArgs: "Write-Host hello",
			},
			expectedInArgs:  []string{"-Command", "Write-Host hello"},
			shouldSkipOnMac: true,
		},
		{
			name: "CmdExeCommand",
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
			name: "BasicPowershellCommand",
			cmd:  "powershell",
			args: []string{},
			builder: shellCommandBuilder{
				ShellCommandArgs: "Get-Date",
			},
			expected: []string{"powershell", "-Command", "Get-Date"},
		},
		{
			name: "PowershellWithExistingCommand",
			cmd:  "powershell",
			args: []string{"-Command"},
			builder: shellCommandBuilder{
				ShellCommandArgs: "Get-Date",
			},
			expected: []string{"powershell", "-Command", "Get-Date"},
		},
		{
			name: "PowershellWithScript",
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
			name: "BasicCmdCommand",
			cmd:  "cmd",
			args: []string{},
			builder: shellCommandBuilder{
				ShellCommandArgs: "dir",
			},
			expected: []string{"cmd", "/c", "dir"},
		},
		{
			name: "CmdWithExisting/C",
			cmd:  "cmd",
			args: []string{"/c"},
			builder: shellCommandBuilder{
				ShellCommandArgs: "dir",
			},
			expected: []string{"cmd", "/c", "dir"},
		},
		{
			name: "CmdWithScript",
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

	// Test that Kill returns nil for nil command
	executor := &commandExecutor{}
	err := executor.Kill(os.Interrupt)
	assert.NoError(t, err)

	// Test that Kill returns nil for command without process
	executor = &commandExecutor{cmd: &exec.Cmd{}}
	err = executor.Kill(os.Interrupt)
	assert.NoError(t, err)
}

func TestCommandConfig_NewCmd(t *testing.T) {
	ctx := context.Background()
	// Create a minimal DAG for the test
	dag := &digraph.DAG{
		Name: "test-dag",
		Env:  []string{},
	}
	// Setup the context with DAG environment
	ctx = digraph.SetupDAGContext(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "", nil, nil)

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
			name: "SimpleCommand",
			config: commandConfig{
				Ctx:     ctx,
				Command: "echo",
				Args:    []string{"hello"},
			},
			checkPath: "echo",
		},
		{
			name: "ShellCommand",
			config: commandConfig{
				Ctx:              ctx,
				ShellCommand:     "/bin/sh",
				ShellCommandArgs: "echo hello",
			},
			checkPath: "/bin/sh",
		},
		{
			name: "ScriptFile",
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
	// Create a minimal DAG for the test
	dag := &digraph.DAG{
		Name: "test-dag",
		Env:  []string{},
	}
	// Setup the context with DAG environment
	ctx = digraph.SetupDAGContext(ctx, dag, nil, digraph.DAGRunRef{}, "test-run", "", nil, nil)

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
			name: "SuccessfulCommand",
			step: digraph.Step{
				Name:    "test",
				Command: "true",
			},
			expectedCode: 0,
			shouldError:  false,
		},
		{
			name: "FailingCommand",
			step: digraph.Step{
				Name:    "test",
				Command: "false",
			},
			expectedCode: 1,
			shouldError:  true,
		},
		{
			name: "ExitWithSpecificCode",
			step: digraph.Step{
				Name:    "test",
				Command: "/bin/sh",
				Args:    []string{"-c", "exit 42"},
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
