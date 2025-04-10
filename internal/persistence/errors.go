package persistence

import "errors"

var (
	ErrDAGNotFound       = errors.New("DAG is not found")
	ErrRequestIDNotFound = errors.New("request id not found")
	ErrNoStatusData      = errors.New("no status data")
)
