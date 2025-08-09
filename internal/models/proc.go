package models

import (
	"context"

	"github.com/dagu-org/dagu/internal/digraph"
)

// ProcStore is an interface for managing process storage.
type ProcStore interface {
	// Acquire creates a new process for a given group name and DAG-run reference.
	// It automatically starts the heartbeat for the process.
	Acquire(ctx context.Context, groupName string, dagRun digraph.DAGRunRef) (ProcHandle, error)
	// CountAlive retrieves the number of processes associated with a group name.
	CountAlive(ctx context.Context, groupName string) (int, error)
	// IsRunAlive checks if a specific DAG run is currently alive.
	IsRunAlive(ctx context.Context, groupName string, dagRun digraph.DAGRunRef) (bool, error)
	// ListAlive returns list of running DAG runs by the group name.
	ListAlive(ctx context.Context, groupName string) ([]digraph.DAGRunRef, error)
}

// ProcHandle represents a process that is associated with a dag-run.
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
