package models

import (
	"context"

	"github.com/dagu-org/dagu/internal/digraph"
)

// ProcStore is an interface for managing process storage.
type ProcStore interface {
	// Acquire creates a new process for a given DAG-run.
	// It automatically starts the heartbeat for the process.
	Acquire(ctx context.Context, dagRun digraph.DAGRunRef) (ProcHandle, error)
	// CountAlive retrieves the number of processes associated with a given DAG name.
	// It only counts the processes that are alive.
	CountAlive(ctx context.Context, name string) (int, error)
}

// ProcHandle represents a process that is associated with a DAG-run.
type ProcHandle interface {
	// Stop stops the heartbeat for the process.
	Stop(ctx context.Context) error
	// GetMeta retrieves the metadata for the process.
	GetMeta() ProcMeta
}

// ProcMeta is a struct that holds metadata for a process.
type ProcMeta struct {
	StartedAt int64
	Name      string
	DAGRunID  string
}
