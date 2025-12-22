package types_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContinueOnValue_UnmarshalYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		wantErr         bool
		errContains     string
		wantSkipped     bool
		wantFailed      bool
		wantExitCode    []int
		wantOutput      []string
		wantMarkSuccess bool
		checkIsZero     bool
		checkNotZero    bool
	}{
		{
			name:         "StringSkipped",
			input:        "skipped",
			wantSkipped:  true,
			wantFailed:   false,
			checkNotZero: true,
		},
		{
			name:        "StringFailed",
			input:       "failed",
			wantSkipped: false,
			wantFailed:  true,
		},
		{
			name:        "StringCaseInsensitiveSKIPPED",
			input:       "SKIPPED",
			wantSkipped: true,
		},
		{
			name:       "StringCaseInsensitiveFailed",
			input:      "Failed",
			wantFailed: true,
		},
		{
			name:        "StringWithWhitespace",
			input:       `"  skipped  "`,
			wantSkipped: true,
		},
		{
			name:        "MapFormSkippedOnly",
			input:       "skipped: true",
			wantSkipped: true,
			wantFailed:  false,
		},
		{
			name:        "MapFormFailedOnly",
			input:       "failed: true",
			wantSkipped: false,
			wantFailed:  true,
		},
		{
			name: "MapFormBoth",
			input: `
skipped: true
failed: true
`,
			wantSkipped: true,
			wantFailed:  true,
		},
		{
			name:         "MapWithExitCodesArray",
			input:        "exitCode: [0, 1, 2]",
			wantExitCode: []int{0, 1, 2},
		},
		{
			name:         "MapWithSingleExitCode",
			input:        "exitCode: 1",
			wantExitCode: []int{1},
		},
		{
			name:       "MapWithOutputPattern",
			input:      `output: "success|warning"`,
			wantOutput: []string{"success|warning"},
		},
		{
			name: "MapWithAllFields",
			input: `
skipped: true
failed: true
exitCode: [0, 1]
output: "OK"
markSuccess: true
`,
			wantSkipped:     true,
			wantFailed:      true,
			wantExitCode:    []int{0, 1},
			wantOutput:      []string{"OK"},
			wantMarkSuccess: true,
		},
		{
			name:        "InvalidStringValue",
			input:       "invalid",
			wantErr:     true,
			errContains: "expected 'skipped' or 'failed'",
		},
		{
			name:        "InvalidMapKey",
			input:       "unknown: true",
			wantErr:     true,
			errContains: "unknown key",
		},
		{
			name:        "InvalidSkippedType",
			input:       `skipped: "yes"`,
			wantErr:     true,
			errContains: "expected bool",
		},
		{
			name:        "InvalidExitCodeType",
			input:       `exitCode: "not a number"`,
			wantErr:     true,
			errContains: "cannot parse",
		},
		{
			name:        "InvalidTypeArray",
			input:       "[1, 2, 3]",
			wantErr:     true,
			errContains: "must be string or map",
		},
		{
			name:       "OutputAsStringArray",
			input:      `output: ["success", "warning", "info"]`,
			wantOutput: []string{"success", "warning", "info"},
		},
		{
			name:         "ExitCodeAsInt64",
			input:        "exitCode: 255",
			wantExitCode: []int{255},
		},
		{
			name:         "ExitCodeAsString",
			input:        `exitCode: "42"`,
			wantExitCode: []int{42},
		},
		{
			name:         "ExitCodeArrayWithMixedTypes",
			input:        `exitCode: [0, "1", 2]`,
			wantExitCode: []int{0, 1, 2},
		},
		{
			name:        "OutputInvalidTypeInArray",
			input:       `output: [123, true]`,
			wantErr:     true,
			errContains: "expected string",
		},
		{
			name:        "ExitCodeInvalidString",
			input:       `exitCode: "not-a-number"`,
			wantErr:     true,
			errContains: "cannot parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var c types.ContinueOnValue
			err := yaml.Unmarshal([]byte(tt.input), &c)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			if tt.checkIsZero {
				assert.True(t, c.IsZero())
				return
			}
			if tt.checkNotZero {
				assert.False(t, c.IsZero())
			}
			if tt.wantSkipped {
				assert.True(t, c.Skipped())
			}
			if tt.wantFailed {
				assert.True(t, c.Failed())
			}
			if tt.wantExitCode != nil {
				assert.Equal(t, tt.wantExitCode, c.ExitCode())
			}
			if tt.wantOutput != nil {
				assert.Equal(t, tt.wantOutput, c.Output())
			}
			if tt.wantMarkSuccess {
				assert.True(t, c.MarkSuccess())
			}
		})
	}

	t.Run("ZeroValue", func(t *testing.T) {
		t.Parallel()
		var c types.ContinueOnValue
		assert.True(t, c.IsZero())
		assert.False(t, c.Skipped())
		assert.False(t, c.Failed())
	})
}

func TestContinueOnValue_InStruct(t *testing.T) {
	t.Parallel()

	type StepConfig struct {
		Name       string                `yaml:"name"`
		ContinueOn types.ContinueOnValue `yaml:"continueOn"`
	}

	tests := []struct {
		name         string
		input        string
		wantSkipped  bool
		wantFailed   bool
		wantExitCode []int
		wantIsZero   bool
	}{
		{
			name: "ContinueOnAsString",
			input: `
name: my-step
continueOn: skipped
`,
			wantSkipped: true,
		},
		{
			name: "ContinueOnAsMap",
			input: `
name: my-step
continueOn:
  failed: true
  exitCode: [0, 1]
`,
			wantFailed:   true,
			wantExitCode: []int{0, 1},
		},
		{
			name:       "ContinueOnNotSet",
			input:      "name: my-step",
			wantIsZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var cfg StepConfig
			err := yaml.Unmarshal([]byte(tt.input), &cfg)
			require.NoError(t, err)
			if tt.wantIsZero {
				assert.True(t, cfg.ContinueOn.IsZero())
				return
			}
			if tt.wantSkipped {
				assert.True(t, cfg.ContinueOn.Skipped())
			}
			if tt.wantFailed {
				assert.True(t, cfg.ContinueOn.Failed())
			}
			if tt.wantExitCode != nil {
				assert.Equal(t, tt.wantExitCode, cfg.ContinueOn.ExitCode())
			}
		})
	}
}

func TestContinueOnValue_Value(t *testing.T) {
	t.Parallel()

	t.Run("ValueReturnsRawString", func(t *testing.T) {
		t.Parallel()
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte("skipped"), &c)
		require.NoError(t, err)
		assert.Equal(t, "skipped", c.Value())
	})

	t.Run("ValueReturnsMap", func(t *testing.T) {
		t.Parallel()
		var c types.ContinueOnValue
		err := yaml.Unmarshal([]byte("failed: true"), &c)
		require.NoError(t, err)
		val, ok := c.Value().(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, val["failed"])
	})
}

func TestContinueOnValue_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantErr       bool
		errContains   string
		checkIsZero   bool
		wantFailed    bool
		wantExitCode  []int
		wantOutputNil bool
	}{
		{
			name:        "NullValue",
			input:       "null",
			checkIsZero: true,
		},
		{
			name:       "FailureKeyAlias",
			input:      "failure: true",
			wantFailed: true,
		},
		{
			name:         "ExitCodeAsFloat",
			input:        "exitCode: 1.0",
			wantExitCode: []int{1},
		},
		{
			name:         "ExitCodeArrayWithFloat",
			input:        "exitCode: [1.0, 2.0]",
			wantExitCode: []int{1, 2},
		},
		{
			name:        "InvalidExitCodeTypeInArray",
			input:       "exitCode: [true]",
			wantErr:     true,
			errContains: "expected int",
		},
		{
			name:        "InvalidExitCodeTypeNotIntOrArray",
			input:       "exitCode: {key: value}",
			wantErr:     true,
			errContains: "expected int or array",
		},
		{
			name:          "OutputAsNil",
			input:         "output: null",
			wantOutputNil: true,
		},
		{
			name:          "OutputAsEmptyString",
			input:         `output: ""`,
			wantOutputNil: true,
		},
		{
			name:        "OutputInvalidTypeNotStringOrArray",
			input:       "output: 123",
			wantErr:     true,
			errContains: "expected string or array",
		},
		{
			name:        "MarkSuccessInvalidType",
			input:       `markSuccess: "yes"`,
			wantErr:     true,
			errContains: "expected bool",
		},
		{
			name:        "FailedInvalidType",
			input:       `failed: "yes"`,
			wantErr:     true,
			errContains: "expected bool",
		},
		{
			name:        "ExitCodeInvalidStringInArray",
			input:       `exitCode: ["not-a-number"]`,
			wantErr:     true,
			errContains: "cannot parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var c types.ContinueOnValue
			err := yaml.Unmarshal([]byte(tt.input), &c)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			if tt.checkIsZero {
				assert.True(t, c.IsZero())
			}
			if tt.wantFailed {
				assert.True(t, c.Failed())
			}
			if tt.wantExitCode != nil {
				assert.Equal(t, tt.wantExitCode, c.ExitCode())
			}
			if tt.wantOutputNil {
				assert.Nil(t, c.Output())
			}
		})
	}
}
