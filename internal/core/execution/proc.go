package execution

import (
	"context"
)

// ProcStore is an interface for managing process storage.
type ProcStore interface {
	// Lock try to lock process group return error if it's held by another process
	Lock(ctx context.Context, groupName string) error
	// UnLock unlocks process group
	Unlock(ctx context.Context, groupName string)
	// Acquire creates a new process for a given group name and DAG-run reference.
	// It automatically starts the heartbeat for the process.
	Acquire(ctx context.Context, groupName string, dagRun DAGRunRef) (ProcHandle, error)
	// CountAlive retrieves the number of processes associated with a group name.
	CountAlive(ctx context.Context, groupName string) (int, error)
	// CountAlive retrieves the number of processes associated with a group name.
	CountAliveByDAGName(ctx context.Context, groupName, dagName string) (int, error)
	// IsRunAlive checks if a specific DAG run is currently alive.
	IsRunAlive(ctx context.Context, groupName string, dagRun DAGRunRef) (bool, error)
	// ListAlive returns list of running DAG runs by the group name.
	ListAlive(ctx context.Context, groupName string) ([]DAGRunRef, error)
	// ListAllAlive returns all running DAG runs across all groups.
	// Returns a map where key is the group name and value is list of DAG runs.
	ListAllAlive(ctx context.Context) (map[string][]DAGRunRef, error)
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
