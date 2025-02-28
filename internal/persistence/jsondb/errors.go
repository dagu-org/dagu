package jsondb

import "errors"

var (
	ErrStatusFileOpen    = errors.New("status file already open")
	ErrStatusFileNotOpen = errors.New("status file not open")
)
