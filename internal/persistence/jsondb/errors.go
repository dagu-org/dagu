// Package jsondb provides a JSON-based persistence implementation for DAG execution history.
package jsondb

import "errors"

// Error definitions for directory structure validation
var (
	// ErrInvalidExecutionDir is returned when an execution directory has an invalid format
	// and cannot be parsed to extract timestamp and request ID information.
	ErrInvalidExecutionDir = errors.New("invalid execution directory")
)
