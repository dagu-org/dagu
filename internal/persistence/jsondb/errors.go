package jsondb

import "errors"

// Error definitions for directory structure validation
var (
	// ErrInvalidRunDir is returned when an run directory has an invalid format
	// and cannot be parsed to extract timestamp and request ID information.
	ErrInvalidRunDir = errors.New("invalid run directory")
)
