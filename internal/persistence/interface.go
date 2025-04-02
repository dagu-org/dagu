package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/persistence/grep"
)

var (
	ErrRequestIDNotFound = fmt.Errorf("request id not found")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

// HistoryStore manages execution history records for DAGs
type HistoryStore interface {
	// NewRecord creates a new history record for a DAG run
	NewRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string) (Record, error)
	// NewSubRecord creates a new history record for a sub-DAG run
	NewSubRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, rootDAG digraph.RootDAG) (Record, error)
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
}

// DAGStore manages storage and retrieval of DAG definitions
type DAGStore interface {
	// Create stores a new DAG definition with the given name and returns its ID
	Create(ctx context.Context, name string, spec []byte) (string, error)
	// Delete removes a DAG definition by name
	Delete(ctx context.Context, name string) error
	// List returns all DAG definitions with any errors encountered during loading
	List(ctx context.Context) (ret []*digraph.DAG, errs []string, err error)
	// ListPagination returns a paginated list of DAG definitions with filtering options
	ListPagination(ctx context.Context, params DAGListPaginationArgs) (*DagListPaginationResult, error)
	// GetMetadata retrieves only the metadata of a DAG definition (faster than full load)
	GetMetadata(ctx context.Context, name string) (*digraph.DAG, error)
	// GetDetails retrieves the complete DAG definition including all fields
	GetDetails(ctx context.Context, name string) (*digraph.DAG, error)
	// Grep searches for a pattern in all DAG definitions and returns matching results
	Grep(ctx context.Context, pattern string) (ret []*GrepResult, errs []string, err error)
	// Rename changes a DAG's identifier from oldID to newID
	Rename(ctx context.Context, oldID, newID string) error
	// GetSpec retrieves the raw YAML specification of a DAG
	GetSpec(ctx context.Context, name string) (string, error)
	// UpdateSpec modifies the specification of an existing DAG
	UpdateSpec(ctx context.Context, name string, spec []byte) error
	// TagList returns all unique tags across all DAGs with any errors encountered
	TagList(ctx context.Context) ([]string, []string, error)
}

// DAGListPaginationArgs contains parameters for paginated DAG listing
type DAGListPaginationArgs struct {
	Page  int    // Page number (1-based)
	Limit int    // Maximum number of items per page
	Name  string // Optional name filter
	Tag   string // Optional tag filter
}

// DagListPaginationResult contains the result of a paginated DAG listing operation
type DagListPaginationResult struct {
	DagList   []*digraph.DAG // The list of DAGs for the current page
	Count     int            // Total count of DAGs matching the filter
	ErrorList []string       // Any errors encountered during listing
}

// GrepResult represents the result of a pattern search within a DAG definition
type GrepResult struct {
	Name    string        // Name of the DAG
	DAG     *digraph.DAG  // The DAG object
	Matches []*grep.Match // Matching lines and their context
}

// FlagStore manages persistent flags for DAGs such as suspension state
type FlagStore interface {
	// ToggleSuspend changes the suspension state of a DAG by ID
	ToggleSuspend(id string, suspend bool) error
	// IsSuspended checks if a DAG is currently suspended
	IsSuspended(id string) bool
}
