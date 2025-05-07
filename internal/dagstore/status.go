package dagstore

import (
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/history"
)

func NewStatus(
	dag *digraph.DAG, status history.Status, suspended bool, err error,
) Status {
	var file string
	if dag.Location != "" {
		file = dag.Location
	}
	return Status{
		File:      file,
		DAG:       dag,
		Status:    status,
		Suspended: suspended,
		Error:     err,
	}
}

type Status struct {
	File      string
	DAG       *digraph.DAG
	Status    history.Status
	Suspended bool
	Error     error
}

// ErrorAsString converts the error to a string if it exists, otherwise returns an empty string.
func (s Status) ErrorAsString() string {
	if s.Error == nil {
		return ""
	}
	return s.Error.Error()
}
