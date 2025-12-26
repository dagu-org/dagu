package types

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

// LogOutputMode represents the mode for log output handling.
// It determines how stdout and stderr are written to log files.
type LogOutputMode string

const (
	// LogOutputSeparate keeps stdout and stderr in separate files (.out and .err).
	// This is the default behavior for backward compatibility.
	LogOutputSeparate LogOutputMode = "separate"

	// LogOutputMerged combines stdout and stderr into a single log file (.log).
	// Both streams are interleaved in the order they are written.
	LogOutputMerged LogOutputMode = "merged"
)

// LogOutputValue represents a log output configuration that can be unmarshaled from YAML.
// It accepts a string value that must be one of: "separate" or "merged".
type LogOutputValue struct {
	mode LogOutputMode
	set  bool // whether the value was explicitly set in YAML
}

// UnmarshalYAML implements BytesUnmarshaler for goccy/go-yaml.
func (l *LogOutputValue) UnmarshalYAML(data []byte) error {
	l.set = true

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("logOutput unmarshal error: %w", err)
	}

	switch v := raw.(type) {
	case string:
		value := strings.TrimSpace(strings.ToLower(v))
		switch value {
		case "separate", "":
			l.mode = LogOutputSeparate
		case "merged":
			l.mode = LogOutputMerged
		default:
			return fmt.Errorf("invalid logOutput value: %q (must be 'separate' or 'merged')", v)
		}
		return nil

	case nil:
		l.set = false
		return nil

	default:
		return fmt.Errorf("logOutput must be a string, got %T", v)
	}
}

// IsZero returns true if the value was not set in YAML.
func (l LogOutputValue) IsZero() bool {
	return !l.set
}

// Mode returns the log output mode.
// If the value was not set, it returns LogOutputSeparate as the default.
func (l LogOutputValue) Mode() LogOutputMode {
	if !l.set {
		return LogOutputSeparate
	}
	return l.mode
}

// String returns the string representation of the log output mode.
func (l LogOutputValue) String() string {
	return string(l.Mode())
}

// IsMerged returns true if the log output mode is merged.
func (l LogOutputValue) IsMerged() bool {
	return l.mode == LogOutputMerged
}

// IsSeparate returns true if the log output mode is separate.
func (l LogOutputValue) IsSeparate() bool {
	return !l.set || l.mode == LogOutputSeparate
}
