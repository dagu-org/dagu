package dagstore

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/runstore"
)

// New creates a new Store instance.
func New(runCli runstore.Client, driver Driver) Store {
	return Store{Driver: driver, runClient: runCli}
}

// Store provides operations for managing DAGs in the DAG store.
// It wraps the underlying DAG store and run client to provide a unified interface
// for DAG operations.
type Store struct {
	Driver

	runClient runstore.Client // Client for interacting with run history
}

// dagTemplate is the default template used when creating a new DAG.
// It contains a minimal DAG with a single step that echoes "hello".
var dagTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)

// Create creates a new DAG with the given name using the default template.
// It returns the ID of the newly created DAG or an error if creation fails.
func (store *Store) Create(ctx context.Context, name string) (string, error) {
	id, err := store.Driver.Create(ctx, name, dagTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to create DAG: %w", err)
	}
	return id, nil
}

// Move relocates a DAG from one location to another.
// It updates both the DAG's location in the store and its run history.
// Returns an error if any part of the move operation fails.
func (store *Store) Move(ctx context.Context, oldLoc, newLoc string) error {
	oldDAG, err := store.GetMetadata(ctx, oldLoc)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", oldLoc, err)
	}
	if err := store.Rename(ctx, oldLoc, newLoc); err != nil {
		return err
	}
	newDAG, err := store.GetMetadata(ctx, newLoc)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", newLoc, err)
	}
	if err := store.runClient.Rename(ctx, oldDAG.Name, newDAG.Name); err != nil {
		return fmt.Errorf("failed to rename history for %s: %w", oldLoc, err)
	}
	return nil
}

// ListDAGOptions defines the options for listing DAGs from the DAG store.
type ListDAGOptions struct {
	// Number of items to return per page
	Limit *int
	// Page number (for pagination)
	Page *int
	// Filter DAGs by matching name
	Name *string
	// Filter DAGs by matching tag
	Tag *string
}

// List retrieves a paginated list of DAGs with their statuses.
// It accepts optional functional options for configuring the listing operation.
// Returns a paginated result containing DAG statuses, a list of errors encountered
// during the listing, and an error if the listing operation fails.
func (store *Store) List(ctx context.Context, opts ...ListDAGOption) (*PaginatedResult[Status], []string, error) {
	var options ListDAGOptions
	for _, opt := range opts {
		opt(&options)
	}
	if options.Limit == nil {
		options.Limit = new(int)
		*options.Limit = 100
	}
	if options.Page == nil {
		options.Page = new(int)
		*options.Page = 1
	}

	pg := NewPaginator(*options.Page, *options.Limit)

	dags, errList, err := store.Driver.List(ctx, ListOptions{
		Paginator: &pg,
		Name:      ptrOf(options.Name),
		Tag:       ptrOf(options.Tag),
	})
	if err != nil {
		return nil, errList, err
	}

	var items []Status
	for _, d := range dags.Items {
		status, err := store.runClient.GetLatestStatus(ctx, d)
		if err != nil {
			errList = append(errList, err.Error())
		}
		items = append(items, Status{
			DAG:       d,
			Status:    status,
			Suspended: store.IsSuspended(ctx, d.Location),
			Error:     err,
		})
	}

	r := NewPaginatedResult(items, dags.TotalCount, pg)
	return &r, errList, nil
}

// ListDAGOption is a functional option type for configuring ListDAGOptions.
type ListDAGOption func(*ListDAGOptions)

// WithLimit sets the limit for the number of items to return per page.
func WithLimit(limit int) ListDAGOption {
	return func(opt *ListDAGOptions) {
		opt.Limit = &limit
	}
}

// WithPage sets the page number for pagination.
func WithPage(page int) ListDAGOption {
	return func(opt *ListDAGOptions) {
		opt.Page = &page
	}
}

// WithName sets the file name filter for the DAGs to be listed.
func WithName(name string) ListDAGOption {
	return func(opt *ListDAGOptions) {
		opt.Name = &name
	}
}

// WithTag sets the tag filter for the DAGs to be listed.
func WithTag(tag string) ListDAGOption {
	return func(opt *ListDAGOptions) {
		opt.Tag = &tag
	}
}

// getDAG retrieves a DAG by its location.
// It returns the DAG and any error encountered during retrieval.
// If the DAG is nil but a location is provided, it returns an empty DAG with the location set.
func (store *Store) getDAG(ctx context.Context, loc string) (*digraph.DAG, error) {
	dagDetail, err := store.GetDetails(ctx, loc)
	return store.emptyDAGIfNil(dagDetail, loc), err
}

// Status retrieves the status of a DAG by its location.
// It returns a Status object containing the DAG, its latest run status,
// whether it's suspended, and any error encountered during retrieval.
func (store *Store) Status(ctx context.Context, loc string) (Status, error) {
	dag, err := store.getDAG(ctx, loc)
	if dag == nil {
		// TODO: fix not to use location
		dag = &digraph.DAG{Name: loc, Location: loc}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(dag.Steps...)
	}
	latestStatus, _ := store.runClient.GetLatestStatus(ctx, dag)
	return NewStatus(
		dag, latestStatus, store.IsSuspended(ctx, loc), err,
	), err
}

// emptyDAGIfNil returns the provided DAG if it's not nil,
// otherwise it returns an empty DAG with the provided location.
// This is a helper method to avoid nil pointer dereferences.
func (*Store) emptyDAGIfNil(dag *digraph.DAG, dagLocation string) *digraph.DAG {
	if dag != nil {
		return dag
	}
	return &digraph.DAG{Location: dagLocation}
}

// ptrOf returns the value pointed to by p, or the zero value of type T if p is nil.
// This is a generic helper function for safely dereferencing pointers.
func ptrOf[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
