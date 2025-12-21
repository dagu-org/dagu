package types

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

// EnvValue represents environment variable configuration that can be specified as:
// - A map of key-value pairs
// - An array of maps (for ordered definitions)
// - An array of "KEY=value" strings
// - A mix of maps and strings in an array
//
// YAML examples:
//
//	env:
//	  KEY1: value1
//	  KEY2: value2
//
//	env:
//	  - KEY1: value1
//	  - KEY2: value2
//
//	env:
//	  - KEY1=value1
//	  - KEY2=value2
type EnvValue struct {
	raw     any        // Original value for error reporting
	isSet   bool       // Whether the field was set in YAML
	entries []EnvEntry // Parsed entries in order
}

// EnvEntry represents a single environment variable entry.
type EnvEntry struct {
	Key   string
	Value string
}

// UnmarshalYAML implements BytesUnmarshaler for goccy/go-yaml.
func (e *EnvValue) UnmarshalYAML(data []byte) error {
	e.isSet = true

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("env unmarshal error: %w", err)
	}
	e.raw = raw

	switch v := raw.(type) {
	case map[string]any:
		// Map of key-value pairs
		return e.parseMap(v)

	case []any:
		// Array of maps or strings
		return e.parseArray(v)

	case nil:
		e.isSet = false
		return nil

	default:
		return fmt.Errorf("env must be map or array, got %T", v)
	}
}

func (e *EnvValue) parseMap(m map[string]any) error {
	for key, v := range m {
		value := stringifyValue(v)
		e.entries = append(e.entries, EnvEntry{Key: key, Value: value})
	}
	return nil
}

func (e *EnvValue) parseArray(arr []any) error {
	for i, item := range arr {
		switch v := item.(type) {
		case map[string]any:
			for key, val := range v {
				value := stringifyValue(val)
				e.entries = append(e.entries, EnvEntry{Key: key, Value: value})
			}
		case string:
			key, val, found := strings.Cut(v, "=")
			if !found {
				return fmt.Errorf("env[%d]: invalid format %q (expected KEY=value)", i, v)
			}
			e.entries = append(e.entries, EnvEntry{Key: key, Value: val})
		default:
			return fmt.Errorf("env[%d]: expected map or string, got %T", i, item)
		}
	}
	return nil
}

func stringifyValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// IsZero returns true if env was not set in YAML.
func (e EnvValue) IsZero() bool { return !e.isSet }

// Value returns the original raw value for error reporting.
func (e EnvValue) Value() any { return e.raw }

// Entries returns the parsed environment entries in order.
func (e EnvValue) Entries() []EnvEntry { return e.entries }
