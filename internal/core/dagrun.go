package core

import (
	"errors"
	"strings"
)

// Errors for RunRef parsing
var (
	ErrInvalidRunRefFormat = errors.New("invalid dag-run reference format")
)

// DAGRunRef represents a reference to a dag-run
type DAGRunRef struct {
	Name string `json:"name,omitempty"`
	ID   string `json:"id,omitempty"`
}

// NewDAGRunRef creates a new reference to dag-run with the given DAG name and run ID.
// It is used to identify a specific dag-run.
func NewDAGRunRef(name, runID string) DAGRunRef {
	return DAGRunRef{
		Name: name,
		ID:   runID,
	}
}

// String returns a string representation of the dag-run reference.
func (e DAGRunRef) String() string {
	return e.Name + ":" + e.ID
}

// Zero checks if the DAGRunRef is a zero value.
func (e DAGRunRef) Zero() bool {
	return e == zeroRef
}

// ParseDAGRunRef parses a string into a DAGRunRef.
// The expected format is "name:runId".
// If the format is invalid, it returns an error.
func ParseDAGRunRef(s string) (DAGRunRef, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return DAGRunRef{}, ErrInvalidRunRefFormat
	}
	return NewDAGRunRef(parts[0], parts[1]), nil
}

// zeroRef is a zero value for DAGRunRef.
var zeroRef DAGRunRef
