package eval

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptions_Defaults(t *testing.T) {
	opts := NewOptions()
	assert.True(t, opts.ExpandEnv, "ExpandEnv should default to true")
	assert.True(t, opts.ExpandShell, "ExpandShell should default to true")
	assert.True(t, opts.Substitute, "Substitute should default to true")
	assert.Nil(t, opts.Variables, "Variables should default to nil")
	assert.Nil(t, opts.StepMap, "StepMap should default to nil")
}

func TestWithoutExpandShell(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		input   string
		opts    []Option
		want    string
		wantErr bool
	}{
		{
			name:    "ShellExpansionEnabled",
			input:   "${VAR:0:3}",
			opts:    []Option{WithVariables(map[string]string{"VAR": "HelloWorld"})},
			want:    "Hel",
			wantErr: false,
		},
		{
			name:    "ShellExpansionDisabledPreservesSubstring",
			input:   "${VAR:0:3}",
			opts:    []Option{WithVariables(map[string]string{"VAR": "HelloWorld"}), WithoutExpandShell()},
			want:    "${VAR:0:3}",
			wantErr: false,
		},
		{
			name:    "SimpleVarStillWorks",
			input:   "${VAR}",
			opts:    []Option{WithVariables(map[string]string{"VAR": "value"}), WithoutExpandShell()},
			want:    "value",
			wantErr: false,
		},
		{
			name:    "EnvVarStillExpandsWithoutShellExpansion",
			input:   "$TEST_VAR",
			opts:    []Option{WithoutExpandShell()},
			want:    "test_value_for_shell",
			wantErr: false,
		},
		{
			name:    "CommandSubstitutionStillWorks",
			input:   "`echo hello`",
			opts:    []Option{WithoutExpandShell()},
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "MixedContentWithShellDisabled",
			input:   "prefix ${VAR} suffix",
			opts:    []Option{WithVariables(map[string]string{"VAR": "middle"}), WithoutExpandShell()},
			want:    "prefix middle suffix",
			wantErr: false,
		},
	}

	require.NoError(t, os.Setenv("TEST_VAR", "test_value_for_shell"))
	defer func() { _ = os.Unsetenv("TEST_VAR") }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := String(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestOptions_Combinations(t *testing.T) {
	t.Setenv("TEST_ENV", "env_value")

	tests := []struct {
		name  string
		input string
		opts  []Option
		want  string
	}{
		{
			name:  "AllFeaturesDisabled",
			input: "$TEST_ENV `echo hello` ${VAR}",
			opts: []Option{
				WithoutExpandEnv(),
				WithoutSubstitute(),
			},
			want: "$TEST_ENV `echo hello` ${VAR}",
		},
		{
			name:  "OnlyVariablesEnabled",
			input: "$TEST_ENV `echo hello` ${VAR}",
			opts: []Option{
				OnlyReplaceVars(),
				WithVariables(map[string]string{"VAR": "value"}),
			},
			want: "$TEST_ENV `echo hello` value",
		},
		{
			name:  "MultipleVariableSetsWithStepMap",
			input: "${VAR1} ${VAR2} ${step1.exit_code}",
			opts: []Option{
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
			got, err := String(ctx, tt.input, tt.opts...)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
