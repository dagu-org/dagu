package runstore

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

// Store provides an interface for managing the execution data of DAGs.
type Store interface {
	// NewRecord creates a new history record for a DAG run
	NewRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, opts NewRecordOptions) (Record, error)
	// Update updates the status of an existing record identified by name and reqID
	Update(ctx context.Context, name, reqID string, status Status) error
	// Recent returns the most recent history records for a DAG, limited by itemLimit
	Recent(ctx context.Context, name string, itemLimit int) []Record
	// Latest returns the most recent history record for a DAG
	Latest(ctx context.Context, name string) (Record, error)
	// FindByRequestID finds a history record by its request ID
	FindByRequestID(ctx context.Context, name string, reqID string) (Record, error)
	// FindBySubRequestID finds a sub-DAG history record by its request ID
	FindBySubRequestID(ctx context.Context, reqID string, rootDAG digraph.RootDAG) (Record, error)
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
	// ReadRun retrieves the run metadata for this record
	ReadRun(ctx context.Context) (*Run, error)
	// ReadStatus retrieves the execution status for this record
	ReadStatus(ctx context.Context) (*Status, error)
	// ReadDAG retrieves the DAG definition for this record
	ReadDAG(ctx context.Context) (*digraph.DAG, error)
}

// Run represents metadata about a DAG run
type Run struct {
	File   string
	Status Status
}

// NewRun creates a new Run instance with the specified file path and status
func NewRun(file string, status Status) *Run {
	return &Run{
		File:   file,
		Status: status,
	}
}
