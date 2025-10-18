package masking

import (
	"strings"
)

const (
	// DefaultMaskString is the default replacement string for masked values
	DefaultMaskString = "*******"
)

// SourcedEnvVars groups environment variables by their source
type SourcedEnvVars struct {
	Secrets []string // Environment variables from secrets
}

// Masker provides masking functionality for sensitive data
type Masker struct {
	sensitiveVals map[string]bool // Set of values to mask
}

// NewMasker creates a masker from sourced environment variables
func NewMasker(sources SourcedEnvVars) *Masker {
	sensitiveVals := make(map[string]bool)

	for _, env := range sources.Secrets {
		_, val := splitEnv(env)
		// Skip empty or invalid values to avoid masking everything
		// (strings.ReplaceAll with empty string would insert mask between every character)
		if val != "" {
			sensitiveVals[val] = true
		}
	}

	return &Masker{
		sensitiveVals: sensitiveVals,
	}
}

// MaskString replaces sensitive values in the input string
func (m *Masker) MaskString(input string) string {
	if len(m.sensitiveVals) == 0 {
		return input // Fast path
	}

	// Sort values by length (longest first) to avoid partial matches
	values := make([]string, 0, len(m.sensitiveVals))
	for val := range m.sensitiveVals {
		values = append(values, val)
	}

	// Simple sort by length (descending)
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if len(values[j]) > len(values[i]) {
				values[i], values[j] = values[j], values[i]
			}
		}
	}

	result := input
	for _, val := range values {
		result = strings.ReplaceAll(result, val, DefaultMaskString)
	}

	return result
}

// MaskBytes replaces sensitive values in the input bytes
func (m *Masker) MaskBytes(input []byte) []byte {
	return []byte(m.MaskString(string(input)))
}

// splitEnv splits "KEY=value" into (KEY, value)
func splitEnv(env string) (string, string) {
	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
