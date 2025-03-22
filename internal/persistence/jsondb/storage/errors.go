package storage

import "errors"

// Errors for storage package.
var (
	ErrRemoveDirectory    = errors.New("failed to remove directory")
	ErrMoveDirectory      = errors.New("failed to move directory")
	ErrCreateNewDirectory = errors.New("failed to create new directory")
)
