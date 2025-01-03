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

func TestSplitCommandWithParse(t *testing.T) {
	t.Run("CommandSubstitution", func(t *testing.T) {
		cmd, args, err := SplitCommandWithEval("echo `echo hello`")
		require.NoError(t, err)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "hello", args[0])
	})
	t.Run("QuotedCommandSubstitution", func(t *testing.T) {
		cmd, args, err := SplitCommandWithEval("echo `echo \"hello world\"`")
		require.NoError(t, err)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "hello world", args[0])
	})
	t.Run("EnvVar", func(t *testing.T) {
		os.Setenv("TEST_ARG", "hello")
		cmd, args, err := SplitCommandWithEval("echo $TEST_ARG")
		require.NoError(t, err)
		require.Equal(t, "echo", cmd)
		require.Len(t, args, 1)
		require.Equal(t, "$TEST_ARG", args[0]) // env var should not be expanded
	})
}

func TestSubstituteStringFields(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("NESTED_VAR", "nested_value")
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("NESTED_VAR")

	type Nested struct {
		NestedField   string
		NestedCommand string
		unexported    string
	}

	type TestStruct struct {
		SimpleField  string
		EnvField     string
		CommandField string
		MultiField   string
		EmptyField   string
		unexported   string
		NestedStruct Nested
	}

	tests := []struct {
		name    string
		input   TestStruct
		want    TestStruct
		wantErr bool
	}{
		{
			name: "basic substitution",
			input: TestStruct{
				SimpleField:  "hello",
				EnvField:     "$TEST_VAR",
				CommandField: "`echo hello`",
				MultiField:   "$TEST_VAR and `echo command`",
				EmptyField:   "",
				NestedStruct: Nested{
					NestedField:   "$NESTED_VAR",
					NestedCommand: "`echo nested`",
					unexported:    "should not change",
				},
				unexported: "should not change",
			},
			want: TestStruct{
				SimpleField:  "hello",
				EnvField:     "test_value",
				CommandField: "hello",
				MultiField:   "test_value and command",
				EmptyField:   "",
				NestedStruct: Nested{
					NestedField:   "nested_value",
					NestedCommand: "nested",
					unexported:    "should not change",
				},
				unexported: "should not change",
			},
			wantErr: false,
		},
		{
			name: "invalid command",
			input: TestStruct{
				CommandField: "`invalid_command_that_does_not_exist`",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvalStringFields(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SubstituteStringFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SubstituteStringFields() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSubstituteStringFields_AnonymousStruct(t *testing.T) {
	obj, err := EvalStringFields(struct {
		Field string
	}{
		Field: "`echo hello`",
	})
	require.NoError(t, err)
	require.Equal(t, "hello", obj.Field)
}

func TestSubstituteStringFields_NonStruct(t *testing.T) {
	_, err := EvalStringFields("not a struct")
	if err == nil {
		t.Error("SubstituteStringFields() should return error for non-struct input")
	}
}

func TestSubstituteStringFields_NestedStructs(t *testing.T) {
	type DeepNested struct {
		Field string
	}

	type Nested struct {
		Field      string
		DeepNested DeepNested
	}

	type Root struct {
		Field  string
		Nested Nested
	}

	input := Root{
		Field: "$TEST_VAR",
		Nested: Nested{
			Field: "`echo nested`",
			DeepNested: DeepNested{
				Field: "$NESTED_VAR",
			},
		},
	}

	// Set up environment
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("NESTED_VAR", "deep_nested_value")
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("NESTED_VAR")

	want := Root{
		Field: "test_value",
		Nested: Nested{
			Field: "nested",
			DeepNested: DeepNested{
				Field: "deep_nested_value",
			},
		},
	}

	got, err := EvalStringFields(input)
	if err != nil {
		t.Fatalf("SubstituteStringFields() error = %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("SubstituteStringFields() = %+v, want %+v", got, want)
	}
}

func TestSubstituteStringFields_EmptyStruct(t *testing.T) {
	type Empty struct{}

	input := Empty{}
	got, err := EvalStringFields(input)
	if err != nil {
		t.Fatalf("SubstituteStringFields() error = %v", err)
	}

	if !reflect.DeepEqual(got, input) {
		t.Errorf("SubstituteStringFields() = %+v, want %+v", got, input)
	}
}

func TestReplaceVars(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "basic substitution",
			template: "${FOO}",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "BAR",
		},
		{
			name:     "short syntax",
			template: "$FOO",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "BAR",
		},
		{
			name:     "no substitution",
			template: "$FOO_",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "$FOO_",
		},
		{
			name:     "in middle of string",
			template: "prefix $FOO suffix",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "prefix BAR suffix",
		},
		{
			name:     "in middle of string and no substitution",
			template: "prefix $FOO1 suffix",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "prefix $FOO1 suffix",
		},
		{
			name:     "missing var",
			template: "${MISSING}",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "${MISSING}",
		},
		{
			name:     "multiple vars",
			template: "$FOO ${BAR} $BAZ",
			vars: map[string]string{
				"FOO": "1",
				"BAR": "2",
				"BAZ": "3",
			},
			want: "1 2 3",
		},
		{
			name:     "nested vars not supported",
			template: "${FOO${BAR}}",
			vars:     map[string]string{"FOO": "1", "BAR": "2"},
			want:     "${FOO${BAR}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceVars(tt.template, tt.vars)
			if got != tt.want {
				t.Errorf("replaceVars() = %v, want %v", got, tt.want)
			}
		})
	}
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
