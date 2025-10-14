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
				DAGEnv: []string{"API_KEY=secret123"},
			},
			input:    "The API key is secret123",
			expected: "The API key is *******",
		},
		{
			name: "multiple values",
			sources: SourcedEnvVars{
				DAGEnv: []string{
					"API_KEY=secret123",
					"PASSWORD=pass456",
				},
			},
			input:    "Key: secret123, Pass: pass456",
			expected: "Key: *******, Pass: *******",
		},
		{
			name: "with safelist",
			sources: SourcedEnvVars{
				DAGEnv:   []string{"API_KEY=secret123", "LOG_LEVEL=debug"},
				Safelist: []string{"LOG_LEVEL"},
			},
			input:    "Key: secret123, Level: debug",
			expected: "Key: *******, Level: debug",
		},
		{
			name: "min length filter",
			sources: SourcedEnvVars{
				DAGEnv: []string{"SHORT=ab", "LONG=longvalue"},
			},
			input:    "Short: ab, Long: longvalue",
			expected: "Short: ab, Long: *******",
		},
		{
			name: "overlapping values",
			sources: SourcedEnvVars{
				DAGEnv: []string{"SHORT=abc", "LONG=abcdef"},
			},
			input:    "Value: abcdef",
			expected: "Value: *******",
		},
		{
			name: "no match",
			sources: SourcedEnvVars{
				DAGEnv: []string{"API_KEY=secret123"},
			},
			input:    "This has no secrets",
			expected: "This has no secrets",
		},
		{
			name: "step env",
			sources: SourcedEnvVars{
				DAGEnv:  []string{"DAG_SECRET=dag123"},
				StepEnv: []string{"STEP_SECRET=step456"},
			},
			input:    "DAG: dag123, Step: step456",
			expected: "DAG: *******, Step: *******",
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
		DAGEnv: []string{"API_KEY=secret123"},
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
