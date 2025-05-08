package digraph

import (
	"errors"
	"strings"
)

// Errors for ExecRef
var (
	ErrInvalidExecRefFormat = errors.New("invalid ExecRef format")
)

// ExecRef represents a reference to an execution of a DAG.
type ExecRef struct {
	Name   string `json:"name,omitempty"`
	ExecID string `json:"execId,omitempty"`
}

// NewExecRef creates a new ExecRef with the given name and execID.
func NewExecRef(name, execID string) ExecRef {
	return ExecRef{
		Name:   name,
		ExecID: execID,
	}
}

// String returns a string representation of the ExecRef.
func (e ExecRef) String() string {
	return e.Name + ":" + e.ExecID
}

func (e ExecRef) IsZero() bool {
	return e == zeroRef
}

// ParseExecRef parses a string representation of an ExecRef and returns the ExecRef.
func ParseExecRef(s string) (ExecRef, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return ExecRef{}, ErrInvalidExecRefFormat
	}
	return NewExecRef(parts[0], parts[1]), nil
}

var zeroRef ExecRef
