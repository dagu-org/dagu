package history

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
)

// Error variables for history operations
var (
	ErrRequestIDNotFound = errors.New("request id not found")
	ErrNoStatusData      = errors.New("no status data")
)

// Database provides an interface for interacting with the underlying database
// for storing and retrieving execution history records of DAGs.
// It abstracts the details of the storage mechanism, allowing for different
// implementations (e.g., file-based, in-memory, etc.) to be used interchangeably.
type Database interface {
	// Create creates a new history record for a DAG run
	Create(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, opts NewRecordOptions) (Record, error)
	// Recent returns the most recent history records for a DAG, limited by itemLimit
	Recent(ctx context.Context, name string, itemLimit int) []Record
	// Latest returns the most recent history record for a DAG
	Latest(ctx context.Context, name string) (Record, error)
	// Find finds a history record by its request ID
	Find(ctx context.Context, name string, reqID string) (Record, error)
	// FindSubRun finds a sub-run record by its request ID
	FindSubRun(ctx context.Context, name, reqID string, subRunID string) (Record, error)
	// RemoveOld removes history records older than retentionDays
	RemoveOld(ctx context.Context, name string, retentionDays int) error
	// Rename renames all history records from oldName to newName
	Rename(ctx context.Context, oldName, newName string) error
}

// NewRecordOptions contains options for creating a new history record
type NewRecordOptions struct {
	Root  *digraph.RootDAG
	Retry bool
}

// Record represents a single execution history record that can be read and written
type Record interface {
	// Open prepares the record for writing
	Open(ctx context.Context) error
	// Write updates the record with new status information
	Write(ctx context.Context, status Status) error
	// Close finalizes any pending operations on the record
	Close(ctx context.Context) error
	// ReadStatus retrieves the execution status for this record
	ReadStatus(ctx context.Context) (*Status, error)
	// ReadDAG retrieves the DAG definition for this record
	ReadDAG(ctx context.Context) (*digraph.DAG, error)
}
