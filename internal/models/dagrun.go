package models

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
)

// Errors related to DAG run management
var (
	ErrDAGRunIDNotFound = errors.New("DAG run ID not found")
	ErrNoStatusData     = errors.New("no status data")
)

// DAGRunStore provides an interface for interacting with the underlying database
// for storing and retrieving DAG-run data.
// It abstracts the details of the storage mechanism, allowing for different
// implementations (e.g., file-based, in-memory, etc.) to be used interchangeably.
type DAGRunStore interface {
	// CreateAttempt creates a new execution record for a workflow
	CreateAttempt(ctx context.Context, dag *digraph.DAG, ts time.Time, workflowID string, opts NewDAGRunAttemptOptions) (DAGRunAttempt, error)
	// RecentAttempts returns the most recent workflows for the given name, limited by itemLimit
	RecentAttempts(ctx context.Context, name string, itemLimit int) []DAGRunAttempt
	// LatestAttempt returns the most recent workflows for the given name
	LatestAttempt(ctx context.Context, name string) (DAGRunAttempt, error)
	// ListStatuses returns a list of statuses for the given workflow ID
	ListStatuses(ctx context.Context, opts ...ListDAGRunStatusesOption) ([]*DAGRunStatus, error)
	// FindAttempt finds a run by it's workflow ID
	FindAttempt(ctx context.Context, workflow digraph.DAGRunRef) (DAGRunAttempt, error)
	// FindChildAttempt finds a child workflow record by its workflow ID
	FindChildAttempt(ctx context.Context, dagRun digraph.DAGRunRef, childDAGRunID string) (DAGRunAttempt, error)
	// RemoveOldDAGRuns delete run data older than retentionDays
	RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int) error
	// RenameDAGRuns renames all run data from oldName to newName
	// The name means the DAG name, renaming it will allow user to manage those runs
	// with the new DAG name.
	RenameDAGRuns(ctx context.Context, oldName, newName string) error
}

// ListDAGRunStatusesOptions contains options for listing runs
type ListDAGRunStatusesOptions struct {
	DAGRunID  string
	Name      string
	ExactName string
	From      TimeInUTC
	To        TimeInUTC
	Statuses  []scheduler.Status
	Limit     int
}

// ListRunsOption is a functional option for configuring ListRunsOptions
type ListDAGRunStatusesOption func(*ListDAGRunStatusesOptions)

// WithFrom sets the start time for listing DAG runs
func WithFrom(from TimeInUTC) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.From = from
	}
}

// WithTo sets the end time for listing DAG runs
func WithTo(to TimeInUTC) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.To = to
	}
}

// WithStatuses sets the statuses for listing DAG runs
func WithStatuses(statuses []scheduler.Status) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.Statuses = statuses
	}
}

// WithExactName sets the name for listing DAG runs
func WithExactName(name string) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.ExactName = name
	}
}

// WithName sets the name for listing DAG runs
func WithName(name string) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.Name = name
	}
}

// WithDAGRunID sets the workflow ID for listing DAG runs
func WithDAGRunID(dagRunID string) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.DAGRunID = dagRunID
	}
}

// NewDAGRunAttemptOptions contains options for creating a new run record
type NewDAGRunAttemptOptions struct {
	RootDAGRun *digraph.DAGRunRef
	Retry      bool
}

// DAGRunAttempt represents a single execution of a workflow that can be read and written
type DAGRunAttempt interface {
	// ID returns the ID of the attempt, which is a unique identifier for the run
	ID() string
	// Open prepares the attempt for writing status updates
	Open(ctx context.Context) error
	// Write updates the status of the attempt
	Write(ctx context.Context, status DAGRunStatus) error
	// Close finalizes writing to the attempt
	Close(ctx context.Context) error
	// ReadStatus retrieves the current status of the attempt
	ReadStatus(ctx context.Context) (*DAGRunStatus, error)
	// ReadDAG reads the DAG associated with this run attempt
	ReadDAG(ctx context.Context) (*digraph.DAG, error)
}
