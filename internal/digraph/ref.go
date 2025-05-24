package digraph

import (
	"errors"
	"strings"
)

// Errors for ExecRef
var (
	ErrInvalidWorkflowRefFormat = errors.New("invalid workflow-ref format")
)

// WorkflowRef represents a reference to a workflow execution
type WorkflowRef struct {
	Name       string `json:"name,omitempty"`
	WorkflowID string `json:"workflowId,omitempty"`
}

// NewWorkflowRef creates a new WorkflowRef with the given name and workflow ID.
// It is used to identify a specific execution of a workflow.
func NewWorkflowRef(name, workflowID string) WorkflowRef {
	return WorkflowRef{
		Name:       name,
		WorkflowID: workflowID,
	}
}

// String returns a string representation of the ExecRef.
func (e WorkflowRef) String() string {
	return e.Name + ":" + e.WorkflowID
}

// Zero checks if the WorkflowRef is a zero value.
func (e WorkflowRef) Zero() bool {
	return e == zeroRef
}

// ParseWorkflowRef parses a string representation of a WorkflowRef.
// The expected format is "name:workflowID".
func ParseWorkflowRef(s string) (WorkflowRef, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return WorkflowRef{}, ErrInvalidWorkflowRefFormat
	}
	return NewWorkflowRef(parts[0], parts[1]), nil
}

// zeroRef is a zero value for WorkflowRef, used for comparison.
var zeroRef WorkflowRef
