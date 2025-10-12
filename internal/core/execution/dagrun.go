package execution

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/core"
)

// Errors related to dag-run management
var (
	ErrDAGRunIDNotFound    = errors.New("dag-run ID not found")
	ErrNoStatusData        = errors.New("no status data")
	ErrCorruptedStatusFile = errors.New("corrupted status file") // Status file exists but contains no valid data or is corrupted
)

// DAGRunStore provides an interface for interacting with the underlying database
// for storing and retrieving dag-run data.
// It abstracts the details of the storage mechanism, allowing for different
// implementations (e.g., file-based, in-memory, etc.) to be used interchangeably.
type DAGRunStore interface {
	// CreateAttempt creates a new execution record for a dag-run.
	CreateAttempt(ctx context.Context, dag *core.DAG, ts time.Time, dagRunID string, opts NewDAGRunAttemptOptions) (DAGRunAttempt, error)
	// RecentAttempts returns the most recent dag-run's attempt for the DAG name, limited by itemLimit
	RecentAttempts(ctx context.Context, name string, itemLimit int) []DAGRunAttempt
	// LatestAttempt returns the most recent dag-run's attempt for the DAG name.
	LatestAttempt(ctx context.Context, name string) (DAGRunAttempt, error)
	// ListStatuses returns a list of statuses.
	ListStatuses(ctx context.Context, opts ...ListDAGRunStatusesOption) ([]*DAGRunStatus, error)
	// FindAttempt finds the latest attempt for the dag-run.
	FindAttempt(ctx context.Context, dagRun DAGRunRef) (DAGRunAttempt, error)
	// FindChildAttempt finds a child dag-run record by dag-run ID.
	FindChildAttempt(ctx context.Context, dagRun DAGRunRef, childDAGRunID string) (DAGRunAttempt, error)
	// RemoveOldDAGRuns delete dag-run records older than retentionDays
	// If retentionDays is negative, it won't delete any records.
	// If retentionDays is zero, it will delete all records for the DAG name.
	// But it will not delete the records with non-final statuses (e.g., running, queued).
	RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int) error
	// RenameDAGRuns renames all run data from oldName to newName
	// The name means the DAG name, renaming it will allow user to manage those runs
	// with the new DAG name.
	RenameDAGRuns(ctx context.Context, oldName, newName string) error
	// RemoveDAGRun removes a dag-run record by its reference.
	RemoveDAGRun(ctx context.Context, dagRun DAGRunRef) error
}

// ListDAGRunStatusesOptions contains options for listing runs
type ListDAGRunStatusesOptions struct {
	DAGRunID  string
	Name      string
	ExactName string
	From      TimeInUTC
	To        TimeInUTC
	Statuses  []core.Status
	Limit     int
}

// ListRunsOption is a functional option for configuring ListRunsOptions
type ListDAGRunStatusesOption func(*ListDAGRunStatusesOptions)

// WithFrom sets the start time for listing dag-runs
func WithFrom(from TimeInUTC) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.From = from
	}
}

// WithTo sets the end time for listing dag-runs
func WithTo(to TimeInUTC) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.To = to
	}
}

// WithStatuses sets the statuses for listing dag-runs
func WithStatuses(statuses []core.Status) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.Statuses = statuses
	}
}

// WithExactName sets the name for listing dag-runs
func WithExactName(name string) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.ExactName = name
	}
}

// WithName sets the name for listing dag-runs
func WithName(name string) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.Name = name
	}
}

// WithDAGRunID sets the dag-run ID for listing dag-runs
func WithDAGRunID(dagRunID string) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.DAGRunID = dagRunID
	}
}

// NewDAGRunAttemptOptions contains options for creating a new run record
type NewDAGRunAttemptOptions struct {
	// RootDAGRun is the root dag-run reference for this attempt.
	RootDAGRun *DAGRunRef
	// Retry indicates whether this is a retry of a previous run.
	Retry bool
}

// DAGRunAttempt represents a single execution of a dag-run to record the status and execution details.
type DAGRunAttempt interface {
	// ID returns the identifier for the attempt that is unique within the dag-run.
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
	ReadDAG(ctx context.Context) (*core.DAG, error)
	// RequestCancel requests cancellation of the dag-run attempt.
	RequestCancel(ctx context.Context) error
	// CancelRequested checks if a cancellation has been requested for this attempt.
	CancelRequested(ctx context.Context) (bool, error)
	// Hide marks the attempt as hidden from normal operations.
	// This is useful for preserving previous state visibility when dequeuing.
	Hide(ctx context.Context) error
	// Hidden returns true if the attempt is hidden from normal operations.
	Hidden() bool
}

// Errors for RunRef parsing
var (
	ErrInvalidRunRefFormat = errors.New("invalid dag-run reference format")
)

// DAGRunRef represents a reference to a dag-run
type DAGRunRef struct {
	Name string `json:"name,omitempty"`
	ID   string `json:"id,omitempty"`
}

// NewDAGRunRef creates a new reference to dag-run with the given DAG name and run ID.
// It is used to identify a specific dag-run.
func NewDAGRunRef(name, runID string) DAGRunRef {
	return DAGRunRef{
		Name: name,
		ID:   runID,
	}
}

// String returns a string representation of the dag-run reference.
func (e DAGRunRef) String() string {
	return e.Name + ":" + e.ID
}

// Zero checks if the DAGRunRef is a zero value.
func (e DAGRunRef) Zero() bool {
	return e == zeroRef
}

// ParseDAGRunRef parses a string into a DAGRunRef.
// The expected format is "name:runId".
// If the format is invalid, it returns an error.
func ParseDAGRunRef(s string) (DAGRunRef, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return DAGRunRef{}, ErrInvalidRunRefFormat
	}
	return NewDAGRunRef(parts[0], parts[1]), nil
}

// zeroRef is a zero value for DAGRunRef.
var zeroRef DAGRunRef
