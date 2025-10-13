package masking

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMasker_EmptySources(t *testing.T) {
	m := NewMasker(SourcedEnvVars{})

	assert.NotNil(t, m)
	assert.Empty(t, m.sensitiveVals)
}

func TestMasker_MaskString_SingleValue(t *testing.T) {
	sources := SourcedEnvVars{
		DAGEnv: []string{"API_KEY=secret123"},
	}

	m := NewMasker(sources)

	input := "The API key is secret123"
	output := m.MaskString(input)

	assert.Equal(t, "The API key is *******", output)
}

func TestMasker_MaskString_MultipleValues(t *testing.T) {
	sources := SourcedEnvVars{
		DAGEnv: []string{
			"API_KEY=secret123",
			"PASSWORD=pass456",
		},
	}

	m := NewMasker(sources)

	input := "Key: secret123, Pass: pass456"
	output := m.MaskString(input)

	assert.Equal(t, "Key: *******, Pass: *******", output)
}

func TestMasker_MaskString_WithSafelist(t *testing.T) {
	sources := SourcedEnvVars{
		DAGEnv: []string{
			"API_KEY=secret123",
			"LOG_LEVEL=debug",
		},
		Safelist: []string{"LOG_LEVEL"},
	}

	m := NewMasker(sources)

	input := "Key: secret123, Level: debug"
	output := m.MaskString(input)

	assert.Equal(t, "Key: *******, Level: debug", output)
}

func TestMasker_MaskString_MinLength(t *testing.T) {
	sources := SourcedEnvVars{
		DAGEnv: []string{
			"SHORT=ab",
			"LONG=longvalue",
		},
	}

	m := NewMasker(sources)

	input := "Short: ab, Long: longvalue"
	output := m.MaskString(input)

	// "ab" should not be masked (too short, min length is 3)
	// "longvalue" should be masked
	assert.Equal(t, "Short: ab, Long: *******", output)
}

func TestMasker_MaskString_OverlappingValues(t *testing.T) {
	sources := SourcedEnvVars{
		DAGEnv: []string{
			"SHORT=abc",
			"LONG=abcdef",
		},
	}

	m := NewMasker(sources)

	input := "Value: abcdef"
	output := m.MaskString(input)

	// Longer value should be masked first
	assert.Equal(t, "Value: *******", output)
}

func TestMasker_MaskString_NoMatch(t *testing.T) {
	sources := SourcedEnvVars{
		DAGEnv: []string{"API_KEY=secret123"},
	}

	m := NewMasker(sources)

	input := "This has no secrets"
	output := m.MaskString(input)

	assert.Equal(t, input, output) // Unchanged
}

func TestMasker_StepEnv(t *testing.T) {
	sources := SourcedEnvVars{
		DAGEnv:  []string{"DAG_SECRET=dag123"},
		StepEnv: []string{"STEP_SECRET=step456"},
	}

	m := NewMasker(sources)

	input := "DAG: dag123, Step: step456"
	output := m.MaskString(input)

	assert.Equal(t, "DAG: *******, Step: *******", output)
}

func TestMasker_MaskBytes(t *testing.T) {
	sources := SourcedEnvVars{
		DAGEnv: []string{"API_KEY=secret123"},
	}

	m := NewMasker(sources)

	input := []byte("The API key is secret123")
	output := m.MaskBytes(input)

	assert.Equal(t, []byte("The API key is *******"), output)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "*******", DefaultMaskString)
	assert.Equal(t, 3, DefaultMinLength)
}

func TestSplitEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKey  string
		wantVal  string
	}{
		{
			name:     "Valid env var",
			input:    "API_KEY=secret123",
			wantKey:  "API_KEY",
			wantVal:  "secret123",
		},
		{
			name:     "Value with equals sign",
			input:    "BASE64=YWJjZGVm==",
			wantKey:  "BASE64",
			wantVal:  "YWJjZGVm==",
		},
		{
			name:     "No equals sign",
			input:    "INVALID",
			wantKey:  "",
			wantVal:  "",
		},
		{
			name:     "Empty value",
			input:    "EMPTY=",
			wantKey:  "EMPTY",
			wantVal:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, val := splitEnv(tt.input)
			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, tt.wantVal, val)
		})
	}
}
