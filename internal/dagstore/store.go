package dagstore

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/internal/digraph"
)

// Errors for DAG file operations
var (
	ErrDAGAlreadyExists = errors.New("DAG already exists")
	ErrDAGNotFound      = errors.New("DAG is not found")
)

// Store manages the DAG files and their metadata (e.g., tags, suspend status).
type Store interface {
	// Create stores a new DAG definition with the given name and returns its ID
	Create(ctx context.Context, name string, spec []byte) (string, error)
	// Delete removes a DAG definition by name
	Delete(ctx context.Context, name string) error
	// List returns a paginated list of DAG definitions with filtering options
	List(ctx context.Context, params ListOptions) (PaginatedResult[*digraph.DAG], []string, error)
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
	// LoadSpec loads a DAG from a YAML file and returns the DAG object
	LoadSpec(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error)
	// TagList returns all unique tags across all DAGs with any errors encountered
	TagList(ctx context.Context) ([]string, []string, error)
	// ToggleSuspend changes the suspension state of a DAG by ID
	ToggleSuspend(id string, suspend bool) error
	// IsSuspended checks if a DAG is currently suspended
	IsSuspended(id string) bool
}

// ListOptions contains parameters for paginated DAG listing
type ListOptions struct {
	Paginator *Paginator
	Name      string // Optional name filter
	Tag       string // Optional tag filter
}

// ListResult contains the result of a paginated DAG listing operation
type ListResult struct {
	DAGs   []*digraph.DAG // The list of DAGs for the current page
	Count  int            // Total count of DAGs matching the filter
	Errors []string       // Any errors encountered during listing
}

// GrepResult represents the result of a pattern search within a DAG definition
type GrepResult struct {
	Name    string       // Name of the DAG
	DAG     *digraph.DAG // The DAG object
	Matches []*Match     // Matching lines and their context
}

// Match contains matched line number and line content.
type Match struct {
	Line       string
	LineNumber int
	StartLine  int
}
