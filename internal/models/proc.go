package models

import (
	"context"

	"github.com/dagu-org/dagu/internal/digraph"
)

// ProcStore is an interface for managing process storage.
type ProcStore interface {
	// Get retrieves a process by its workflow reference.
	Get(ctx context.Context, workflow digraph.WorkflowRef) (Proc, error)
	// Count retrieves the number of processes associated with a given workflow name.
	// It only counts the processes that are alive.
	Count(ctx context.Context, name string) (int, error)
}

// Proc represents a process that is associated with a workflow.
type Proc interface {
	// Start starts the heartbeat for the process.
	Start(ctx context.Context) error
	// Stop stops the heartbeat for the process.
	Stop(ctx context.Context) error
}
