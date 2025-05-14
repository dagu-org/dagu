package models

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
)

// Error variables for history operations
var (
	ErrWorkflowIDNotFound = errors.New("workflow ID not found")
	ErrNoStatusData       = errors.New("no status data")
)

// HistoryRepository provides an interface for interacting with the underlying database
// for storing and retrieving workflow run data.
// It abstracts the details of the storage mechanism, allowing for different
// implementations (e.g., file-based, in-memory, etc.) to be used interchangeably.
type HistoryRepository interface {
	// CreateRun creates a new execution record for a workflow
	CreateRun(ctx context.Context, dag *digraph.DAG, ts time.Time, workflowID string, opts NewRunOptions) (Run, error)
	// RecentRuns returns the most recent workflows for the given name, limited by itemLimit
	RecentRuns(ctx context.Context, name string, itemLimit int) []Run
	// LatestRun returns the most recent workflows for the given name
	LatestRun(ctx context.Context, name string) (Run, error)
	// ListStatuses returns a list of statuses for the given workflow ID
	ListStatuses(ctx context.Context, opts ...ListStatusesOption) ([]*Status, error)
	// FindRun finds a run by it's workflow ID
	FindRun(ctx context.Context, workflow digraph.WorkflowRef) (Run, error)
	// FindChildWorkflowRun finds a child workflow record by its workflow ID
	FindChildWorkflowRun(ctx context.Context, workflow digraph.WorkflowRef, childWorkflowID string) (Run, error)
	// RemoveOldWorkflows delete run data older than retentionDays
	RemoveOldWorkflows(ctx context.Context, name string, retentionDays int) error
	// RenameWorkflows renames all run data from oldName to newName
	RenameWorkflows(ctx context.Context, oldName, newName string) error
}

// ListStatusesOptions contains options for listing runs
type ListStatusesOptions struct {
	WorkflowID string
	Name       string
	ExactName  string
	From       TimeInUTC
	To         TimeInUTC
	Statuses   []scheduler.Status
	Limit      int
}

// ListRunsOption is a functional option for configuring ListRunsOptions
type ListStatusesOption func(*ListStatusesOptions)

// WithFrom sets the start time for listing workflows
func WithFrom(from TimeInUTC) ListStatusesOption {
	return func(o *ListStatusesOptions) {
		o.From = from
	}
}

// WithTo sets the end time for listing workflows
func WithTo(to TimeInUTC) ListStatusesOption {
	return func(o *ListStatusesOptions) {
		o.To = to
	}
}

// WithStatuses sets the statuses for listing workflows
func WithStatuses(statuses []scheduler.Status) ListStatusesOption {
	return func(o *ListStatusesOptions) {
		o.Statuses = statuses
	}
}

// WithExactName sets the name for listing workflows
func WithExactName(name string) ListStatusesOption {
	return func(o *ListStatusesOptions) {
		o.ExactName = name
	}
}

// WithName sets the name for listing workflows
func WithName(name string) ListStatusesOption {
	return func(o *ListStatusesOptions) {
		o.Name = name
	}
}

// WithWorkflowID sets the workflow ID for listing workflows
func WithWorkflowID(workflowID string) ListStatusesOption {
	return func(o *ListStatusesOptions) {
		o.WorkflowID = workflowID
	}
}

// NewRunOptions contains options for creating a new run record
type NewRunOptions struct {
	Root  *digraph.WorkflowRef
	Retry bool
}

// Run represents a single execution of a workflow that can be read and written
type Run interface {
	// ID returns the ID of the run
	ID() string
	// Open prepares the run for writing
	Open(ctx context.Context) error
	// Write updates the run with new status information
	Write(ctx context.Context, status Status) error
	// Close finalizes any pending operations on the run
	Close(ctx context.Context) error
	// ReadStatus retrieves the execution status for this run
	ReadStatus(ctx context.Context) (*Status, error)
	// ReadDAG retrieves the DAG definition for this run
	ReadDAG(ctx context.Context) (*digraph.DAG, error)
}
