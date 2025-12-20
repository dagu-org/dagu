package types

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// StringOrArray represents a value that can be specified as either a single
// string or an array of strings.
//
// YAML examples:
//
//	depends: "step1"
//	depends: ["step1", "step2"]
//	dotenv: ".env"
//	dotenv: [".env", ".env.local"]
type StringOrArray struct {
	raw    any      // Original value for error reporting
	isSet  bool     // Whether the field was set in YAML
	values []string // Parsed values
}

// UnmarshalYAML implements yaml.Unmarshaler for StringOrArray.
func (s *StringOrArray) UnmarshalYAML(node *yaml.Node) error {
	s.isSet = true

	switch node.Kind {
	case yaml.ScalarNode:
		// Single string value
		s.raw = node.Value
		if node.Value != "" {
			s.values = []string{node.Value}
		}
		return nil

	case yaml.SequenceNode:
		// Array of strings
		var arr []string
		if err := node.Decode(&arr); err != nil {
			return fmt.Errorf("array must contain strings: %w", err)
		}
		s.raw = arr
		s.values = arr
		return nil

	default:
		return fmt.Errorf("must be string or array, got %v", node.Tag)
	}
}

// IsZero returns true if the value was not set in YAML.
func (s StringOrArray) IsZero() bool { return !s.isSet }

// Value returns the original raw value for error reporting.
func (s StringOrArray) Value() any { return s.raw }

// Values returns the parsed string values.
func (s StringOrArray) Values() []string { return s.values }

// IsEmpty returns true if set but contains no values (empty array).
func (s StringOrArray) IsEmpty() bool { return s.isSet && len(s.values) == 0 }

// MailToValue is an alias for StringOrArray used for email recipients.
// YAML examples:
//
//	to: user@example.com
//	to: ["user1@example.com", "user2@example.com"]
type MailToValue = StringOrArray

// TagsValue is an alias for StringOrArray used for tags.
// YAML examples:
//
//	tags: production
//	tags: ["production", "critical"]
type TagsValue = StringOrArray
