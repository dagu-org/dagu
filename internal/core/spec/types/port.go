package types

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// PortValue represents a port number that can be specified as either
// a string or an integer.
//
// YAML examples:
//
//	port: 22
//	port: "22"
type PortValue struct {
	raw   any    // Original value for error reporting
	isSet bool   // Whether the field was set in YAML
	value string // Normalized string value
}

// UnmarshalYAML implements yaml.Unmarshaler for PortValue.
func (p *PortValue) UnmarshalYAML(node *yaml.Node) error {
	p.isSet = true
	p.raw = node.Value

	switch node.Kind {
	case yaml.ScalarNode:
		p.value = node.Value
		return nil
	default:
		return fmt.Errorf("port must be string or number, got %v", node.Tag)
	}
}

// IsZero returns true if the port was not set in YAML.
func (p PortValue) IsZero() bool { return !p.isSet }

// Value returns the original raw value for error reporting.
func (p PortValue) Value() any { return p.raw }

// String returns the port as a string.
func (p PortValue) String() string { return p.value }
