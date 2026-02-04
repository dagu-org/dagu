package eval

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteCommands(t *testing.T) {
	// Skip tests on Windows as they require different commands
	if runtime.GOOS == "windows" {
		t.Skip("Skipping tests on Windows")
	}

	tests := []struct {
		name     string
		input    string
		want     string
		wantErr  bool
		setupEnv map[string]string
		skipOnOS []string
	}{
		{
			name:    "NoCommandSubstitutionNeeded",
			input:   "hello world",
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "SimpleEchoCommand",
			input:   "prefix `echo hello` suffix",
			want:    "prefix hello suffix",
			wantErr: false,
		},
		{
			name:    "MultipleCommands",
			input:   "`echo foo` and `echo bar`",
			want:    "foo and bar",
			wantErr: false,
		},
		{
			name:    "NestedQuotes",
			input:   "`echo \"hello world\"`",
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "CommandWithEnvironmentVariable",
			input:   "`echo $TEST_VAR`",
			want:    "test_value",
			wantErr: false,
			setupEnv: map[string]string{
				"TEST_VAR": "test_value",
			},
		},
		{
			name:    "CommandWithSpaces",
			input:   "`echo 'hello   world'`",
			want:    "hello   world",
			wantErr: false,
		},
		{
			name:    "InvalidCommand",
			input:   "`nonexistentcommand123`",
			wantErr: true,
		},
		{
			name:    "EmptyBackticks",
			input:   "``",
			want:    "``",
			wantErr: false,
		},
		{
			name:    "CommandThatReturnsError",
			input:   "`exit 1`",
			wantErr: true,
		},
		{
			name:    "CommandWithPipeline",
			input:   "`echo hello | tr 'a-z' 'A-Z'`",
			want:    "HELLO",
			wantErr: false,
		},
		{
			name:    "MultipleLinesInOutput",
			input:   "`printf 'line1\\nline2'`",
			want:    "line1\nline2",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip if test should be skipped on current OS
			for _, os := range tt.skipOnOS {
				if runtime.GOOS == os {
					t.Skipf("Skipping test on %s", os)
					return
				}
			}

			// Setup environment if needed
			for k, v := range tt.setupEnv {
				t.Setenv(k, v)
			}

			// Run test
			got, err := substituteCommandsWithContext(context.Background(), tt.input)

			// Check error
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Compare output
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSubstituteCommandsEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "EmptyInput",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "OnlySpaces",
			input:   "     ",
			want:    "     ",
			wantErr: false,
		},
		{
			name:    "UnmatchedBackticks",
			input:   "hello `world",
			want:    "hello `world",
			wantErr: false,
		},
		{
			name:    "EscapedBackticks",
			input:   "hello \\`world\\`",
			want:    "hello \\`world\\`",
			wantErr: false,
		},
		{
			name:    "MultipleBackticksWithoutCommand",
			input:   "``````",
			want:    "``````",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := substituteCommandsWithContext(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSubstituteCommands_Extended(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "SimpleCommandSubstitution",
			input:   "`echo hello`",
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "CommandInMiddleOfString",
			input:   "prefix `echo test` suffix",
			want:    "prefix test suffix",
			wantErr: false,
		},
		{
			name:    "MultipleCommands",
			input:   "`echo one` and `echo two`",
			want:    "one and two",
			wantErr: false,
		},
		{
			name:    "NestedBackticksNotSupported",
			input:   "`echo `echo nested``",
			want:    "echo nested``",
			wantErr: false,
		},
		{
			name:    "CommandWithArgs",
			input:   "`echo hello world`",
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "EmptyCommand",
			input:   "``",
			want:    "``",
			wantErr: false,
		},
		{
			name:    "CommandFailure",
			input:   "`false`",
			want:    "",
			wantErr: true,
		},
		{
			name:    "InvalidCommand",
			input:   "`command_that_does_not_exist`",
			want:    "",
			wantErr: true,
		},
		{
			name:    "NoCommandSubstitution",
			input:   "plain text without backticks",
			want:    "plain text without backticks",
			wantErr: false,
		},
		{
			name:    "EscapedBackticks",
			input:   "text with \\`escaped\\` backticks",
			want:    "text with \\`escaped\\` backticks",
			wantErr: false,
		},
		{
			name:    "CommandWithNewlineOutput",
			input:   "`printf 'line1\nline2'`",
			want:    "line1\nline2",
			wantErr: false,
		},
		{
			name:    "CommandWithTrailingNewlineRemoved",
			input:   "`echo -n hello`",
			want:    "hello",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := substituteCommandsWithContext(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- buildShellCommand coverage ---

func TestBuildShellCommand_PowerShell(t *testing.T) {
	cmd := buildShellCommand("powershell", "Get-Date")
	assert.Equal(t, "powershell", cmd.Path)
	assert.Contains(t, cmd.Args, "-Command")
}

func TestBuildShellCommand_Cmd(t *testing.T) {
	cmd := buildShellCommand("cmd.exe", "dir")
	assert.Contains(t, cmd.Args, "/c")
}

func TestBuildShellCommand_EmptyShell(t *testing.T) {
	cmd := buildShellCommand("", "echo hi")
	assert.Contains(t, cmd.Args, "-c")
}

// --- runCommandWithContext coverage ---

func TestRunCommandWithContext_WithScope(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("CMD_TEST_VAR", "from_scope", EnvSourceDAGEnv)
	// Need PATH to find echo
	scope = scope.WithEntry("PATH", os.Getenv("PATH"), EnvSourceOS)
	ctx := WithEnvScope(context.Background(), scope)

	result, err := runCommandWithContext(ctx, "echo hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}
