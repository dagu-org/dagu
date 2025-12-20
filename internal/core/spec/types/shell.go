package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ShellValue represents a shell configuration that can be specified as either
// a string (e.g., "bash -e") or an array (e.g., ["bash", "-e"]).
//
// YAML examples:
//
//	shell: "bash -e"
//	shell: bash
//	shell: ["bash", "-e"]
//	shell:
//	  - bash
//	  - -e
type ShellValue struct {
	raw       any      // Original value for error reporting
	isSet     bool     // Whether the field was set in YAML
	command   string   // The shell command (first element)
	arguments []string // Additional arguments
}

// UnmarshalYAML implements yaml.Unmarshaler for ShellValue.
func (s *ShellValue) UnmarshalYAML(node *yaml.Node) error {
	s.isSet = true

	switch node.Kind {
	case yaml.ScalarNode:
		// String value: "bash -e" or "bash"
		s.raw = node.Value
		s.command = strings.TrimSpace(node.Value)
		return nil

	case yaml.SequenceNode:
		// Array value: ["bash", "-e"]
		var arr []string
		if err := node.Decode(&arr); err != nil {
			return fmt.Errorf("shell array must contain strings: %w", err)
		}
		s.raw = arr
		if len(arr) > 0 {
			s.command = arr[0]
			s.arguments = arr[1:]
		}
		return nil

	default:
		return fmt.Errorf("shell must be string or array, got %v", node.Tag)
	}
}

// IsZero returns true if the shell value was not set in YAML.
func (s ShellValue) IsZero() bool { return !s.isSet }

// Value returns the original raw value for error reporting.
func (s ShellValue) Value() any { return s.raw }

// Command returns the shell command (first element or full string).
func (s ShellValue) Command() string { return s.command }

// Arguments returns the additional shell arguments (only set for array form).
func (s ShellValue) Arguments() []string { return s.arguments }

// IsArray returns true if the value was specified as an array.
func (s ShellValue) IsArray() bool {
	_, ok := s.raw.([]string)
	return ok
}
