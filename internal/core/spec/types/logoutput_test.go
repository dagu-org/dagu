package types

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogOutputValue_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMode    LogOutputMode
		wantSet     bool
		wantErr     bool
		errContains string
	}{
		{
			name:     "separate mode",
			input:    "logOutput: separate",
			wantMode: LogOutputSeparate,
			wantSet:  true,
		},
		{
			name:     "merged mode",
			input:    "logOutput: merged",
			wantMode: LogOutputMerged,
			wantSet:  true,
		},
		{
			name:     "merged mode uppercase",
			input:    "logOutput: MERGED",
			wantMode: LogOutputMerged,
			wantSet:  true,
		},
		{
			name:     "separate mode mixed case",
			input:    "logOutput: Separate",
			wantMode: LogOutputSeparate,
			wantSet:  true,
		},
		{
			name:     "empty string defaults to separate",
			input:    "logOutput: ''",
			wantMode: LogOutputSeparate,
			wantSet:  true,
		},
		{
			name:        "invalid value",
			input:       "logOutput: invalid",
			wantErr:     true,
			errContains: "invalid logOutput value",
		},
		{
			name:        "invalid value - both",
			input:       "logOutput: both",
			wantErr:     true,
			errContains: "invalid logOutput value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				LogOutput LogOutputValue `yaml:"logOutput"`
			}

			err := yaml.Unmarshal([]byte(tt.input), &result)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, result.LogOutput.Mode())
			assert.Equal(t, tt.wantSet, !result.LogOutput.IsZero())
		})
	}
}

func TestLogOutputValue_UnmarshalYAML_InvalidType(t *testing.T) {
	var result struct {
		LogOutput LogOutputValue `yaml:"logOutput"`
	}

	err := yaml.Unmarshal([]byte("logOutput:\n  - item1\n  - item2"), &result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a string")
}

func TestLogOutputValue_DefaultValue(t *testing.T) {
	var result struct {
		LogOutput LogOutputValue `yaml:"logOutput"`
	}

	// When not set in YAML
	err := yaml.Unmarshal([]byte("other: value"), &result)
	require.NoError(t, err)

	// Should be zero
	assert.True(t, result.LogOutput.IsZero())
	// Default mode should be separate
	assert.Equal(t, LogOutputSeparate, result.LogOutput.Mode())
	assert.True(t, result.LogOutput.IsSeparate())
	assert.False(t, result.LogOutput.IsMerged())
}

func TestLogOutputValue_Methods(t *testing.T) {
	t.Run("IsMerged", func(t *testing.T) {
		merged := LogOutputValue{mode: LogOutputMerged, set: true}
		assert.True(t, merged.IsMerged())
		assert.False(t, merged.IsSeparate())

		separate := LogOutputValue{mode: LogOutputSeparate, set: true}
		assert.False(t, separate.IsMerged())
		assert.True(t, separate.IsSeparate())
	})

	t.Run("String", func(t *testing.T) {
		merged := LogOutputValue{mode: LogOutputMerged, set: true}
		assert.Equal(t, "merged", merged.String())

		separate := LogOutputValue{mode: LogOutputSeparate, set: true}
		assert.Equal(t, "separate", separate.String())

		unset := LogOutputValue{}
		assert.Equal(t, "separate", unset.String()) // default
	})
}

func TestLogOutputMode_Constants(t *testing.T) {
	// Ensure constants have expected values
	assert.Equal(t, LogOutputMode("separate"), LogOutputSeparate)
	assert.Equal(t, LogOutputMode("merged"), LogOutputMerged)
}
