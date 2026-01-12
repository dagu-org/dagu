package masking

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMasker_MaskString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sources  SourcedEnvVars
		input    string
		expected string
	}{
		{
			name: "single value",
			sources: SourcedEnvVars{
				Secrets: []string{"API_KEY=secret123"},
			},
			input:    "The API key is secret123",
			expected: "The API key is *******",
		},
		{
			name: "multiple values",
			sources: SourcedEnvVars{
				Secrets: []string{
					"API_KEY=secret123",
					"PASSWORD=pass456",
				},
			},
			input:    "Key: secret123, Pass: pass456",
			expected: "Key: *******, Pass: *******",
		},
		{
			name: "overlapping values",
			sources: SourcedEnvVars{
				Secrets: []string{"SHORT=abc", "LONG=abcdef"},
			},
			input:    "Value: abcdef",
			expected: "Value: *******",
		},
		{
			name: "no match",
			sources: SourcedEnvVars{
				Secrets: []string{"API_KEY=secret123"},
			},
			input:    "This has no secrets",
			expected: "This has no secrets",
		},
		{
			name: "empty secret value does not mask everything",
			sources: SourcedEnvVars{
				Secrets: []string{
					"EMPTY_SECRET=",
					"REAL_SECRET=actual_value",
				},
			},
			input:    "Testing empty secrets with actual_value here",
			expected: "Testing empty secrets with ******* here",
		},
		{
			name: "only empty secrets",
			sources: SourcedEnvVars{
				Secrets: []string{"EMPTY1=", "EMPTY2="},
			},
			input:    "This should not be masked at all",
			expected: "This should not be masked at all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := NewMasker(tt.sources)
			output := m.MaskString(tt.input)

			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestMasker_MaskBytes(t *testing.T) {
	t.Parallel()

	sources := SourcedEnvVars{
		Secrets: []string{"API_KEY=secret123"},
	}

	m := NewMasker(sources)
	input := []byte("The API key is secret123")
	output := m.MaskBytes(input)

	assert.Equal(t, []byte("The API key is *******"), output)
}

func TestSplitEnv(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantKey string
		wantVal string
	}{
		{
			name:    "valid env var",
			input:   "API_KEY=secret123",
			wantKey: "API_KEY",
			wantVal: "secret123",
		},
		{
			name:    "value with equals sign",
			input:   "BASE64=YWJjZGVm==",
			wantKey: "BASE64",
			wantVal: "YWJjZGVm==",
		},
		{
			name:    "no equals sign",
			input:   "INVALID",
			wantKey: "",
			wantVal: "",
		},
		{
			name:    "empty value",
			input:   "EMPTY=",
			wantKey: "EMPTY",
			wantVal: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			key, val := splitEnv(tt.input)

			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}
