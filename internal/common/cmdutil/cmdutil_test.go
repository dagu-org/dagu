package cmdutil

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitCommandWithQuotes(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		cmd, args, err := SplitCommand("ls -al test/")
		require.NoError(t, err)
		require.Equal(t, "ls", cmd)
		require.Len(t, args, 2)
		require.Equal(t, "-al", args[0])
		require.Equal(t, "test/", args[1])
	})
	t.Run("WithJSON", func(t *testing.T) {
		cmd, args, err := SplitCommand(`echo {"key":"value"}`)
		require.NoError(t, err)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, `{"key":"value"}`, args[0])
	})
	t.Run("WithQuotedJSON", func(t *testing.T) {
		cmd, args, err := SplitCommand(`echo "{\"key\":\"value\"}"`)
		require.NoError(t, err)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, `{"key":"value"}`, args[0])
	})
	t.Run("ShellWithSingleQuotedCommand", func(t *testing.T) {
		cmd, args, err := SplitCommand(`sh -c 'echo this is stderr >&2'`)
		require.NoError(t, err)
		require.Equal(t, "sh", cmd)
		require.Len(t, args, 2)
		require.Equal(t, "-c", args[0])
		require.Equal(t, "echo this is stderr >&2", args[1])
	})
}

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCmd   string
		wantArgs  []string
		wantErr   bool
		errorType error
	}{
		{
			name:     "SimpleCommandNoArgs",
			input:    "echo",
			wantCmd:  "echo",
			wantArgs: []string{},
		},
		{
			name:     "CommandWithSingleArg",
			input:    "echo hello",
			wantCmd:  "echo",
			wantArgs: []string{"hello"},
		},
		{
			name:     "CommandWithBacktick",
			input:    "echo `echo hello`",
			wantCmd:  "echo",
			wantArgs: []string{"`echo hello`"},
		},
		{
			name:     "CommandWithMultipleArgs",
			input:    "echo hello world",
			wantCmd:  "echo",
			wantArgs: []string{"hello", "world"},
		},
		{
			name:     "CommandWithQuotedArgs",
			input:    `echo "hello world"`,
			wantCmd:  "echo",
			wantArgs: []string{"hello world"},
		},
		{
			name:     "CommandWithPipe",
			input:    "echo foo | grep foo",
			wantCmd:  "echo",
			wantArgs: []string{"foo", "|", "grep", "foo"},
		},
		{
			name:     "ComplexPipeCommand",
			input:    "echo foo | grep foo | wc -l",
			wantCmd:  "echo",
			wantArgs: []string{"foo", "|", "grep", "foo", "|", "wc", "-l"},
		},
		{
			name:     "CommandWithQuotedPipe",
			input:    `echo "hello|world"`,
			wantCmd:  "echo",
			wantArgs: []string{"hello|world"},
		},
		{
			name:      "EmptyCommand",
			input:     "",
			wantErr:   true,
			errorType: ErrCommandIsEmpty,
		},
		{
			name:     "CommandWithEscapedQuotes",
			input:    `echo "\"hello world\""`,
			wantCmd:  "echo",
			wantArgs: []string{`"hello world"`},
		},
		{
			name:     "CommandWithJSON",
			input:    `echo "{\n\t\"key\": \"value\"\n}"`,
			wantCmd:  "echo",
			wantArgs: []string{"{\n\t\"key\": \"value\"\n}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotArgs, err := SplitCommand(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("splitCommand() error = nil, want error")
					return
				}
				if tt.errorType != nil && err != tt.errorType {
					t.Errorf("splitCommand() error = %v, want %v", err, tt.errorType)
				}
				return
			}

			if err != nil {
				t.Errorf("splitCommand() error = %v, want nil", err)
				return
			}

			if gotCmd != tt.wantCmd {
				t.Errorf("splitCommand() gotCmd = %v, want %v", gotCmd, tt.wantCmd)
			}

			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("splitCommand() gotArgs length = %v, want %v", len(gotArgs), len(tt.wantArgs))
				return
			}

			for i := range gotArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Errorf("splitCommand() gotArgs[%d] = %v, want %v", i, gotArgs[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestSplitCommandWithSub(t *testing.T) {
	t.Run("CommandSubstitution", func(t *testing.T) {
		cmd, args, err := SplitCommandWithSub("echo `echo hello`")
		require.NoError(t, err)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "hello", args[0])
	})
	t.Run("QuotedCommandSubstitution", func(t *testing.T) {
		cmd, args, err := SplitCommandWithSub("echo `echo \"hello world\"`")
		require.NoError(t, err)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "hello world", args[0])
	})
	t.Run("EnvVar", func(t *testing.T) {
		_ = os.Setenv("TEST_ARG", "hello")
		cmd, args, err := SplitCommandWithSub("echo $TEST_ARG")
		require.NoError(t, err)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "$TEST_ARG", args[0]) // env var should not be expanded
	})
}

// TestBuildCommandString demonstrates table-driven tests for BuildCommandString.
func TestBuildEscapedCommandString(t *testing.T) {
	type testCase struct {
		name string
		cmd  string
		args []string
		want string
	}

	tests := []testCase{
		{
			name: "Piping",
			cmd:  "echo",
			args: []string{"hello", "|", "wc", "-c"},
			want: "echo hello | wc -c",
		},
		{
			name: "Redirection",
			cmd:  "echo",
			args: []string{"'test content'", ">", "testfile.txt", "&&", "cat", "testfile.txt"},
			want: `echo 'test content' > testfile.txt && cat testfile.txt`,
		},
		{
			name: "KeyValueArgument",
			cmd:  "echo",
			args: []string{`key="value"`},
			want: `echo key="value"`,
		},
		{
			name: "JSONArgument",
			cmd:  "echo",
			args: []string{`{"foo":"bar","hello":"world"}`},
			want: `echo {"foo":"bar","hello":"world"}`,
		},
		{
			name: "KeyValueArgument",
			cmd:  "echo",
			args: []string{`key="some value"`},
			want: `echo key="some value"`,
		},
		{
			name: "DoubleQuotes",
			cmd:  "echo",
			args: []string{`a "b" c`},
			want: `echo "a \"b\" c"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Build the final command line that will be passed to `sh -c`.
			cmdStr := BuildCommandEscapedString(tc.cmd, tc.args)

			// Check if the built command string is as expected.
			require.Equal(t, tc.want, cmdStr, "unexpected command string")
		})
	}
}

func TestParsePipedCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [][]string
		wantErr bool
	}{
		{
			name:  "SimpleCommandNoArgs",
			input: "echo",
			want:  [][]string{{"echo"}},
		},
		{
			name:  "SimpleCommandWithArgs",
			input: "echo foo bar",
			want:  [][]string{{"echo", "foo", "bar"}},
		},
		{
			name:  "CommandWithQuotedArgs",
			input: `echo "hello world"`,
			want:  [][]string{{"echo", `hello world`}},
		},
		{
			name:  "CommandWithPipe",
			input: "echo foo | grep foo",
			want:  [][]string{{"echo", "foo"}, {"grep", "foo"}},
		},
		{
			name:  "MultiplePipes",
			input: "echo foo | grep foo | wc -l",
			want:  [][]string{{"echo", "foo"}, {"grep", "foo"}, {"wc", "-l"}},
		},
		{
			name:  "PipeInQuotes",
			input: `echo "hello|world"`,
			want:  [][]string{{"echo", `hello|world`}},
		},
		{
			name:  "CommandWithSingleQuotedArgs",
			input: `sh -c 'echo this is stderr >&2'`,
			want:  [][]string{{"sh", "-c", `echo this is stderr >&2`}},
		},
		{
			name:  "PipeInSingleQuotes",
			input: `echo 'hello|world'`,
			want:  [][]string{{"echo", `hello|world`}},
		},
		{
			name:  "MixedSingleAndDoubleQuotes",
			input: `echo "hello" 'world'`,
			want:  [][]string{{"echo", `hello`, `world`}},
		},
		{
			name:  "SingleQuotesWithSpaces",
			input: `echo 'hello world'`,
			want:  [][]string{{"echo", `hello world`}},
		},
		{
			name:  "MultipleSpacesBetweenCommands",
			input: "echo foo    |    grep foo",
			want:  [][]string{{"echo", "foo"}, {"grep", "foo"}},
		},
		{
			name:  "CommandWithBackticks",
			input: "echo `date`",
			want:  [][]string{{"echo", "`date`"}},
		},
		{
			name:  "PipeInBackticks",
			input: "echo `echo foo | grep foo`",
			want:  [][]string{{"echo", "`echo foo | grep foo`"}},
		},
		{
			name:  "EscapedQuotes",
			input: `echo "Hello \"World\""`,
			want:  [][]string{{"echo", `Hello "World"`}},
		},
		{
			name:  "EscapedPipe",
			input: `echo foo\|bar`,
			want:  [][]string{{"echo", `foo\|bar`}},
		},
		{
			name:  "EmptyCommand",
			input: "",
			want:  [][]string{},
		},
		{
			name:  "MixedQuotesAndBackticks",
			input: "echo \"hello\" world `date`",
			want:  [][]string{{"echo", `hello`, "world", "`date`"}},
		},
		{
			name:  "ComplexPipeline",
			input: `find . -name "*.go" | xargs grep "fmt" | sort | uniq -c`,
			want: [][]string{
				{"find", ".", "-name", `*.go`},
				{"xargs", "grep", `fmt`},
				{"sort"},
				{"uniq", "-c"},
			},
		},
		{
			name:  "CommandWithEnvironmentVariables",
			input: `echo $HOME | grep home`,
			want:  [][]string{{"echo", "$HOME"}, {"grep", "home"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePipedCommand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePipedCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePipedCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParsePipedCommandErrors tests error cases for ParsePipedCommand
func TestParsePipedCommandErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPipe [][]string // what we expect even in case of errors
	}{
		{
			name:     "UnterminatedQuote",
			input:    `echo "hello`,
			wantPipe: [][]string{{"echo", `"hello`}},
		},
		{
			name:     "UnterminatedBacktick",
			input:    "echo `date",
			wantPipe: [][]string{{"echo", "`date"}},
		},
		{
			name:     "MixedUnterminatedQuotes",
			input:    "echo \"hello `date`\"",
			wantPipe: [][]string{{"echo", "hello `date`"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePipedCommand(tt.input)
			// Currently, ParsePipedCommand doesn't return errors for malformed input
			// But we still want to verify the output matches expected behavior
			if err != nil {
				t.Errorf("ParsePipedCommand() unexpected error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.wantPipe) {
				t.Errorf("ParsePipedCommand() = %v, want %v", got, tt.wantPipe)
			}
		})
	}
}

// TestParsePipedCommandShellOperators tests handling of shell operators like || and &&
func TestParsePipedCommandShellOperators(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [][]string
		wantErr bool
	}{
		{
			name:  "OROperatorFalseTrue",
			input: "false || true",
			want:  [][]string{{"false", "||", "true"}}, // Currently incorrect behavior
		},
		{
			name:  "ANDOperatorTrueEchoSuccess",
			input: "true && echo success",
			want:  [][]string{{"true", "&&", "echo", "success"}}, // Should be single command
		},
		{
			name:  "MixedOperatorsFalseTrueEchoDone",
			input: "false || true && echo done",
			want:  [][]string{{"false", "||", "true", "&&", "echo", "done"}}, // Should be single command
		},
		{
			name:  "ORWithSpacesFalseTrue",
			input: "false  ||  true",
			want:  [][]string{{"false", "||", "true"}}, // Should handle extra spaces
		},
		{
			name:  "SinglePipeVsDoublePipeEchoAGrepAEchoFailed",
			input: "echo a | grep a || echo failed",
			want:  [][]string{{"echo", "a"}, {"grep", "a", "||", "echo", "failed"}}, // First | is pipe, || is OR
		},
		{
			name:  "ComplexShellCommand",
			input: "test -f file.txt && cat file.txt || echo 'file not found'",
			want:  [][]string{{"test", "-f", "file.txt", "&&", "cat", "file.txt", "||", "echo", "file not found"}},
		},
		{
			name:  "ParenthesesGrouping",
			input: "(false || true) && echo success",
			want:  [][]string{{"(false", "||", "true)", "&&", "echo", "success"}},
		},
		{
			name:  "ORInQuotesShouldNotBeParsed",
			input: `echo "false || true"`,
			want:  [][]string{{"echo", "false || true"}}, // Quoted || should remain intact
		},
		{
			name:  "ANDInQuotesShouldNotBeParsed",
			input: `echo "true && false"`,
			want:  [][]string{{"echo", "true && false"}}, // Quoted && should remain intact
		},
		{
			name:  "MixedPipeAndOR",
			input: "ps aux | grep process || echo 'not found'",
			want:  [][]string{{"ps", "aux"}, {"grep", "process", "||", "echo", "not found"}},
		},
		{
			name:  "TriplePipeEdgeCase",
			input: "echo a ||| echo b",
			want:  [][]string{{"echo", "a", "||"}, {"echo", "b"}}, // ||| = | + ||
		},
		{
			name:  "SemicolonOperator",
			input: "echo first; echo second",
			want:  [][]string{{"echo", "first;", "echo", "second"}}, // Semicolon not handled specially
		},
		{
			name:  "BackgroundOperator",
			input: "sleep 10 &",
			want:  [][]string{{"sleep", "10", "&"}}, // & not handled specially
		},
		{
			name:  "SubshellWithOperators",
			input: "$(false || true) && echo ok",
			want:  [][]string{{"$(false", "||", "true)", "&&", "echo", "ok"}},
		},
		{
			name:  "Issue1065ClamscanWithGrepAndORFallback",
			input: `clamscan -r / 2>&1 | grep -A 20 "SCAN SUMMARY" || true`,
			want:  [][]string{{"clamscan", "-r", "/", "2>&1"}, {"grep", "-A", "20", "SCAN SUMMARY", "||", "true"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePipedCommand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePipedCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePipedCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDetectShebang tests shebang detection in scripts
func TestDetectShebang(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		wantCmd  string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "WithShebang",
			script:   "#!/bin/bash\necho hello",
			wantCmd:  "/bin/bash",
			wantArgs: []string{},
		},
		{
			name:     "WithShebangAndArgs",
			script:   "#!/usr/bin/env python3\nprint('hello')",
			wantCmd:  "/usr/bin/env",
			wantArgs: []string{"python3"},
		},
		{
			name:     "WithoutShebang",
			script:   "echo hello",
			wantCmd:  "",
			wantArgs: nil,
		},
		{
			name:     "EmptyScript",
			script:   "",
			wantCmd:  "",
			wantArgs: nil,
		},
		{
			name:     "OnlyShebang",
			script:   "#!",
			wantCmd:  "",
			wantArgs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotArgs, err := DetectShebang(tt.script)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectShebang() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotCmd != tt.wantCmd {
				t.Errorf("DetectShebang() gotCmd = %v, want %v", gotCmd, tt.wantCmd)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("DetectShebang() gotArgs = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

// TestSplitCommandShellOperators tests SplitCommand behavior with shell operators
func TestSplitCommandShellOperators(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCmd  string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "OROperatorFalseTrue",
			input:    "false || true",
			wantCmd:  "false",
			wantArgs: []string{"||", "true"}, // Correct behavior: || stays as single token
		},
		{
			name:     "ANDOperatorTrueEchoSuccess",
			input:    "true && echo success",
			wantCmd:  "true",
			wantArgs: []string{"&&", "echo", "success"},
		},
		{
			name:     "MixedPipeAndOR",
			input:    "echo hello | grep hello || echo not found",
			wantCmd:  "echo",
			wantArgs: []string{"hello", "|", "grep", "hello", "||", "echo", "not", "found"}, // Fixed: || stays as single token
		},
		{
			name:     "ComplexConditional",
			input:    "test -f file && cat file || touch file",
			wantCmd:  "test",
			wantArgs: []string{"-f", "file", "&&", "cat", "file", "||", "touch", "file"}, // Fixed: || stays as single token
		},
		{
			name:     "Issue1065ClamscanCommandWithPipeAndOR",
			input:    `clamscan -r / 2>&1 | grep -A 20 "SCAN SUMMARY" || true`,
			wantCmd:  "clamscan",
			wantArgs: []string{"-r", "/", "2>&1", "|", "grep", "-A", "20", "SCAN SUMMARY", "||", "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotArgs, err := SplitCommand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SplitCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotCmd != tt.wantCmd {
				t.Errorf("SplitCommand() gotCmd = %v, want %v", gotCmd, tt.wantCmd)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("SplitCommand() gotArgs = %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

// TestGetScriptExtension tests file extension detection for different shells
func TestGetScriptExtension(t *testing.T) {
	tests := []struct {
		name         string
		shellCommand string
		want         string
	}{
		// PowerShell Core (pwsh)
		{
			name:         "PwshSimple",
			shellCommand: "pwsh",
			want:         ".ps1",
		},
		{
			name:         "PwshExe",
			shellCommand: "pwsh.exe",
			want:         ".ps1",
		},
		{
			name:         "PwshFullPathWindows",
			shellCommand: "C:\\PowerShell\\pwsh.exe",
			want:         ".ps1",
		},
		{
			name:         "PwshFullPathUnix",
			shellCommand: "/usr/bin/pwsh",
			want:         ".ps1",
		},
		// Windows PowerShell
		{
			name:         "PowerShellSimple",
			shellCommand: "powershell",
			want:         ".ps1",
		},
		{
			name:         "PowerShellExe",
			shellCommand: "powershell.exe",
			want:         ".ps1",
		},
		{
			name:         "PowerShellFullPath",
			shellCommand: "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
			want:         ".ps1",
		},
		// Windows cmd
		{
			name:         "CmdSimple",
			shellCommand: "cmd",
			want:         ".bat",
		},
		{
			name:         "CmdExe",
			shellCommand: "cmd.exe",
			want:         ".bat",
		},
		{
			name:         "CmdFullPath",
			shellCommand: "C:\\Windows\\System32\\cmd.exe",
			want:         ".bat",
		},
		// Unix shells
		{
			name:         "BashSimple",
			shellCommand: "bash",
			want:         ".sh",
		},
		{
			name:         "ShSimple",
			shellCommand: "sh",
			want:         ".sh",
		},
		{
			name:         "ZshSimple",
			shellCommand: "zsh",
			want:         ".sh",
		},
		{
			name:         "BashFullPath",
			shellCommand: "/bin/bash",
			want:         ".sh",
		},
		{
			name:         "FishShell",
			shellCommand: "fish",
			want:         "",
		},
		// Edge cases
		{
			name:         "EmptyString",
			shellCommand: "",
			want:         "",
		},
		{
			name:         "NixShell",
			shellCommand: "nix-shell",
			want:         "",
		},
		{
			name:         "PythonInterpreter",
			shellCommand: "python",
			want:         "",
		},
		// Case insensitivity
		{
			name:         "PwshUppercase",
			shellCommand: "PWSH",
			want:         ".ps1",
		},
		{
			name:         "PowerShellMixedCase",
			shellCommand: "PowerShell.EXE",
			want:         ".ps1",
		},
		{
			name:         "CmdUppercase",
			shellCommand: "CMD.EXE",
			want:         ".bat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetScriptExtension(tt.shellCommand)
			if got != tt.want {
				t.Errorf("GetScriptExtension(%q) = %q, want %q", tt.shellCommand, got, tt.want)
			}
		})
	}
}
