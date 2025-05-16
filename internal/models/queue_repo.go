package models

import (
	"context"

	"github.com/dagu-org/dagu/internal/digraph"
)

// JobQueueRepository provides an interface for interacting with the underlying database
// for storing and retrieving queued workflows.
type JobQueueRepository interface {
	// Enqueue adds a workflow to the queue
	Enqueue(ctx context.Context, name string, ref digraph.WorkflowRef) error
	// Dequeue removes a workflow from the queue
	Dequeue(ctx context.Context, name string, id string) (QueuedJob, error)
	// UpdateStatus updates the status of a queued workflow
	UpdateStatus(ctx context.Context, name string, id string, status JobQueueState) error
	// Delete deletes a workflow from the queue
	Delete(ctx context.Context, name string, id string) error
}

// QueuedJob represents a workflow that is in the queue for execution
type QueuedJob interface {
	// ID returns the ID of the queued workflow
	ID() string
	// WorkflowRef returns the reference of the queued workflow
	WorkflowRef() digraph.WorkflowRef // Status returns the status of the queued workflow
	// Status returns the status of the queued workflow
	Status() JobQueueState
}

// JobQueueState represents the status of a queued workflow
type JobQueueState int

const (
	// JobQueueStatusNone indicates that the workflow is not in the queue
	JobQueueStatusNone JobQueueState = iota
	// JobQueueStatusQueued indicates that the workflow is in the queue
	JobQueueStatusQueued
	// JobQueueStatusRunning indicates that the workflow is currently running
	JobQueueStatusRunning
	// JobQueueStatusCompleted indicates that the workflow has completed
	JobQueueStatusCompleted
)
