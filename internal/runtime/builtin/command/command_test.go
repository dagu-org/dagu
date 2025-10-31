package command

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/executor"
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
	dag := &core.DAG{
		Name: "test-dag",
		Env:  []string{},
	}
	// Setup the context with DAG environment
	ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "", nil, nil, nil)

	env := execution.NewEnv(ctx, core.Step{})
	env.WorkingDir = t.TempDir()
	ctx = execution.WithEnv(ctx, env)

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			scriptFile := tt.scriptFile
			if scriptFile != "" {
				tempDir := t.TempDir()
				scriptFile = filepath.Join(tempDir, "test.sh")
				require.NoError(t, os.WriteFile(scriptFile, []byte("#!/bin/sh\necho hello\n"), 0o755))
			}

			cmd, err := tt.config.newCmd(ctx, scriptFile)
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
	dag := &core.DAG{
		Name: "test-dag",
		Env:  []string{},
	}
	// Setup the context with DAG environment
	ctx = execution.SetupDAGContext(ctx, dag, nil, execution.DAGRunRef{}, "test-run", "", nil, nil, nil)

	env := execution.NewEnv(ctx, core.Step{})
	env.WorkingDir = t.TempDir()
	ctx = execution.WithEnv(ctx, env)

	tests := []struct {
		name         string
		step         core.Step
		expectedCode int
		shouldError  bool
	}{
		{
			name: "SuccessfulCommand",
			step: core.Step{
				Name:    "test",
				Command: "true",
			},
			expectedCode: 0,
			shouldError:  false,
		},
		{
			name: "FailingCommand",
			step: core.Step{
				Name:    "test",
				Command: "false",
			},
			expectedCode: 1,
			shouldError:  true,
		},
		{
			name: "ExitWithSpecificCode",
			step: core.Step{
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
			exec, err := NewCommand(ctx, tt.step)
			require.NoError(t, err)

			err = exec.Run(ctx)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check exit code
			if exitCoder, ok := exec.(executor.ExitCoder); ok {
				assert.Equal(t, tt.expectedCode, exitCoder.ExitCode())
			}
		})
	}
}

func TestReadFirstLine(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"ShebangLine", "#!/bin/bash\necho hello", "#!/bin/bash"},
		{"SingleLine", "single line", "single line"},
		{"EmptyFile", "", ""},
		{"MultipleLines", "first\nsecond\nthird", "first"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "test-*.sh")
			require.NoError(t, err)
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			_, err = tmpFile.WriteString(tt.content)
			require.NoError(t, err)
			_ = tmpFile.Close()

			result, err := readFirstLine(tmpFile.Name())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := readFirstLine("/non/existent/file")
		assert.Error(t, err)
	})
}
