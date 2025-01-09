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
		require.Equal(t, `"{\"key\":\"value\"}"`, args[0])
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
			name:     "simple command no args",
			input:    "echo",
			wantCmd:  "echo",
			wantArgs: []string{},
		},
		{
			name:     "command with single arg",
			input:    "echo hello",
			wantCmd:  "echo",
			wantArgs: []string{"hello"},
		},
		{
			name:     "command with backtick",
			input:    "echo `echo hello`",
			wantCmd:  "echo",
			wantArgs: []string{"`echo hello`"},
		},
		{
			name:     "command with multiple args",
			input:    "echo hello world",
			wantCmd:  "echo",
			wantArgs: []string{"hello", "world"},
		},
		{
			name:     "command with quoted args",
			input:    `echo "hello world"`,
			wantCmd:  "echo",
			wantArgs: []string{"\"hello world\""},
		},
		{
			name:     "command with pipe",
			input:    "echo foo | grep foo",
			wantCmd:  "echo",
			wantArgs: []string{"foo", "|", "grep", "foo"},
		},
		{
			name:     "complex pipe command",
			input:    "echo foo | grep foo | wc -l",
			wantCmd:  "echo",
			wantArgs: []string{"foo", "|", "grep", "foo", "|", "wc", "-l"},
		},
		{
			name:     "command with quoted pipe",
			input:    `echo "hello|world"`,
			wantCmd:  "echo",
			wantArgs: []string{"\"hello|world\""},
		},
		{
			name:      "empty command",
			input:     "",
			wantErr:   true,
			errorType: ErrCommandIsEmpty,
		},
		{
			name:     "command with escaped quotes",
			input:    `echo "\"hello world\""`,
			wantCmd:  "echo",
			wantArgs: []string{`"\"hello world\""`},
		},
		{
			name:     "command with JSON",
			input:    `echo "{\n\t\"key\": \"value\"\n}"`,
			wantCmd:  "echo",
			wantArgs: []string{`"{\n\t\"key\": \"value\"\n}"`},
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
		os.Setenv("TEST_ARG", "hello")
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
			name: "piping",
			cmd:  "echo",
			args: []string{"hello", "|", "wc", "-c"},
			want: "echo hello | wc -c",
		},
		{
			name: "redirection",
			cmd:  "echo",
			args: []string{"'test content'", ">", "testfile.txt", "&&", "cat", "testfile.txt"},
			want: `echo 'test content' > testfile.txt && cat testfile.txt`,
		},
		{
			name: `key="value" argument`,
			cmd:  "echo",
			args: []string{`key="value"`},
			want: `echo key="value"`,
		},
		{
			name: "JSON argument",
			cmd:  "echo",
			args: []string{`{"foo":"bar","hello":"world"}`},
			want: `echo {"foo":"bar","hello":"world"}`,
		},
		{
			name: "key=value argument",
			cmd:  "echo",
			args: []string{`key="some value"`},
			want: `echo key="some value"`,
		},
		{
			name: "double quotes",
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
			name:  "simple command no args",
			input: "echo",
			want:  [][]string{{"echo"}},
		},
		{
			name:  "simple command with args",
			input: "echo foo bar",
			want:  [][]string{{"echo", "foo", "bar"}},
		},
		{
			name:  "command with quoted args",
			input: `echo "hello world"`,
			want:  [][]string{{"echo", `"hello world"`}},
		},
		{
			name:  "command with pipe",
			input: "echo foo | grep foo",
			want:  [][]string{{"echo", "foo"}, {"grep", "foo"}},
		},
		{
			name:  "multiple pipes",
			input: "echo foo | grep foo | wc -l",
			want:  [][]string{{"echo", "foo"}, {"grep", "foo"}, {"wc", "-l"}},
		},
		{
			name:  "pipe in quotes",
			input: `echo "hello|world"`,
			want:  [][]string{{"echo", `"hello|world"`}},
		},
		{
			name:  "multiple spaces between commands",
			input: "echo foo    |    grep foo",
			want:  [][]string{{"echo", "foo"}, {"grep", "foo"}},
		},
		{
			name:  "command with backticks",
			input: "echo `date`",
			want:  [][]string{{"echo", "`date`"}},
		},
		{
			name:  "pipe in backticks",
			input: "echo `echo foo | grep foo`",
			want:  [][]string{{"echo", "`echo foo | grep foo`"}},
		},
		{
			name:  "escaped quotes",
			input: `echo "Hello \"World\""`,
			want:  [][]string{{"echo", `"Hello \"World\""`}},
		},
		{
			name:  "escaped pipe",
			input: `echo foo\|bar`,
			want:  [][]string{{"echo", `foo\|bar`}},
		},
		{
			name:  "empty command",
			input: "",
			want:  [][]string{},
		},
		{
			name:  "mixed quotes and backticks",
			input: "echo \"hello\" world `date`",
			want:  [][]string{{"echo", `"hello"`, "world", "`date`"}},
		},
		{
			name:  "complex pipeline",
			input: `find . -name "*.go" | xargs grep "fmt" | sort | uniq -c`,
			want: [][]string{
				{"find", ".", "-name", `"*.go"`},
				{"xargs", "grep", `"fmt"`},
				{"sort"},
				{"uniq", "-c"},
			},
		},
		{
			name:  "command with environment variables",
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
			name:     "unterminated quote",
			input:    `echo "hello`,
			wantPipe: [][]string{{"echo", `"hello`}},
		},
		{
			name:     "unterminated backtick",
			input:    "echo `date",
			wantPipe: [][]string{{"echo", "`date"}},
		},
		{
			name:     "mixed unterminated quotes",
			input:    "echo \"hello `date`\"",
			wantPipe: [][]string{{"echo", "\"hello `date`\""}},
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
