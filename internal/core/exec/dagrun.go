package exec

import (
	"context"
	"encoding/hex"
	"errors"
	"hash/fnv"
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
	// FindSubAttempt finds a sub dag-run record by dag-run ID.
	FindSubAttempt(ctx context.Context, dagRun DAGRunRef, subDAGRunID string) (DAGRunAttempt, error)
	// CreateSubAttempt creates a new sub dag-run attempt under the root dag-run.
	// This is used for distributed sub-DAG execution where the coordinator needs
	// to create the attempt directory before the worker reports status.
	CreateSubAttempt(ctx context.Context, rootRef DAGRunRef, subDAGRunID string) (DAGRunAttempt, error)
	// RemoveOldDAGRuns deletes dag-run records older than retentionDays.
	// If retentionDays is negative, it won't delete any records.
	// If retentionDays is zero, it will delete all records for the DAG name.
	// But it will not delete the records with non-final statuses (e.g., running, queued).
	// Returns a list of dag-run IDs that were removed (or would be removed in dry-run mode).
	RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int, opts ...RemoveOldDAGRunsOption) ([]string, error)
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
	Tags      []string // Filter by DAG tags (AND logic - all tags must match)
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

// WithTags sets the tags filter for listing dag-runs (AND logic - all tags must match)
func WithTags(tags []string) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.Tags = tags
	}
}

// WithLimit sets the maximum number of results to return when listing dag-runs
func WithLimit(limit int) ListDAGRunStatusesOption {
	return func(o *ListDAGRunStatusesOptions) {
		o.Limit = limit
	}
}

// RemoveOldDAGRunsOptions contains options for removing old dag-runs
type RemoveOldDAGRunsOptions struct {
	// DryRun if true, only returns the paths that would be removed without actually deleting
	DryRun bool
}

// RemoveOldDAGRunsOption is a functional option for configuring RemoveOldDAGRunsOptions
type RemoveOldDAGRunsOption func(*RemoveOldDAGRunsOptions)

// WithDryRun sets the dry-run mode for removing old dag-runs
func WithDryRun() RemoveOldDAGRunsOption {
	return func(o *RemoveOldDAGRunsOptions) {
		o.DryRun = true
	}
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

// GenerateAttemptKey creates a globally unique attempt identifier for cancellation tracking.
// Format: FNV1a64 hash of hierarchy + ":" + attemptId (e.g., "a1b2c3d4e5f67890:abc123").
func GenerateAttemptKey(rootName, rootID, dagName, dagRunID, attemptID string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(rootName + "\x00" + rootID + "\x00" + dagName + "\x00" + dagRunID))
	return hex.EncodeToString(h.Sum(nil)) + ":" + attemptID
}
