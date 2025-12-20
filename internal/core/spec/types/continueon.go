package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ContinueOnValue represents a continue-on configuration that can be specified as:
// - A string shorthand: "skipped" or "failed"
// - A detailed map with configuration options
//
// YAML examples:
//
//	continueOn: skipped
//	continueOn: failed
//	continueOn:
//	  skipped: true
//	  failed: true
//	  exitCode: [0, 1]
//	  output: "pattern"
type ContinueOnValue struct {
	raw      any    // Original value for error reporting
	isSet    bool   // Whether the field was set in YAML
	skipped  bool   // Continue on skipped
	failed   bool   // Continue on failed
	exitCode []int  // Specific exit codes to continue on
	output   string // Output pattern to match
}

// UnmarshalYAML implements yaml.Unmarshaler for ContinueOnValue.
func (c *ContinueOnValue) UnmarshalYAML(node *yaml.Node) error {
	c.isSet = true

	switch node.Kind {
	case yaml.ScalarNode:
		// String shorthand: "skipped" or "failed"
		c.raw = node.Value
		switch strings.ToLower(strings.TrimSpace(node.Value)) {
		case "skipped":
			c.skipped = true
		case "failed":
			c.failed = true
		default:
			return fmt.Errorf("continueOn: expected 'skipped' or 'failed', got %q", node.Value)
		}
		return nil

	case yaml.MappingNode:
		// Detailed configuration
		var m map[string]any
		if err := node.Decode(&m); err != nil {
			return fmt.Errorf("continueOn map decode error: %w", err)
		}
		c.raw = m
		return c.parseMap(m)

	default:
		return fmt.Errorf("continueOn must be string or map, got %v", node.Tag)
	}
}

func (c *ContinueOnValue) parseMap(m map[string]any) error {
	for key, v := range m {
		switch key {
		case "skipped":
			if b, ok := v.(bool); ok {
				c.skipped = b
			} else {
				return fmt.Errorf("continueOn.skipped: expected bool, got %T", v)
			}
		case "failed":
			if b, ok := v.(bool); ok {
				c.failed = b
			} else {
				return fmt.Errorf("continueOn.failed: expected bool, got %T", v)
			}
		case "exitCode":
			codes, err := parseIntArray(v)
			if err != nil {
				return fmt.Errorf("continueOn.exitCode: %w", err)
			}
			c.exitCode = codes
		case "output":
			if s, ok := v.(string); ok {
				c.output = s
			} else {
				return fmt.Errorf("continueOn.output: expected string, got %T", v)
			}
		default:
			return fmt.Errorf("continueOn: unknown key %q", key)
		}
	}
	return nil
}

func parseIntArray(v any) ([]int, error) {
	switch val := v.(type) {
	case []any:
		var result []int
		for i, item := range val {
			n, ok := item.(int)
			if !ok {
				// Try float64 (common in YAML parsing)
				if f, ok := item.(float64); ok {
					n = int(f)
				} else {
					return nil, fmt.Errorf("[%d]: expected int, got %T", i, item)
				}
			}
			result = append(result, n)
		}
		return result, nil
	case int:
		return []int{val}, nil
	case float64:
		return []int{int(val)}, nil
	default:
		return nil, fmt.Errorf("expected int or array of ints, got %T", v)
	}
}

// IsZero returns true if continueOn was not set in YAML.
func (c ContinueOnValue) IsZero() bool { return !c.isSet }

// Value returns the original raw value for error reporting.
func (c ContinueOnValue) Value() any { return c.raw }

// Skipped returns true if should continue on skipped.
func (c ContinueOnValue) Skipped() bool { return c.skipped }

// Failed returns true if should continue on failed.
func (c ContinueOnValue) Failed() bool { return c.failed }

// ExitCode returns exit codes to continue on.
func (c ContinueOnValue) ExitCode() []int { return c.exitCode }

// Output returns output pattern to match.
func (c ContinueOnValue) Output() string { return c.output }
