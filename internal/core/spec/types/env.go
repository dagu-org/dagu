package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
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

// UnmarshalYAML implements yaml.Unmarshaler for EnvValue.
func (e *EnvValue) UnmarshalYAML(node *yaml.Node) error {
	e.isSet = true

	switch node.Kind {
	case yaml.MappingNode:
		// Map of key-value pairs
		var m map[string]any
		if err := node.Decode(&m); err != nil {
			return fmt.Errorf("env map decode error: %w", err)
		}
		e.raw = m
		return e.parseMap(m)

	case yaml.SequenceNode:
		// Array of maps or strings
		var arr []any
		if err := node.Decode(&arr); err != nil {
			return fmt.Errorf("env array decode error: %w", err)
		}
		e.raw = arr
		return e.parseArray(arr)

	default:
		return fmt.Errorf("env must be map or array, got %v", node.Tag)
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
