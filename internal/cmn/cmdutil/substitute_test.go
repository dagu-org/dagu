package cmdutil

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
		name       string
		input      string
		want       string
		wantErr    bool
		setupEnv   map[string]string
		cleanupEnv []string
		skipOnOS   []string
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
			cleanupEnv: []string{"TEST_VAR"},
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
			if tt.setupEnv != nil {
				for k, v := range tt.setupEnv {
					oldValue := os.Getenv(k)
					_ = os.Setenv(k, v)
					defer func() {
						_ = os.Setenv(k, oldValue)
					}()
				}
			}

			// Run test
			got, err := substituteCommandsWithContext(context.Background(), tt.input)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("substituteCommandsWithContext(context.Background(),) error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expect an error, don't check the output
			if tt.wantErr {
				return
			}

			// Compare output
			if got != tt.want {
				t.Errorf("substituteCommandsWithContext(context.Background(),) = %q, want %q", got, tt.want)
			}
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
			if (err != nil) != tt.wantErr {
				t.Errorf("substituteCommandsWithContext(context.Background(),) error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("substituteCommandsWithContext(context.Background(),) = %q, want %q", got, tt.want)
			}
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
			wantErr: true, // Command returns error on non-zero exit code
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
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExpandReferences_ComplexJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		want    string
	}{
		{
			name:  "ArrayAccess",
			input: "${DATA.items.[1].name}",
			dataMap: map[string]string{
				"DATA": `{"items": [{"name": "first"}, {"name": "second"}, {"name": "third"}]}`,
			},
			want: "second",
		},
		{
			name:  "BooleanValue",
			input: "${CONFIG.enabled}",
			dataMap: map[string]string{
				"CONFIG": `{"enabled": true}`,
			},
			want: "true",
		},
		{
			name:  "NumberValue",
			input: "${CONFIG.port}",
			dataMap: map[string]string{
				"CONFIG": `{"port": 8080}`,
			},
			want: "8080",
		},
		{
			name:  "NullValue",
			input: "${CONFIG.optional}",
			dataMap: map[string]string{
				"CONFIG": `{"optional": null}`,
			},
			want: "<nil>",
		},
		{
			name:  "DeeplyNested",
			input: "${DATA.level1.level2.level3.value}",
			dataMap: map[string]string{
				"DATA": `{"level1": {"level2": {"level3": {"value": "deep"}}}}`,
			},
			want: "deep",
		},
		{
			name:  "ArrayOfObjects",
			input: "${USERS.[0].email}",
			dataMap: map[string]string{
				"USERS": `[{"name": "Alice", "email": "alice@example.com"}, {"name": "Bob", "email": "bob@example.com"}]`,
			},
			want: "alice@example.com",
		},
		{
			name:  "SpecialCharactersInJSON",
			input: "${DATA.message}",
			dataMap: map[string]string{
				"DATA": `{"message": "Hello \"World\" with 'quotes'"}`,
			},
			want: `Hello "World" with 'quotes'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := ExpandReferences(ctx, tt.input, tt.dataMap)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvalStringFields_StructWithMapField(t *testing.T) {
	type TestStruct struct {
		Name    string
		Config  map[string]string
		Options map[string]any
	}

	input := TestStruct{
		Name: "`echo test`",
		Config: map[string]string{
			"key1": "$TEST_VAR",
			"key2": "`echo value`",
		},
		Options: map[string]any{
			"enabled": true,
			"command": "`echo option`",
			"nested": map[string]any{
				"value": "$TEST_VAR",
			},
		},
	}

	// Set up environment
	t.Setenv("TEST_VAR", "env_value")

	ctx := context.Background()
	got, err := EvalStringFields(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, "test", got.Name)
	assert.Equal(t, "env_value", got.Config["key1"])
	assert.Equal(t, "value", got.Config["key2"])
	assert.Equal(t, true, got.Options["enabled"])
	assert.Equal(t, "option", got.Options["command"])
	assert.Equal(t, "env_value", got.Options["nested"].(map[string]any)["value"])
}

func TestEvalStringFields_ErrorCases(t *testing.T) {
	// Test with a channel (unsupported type)
	ch := make(chan int)
	_, err := EvalStringFields(context.Background(), ch)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input must be a struct or map")

	// Test struct with invalid command
	type TestStruct struct {
		Field string
	}
	input := TestStruct{
		Field: "`invalid_command_xyz`",
	}
	_, err = EvalStringFields(context.Background(), input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process struct fields")

	// Test map with invalid command
	mapInput := map[string]any{
		"key": "`invalid_command_xyz`",
	}
	_, err = EvalStringFields(context.Background(), mapInput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to process map")
}

func TestEvalOptions_Combinations(t *testing.T) {
	// Set up environment
	t.Setenv("TEST_ENV", "env_value")

	tests := []struct {
		name  string
		input string
		opts  []EvalOption
		want  string
	}{
		{
			name:  "AllFeaturesDisabled",
			input: "$TEST_ENV `echo hello` ${VAR}",
			opts: []EvalOption{
				WithoutExpandEnv(),
				WithoutSubstitute(),
			},
			want: "$TEST_ENV `echo hello` ${VAR}",
		},
		{
			name:  "OnlyVariablesEnabled",
			input: "$TEST_ENV `echo hello` ${VAR}",
			opts: []EvalOption{
				OnlyReplaceVars(),
				WithVariables(map[string]string{"VAR": "value"}),
			},
			want: "$TEST_ENV `echo hello` value",
		},
		{
			name:  "MultipleVariableSetsWithStepMap",
			input: "${VAR1} ${VAR2} ${step1.exit_code}",
			opts: []EvalOption{
				WithVariables(map[string]string{"VAR1": "first"}),
				WithVariables(map[string]string{"VAR2": "second"}),
				WithStepMap(map[string]StepInfo{
					"step1": {ExitCode: "0"},
				}),
			},
			want: "first second 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := EvalString(ctx, tt.input, tt.opts...)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProcessMap_NilValues(t *testing.T) {
	input := map[string]any{
		"string": "value",
		"nil":    nil,
		"ptr":    (*string)(nil),
		"iface":  any(nil),
	}

	ctx := context.Background()
	got, err := EvalStringFields(ctx, input)
	require.NoError(t, err)

	gotMap := got
	assert.Equal(t, "value", gotMap["string"])
	assert.Nil(t, gotMap["nil"])
	assert.Nil(t, gotMap["ptr"])
	assert.Nil(t, gotMap["iface"])
}
