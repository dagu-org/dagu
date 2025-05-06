package dagstore

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/runstore"
)

// NewClient creates a new DAG client.
// It takes a run client for interacting with run history and a DAG store for
// managing DAG specifications and metadata.
func NewClient(
	runCli runstore.Client,
	dagStore Store,
) Client {
	return Client{
		runClient: runCli,
		dagStore:  dagStore,
	}
}

// Client provides operations for managing DAGs in the DAG store.
// It wraps the underlying DAG store and run client to provide a unified interface
// for DAG operations.
type Client struct {
	runClient runstore.Client // Client for interacting with run history
	dagStore  Store           // Store for DAG specifications and metadata
}

var (
	// dagTemplate is the default template used when creating a new DAG.
	// It contains a minimal DAG with a single step that echoes "hello".
	dagTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)
)

// GetSpec retrieves the YAML specification of a DAG by its ID.
// It returns the specification as a string or an error if the DAG cannot be found.
func (cli *Client) GetSpec(ctx context.Context, id string) (string, error) {
	return cli.dagStore.GetSpec(ctx, id)
}

// Create creates a new DAG with the given name using the default template.
// It returns the ID of the newly created DAG or an error if creation fails.
func (cli *Client) Create(ctx context.Context, name string) (string, error) {
	id, err := cli.dagStore.Create(ctx, name, dagTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to create DAG: %w", err)
	}
	return id, nil
}

// Grep searches for DAGs matching the given pattern.
// It returns a list of grep results, a list of errors encountered during the search,
// and an error if the search operation fails.
func (cli *Client) Grep(ctx context.Context, pattern string) (
	[]*GrepResult, []string, error,
) {
	return cli.dagStore.Grep(ctx, pattern)
}

// Move relocates a DAG from one location to another.
// It updates both the DAG's location in the store and its run history.
// Returns an error if any part of the move operation fails.
func (cli *Client) Move(ctx context.Context, oldLoc, newLoc string) error {
	oldDAG, err := cli.dagStore.GetMetadata(ctx, oldLoc)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", oldLoc, err)
	}
	if err := cli.dagStore.Rename(ctx, oldLoc, newLoc); err != nil {
		return err
	}
	newDAG, err := cli.dagStore.GetMetadata(ctx, newLoc)
	if err != nil {
		return fmt.Errorf("failed to get metadata for %s: %w", newLoc, err)
	}
	if err := cli.runClient.Rename(ctx, oldDAG.Name, newDAG.Name); err != nil {
		return fmt.Errorf("failed to rename history for %s: %w", oldLoc, err)
	}
	return nil
}

// Update modifies the specification of an existing DAG.
// It takes the DAG ID and the new specification as a string.
// Returns an error if the update operation fails.
func (cli *Client) Update(ctx context.Context, id string, spec string) error {
	return cli.dagStore.UpdateSpec(ctx, id, []byte(spec))
}

// Delete removes a DAG from the store.
// It takes the name of the DAG to delete.
// Returns an error if the delete operation fails.
func (cli *Client) Delete(ctx context.Context, name string) error {
	return cli.dagStore.Delete(ctx, name)
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
func (cli *Client) List(ctx context.Context, opts ...ListDAGOption) (*PaginatedResult[Status], []string, error) {
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

	dags, errList, err := cli.dagStore.List(ctx, ListOptions{
		Paginator: &pg,
		Name:      ptrOf(options.Name),
		Tag:       ptrOf(options.Tag),
	})
	if err != nil {
		return nil, errList, err
	}

	var items []Status
	for _, d := range dags.Items {
		status, err := cli.runClient.GetLatestStatus(ctx, d)
		if err != nil {
			errList = append(errList, err.Error())
		}
		items = append(items, Status{
			DAG:       d,
			Status:    status,
			Suspended: cli.IsSuspended(ctx, d.Location),
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
func (cli *Client) getDAG(ctx context.Context, loc string) (*digraph.DAG, error) {
	dagDetail, err := cli.dagStore.GetDetails(ctx, loc)
	return cli.emptyDAGIfNil(dagDetail, loc), err
}

// Status retrieves the status of a DAG by its location.
// It returns a Status object containing the DAG, its latest run status,
// whether it's suspended, and any error encountered during retrieval.
func (cli *Client) Status(ctx context.Context, loc string) (Status, error) {
	dag, err := cli.getDAG(ctx, loc)
	if dag == nil {
		// TODO: fix not to use location
		dag = &digraph.DAG{Name: loc, Location: loc}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(dag.Steps...)
	}
	latestStatus, _ := cli.runClient.GetLatestStatus(ctx, dag)
	return NewStatus(
		dag, latestStatus, cli.IsSuspended(ctx, loc), err,
	), err
}

// ToggleSuspend changes the suspension state of a DAG.
// It takes the location of the DAG and a boolean indicating whether to suspend it.
// Returns an error if the operation fails.
func (cli *Client) ToggleSuspend(_ context.Context, loc string, suspend bool) error {
	return cli.dagStore.ToggleSuspend(loc, suspend)
}

// emptyDAGIfNil returns the provided DAG if it's not nil,
// otherwise it returns an empty DAG with the provided location.
// This is a helper method to avoid nil pointer dereferences.
func (*Client) emptyDAGIfNil(dag *digraph.DAG, dagLocation string) *digraph.DAG {
	if dag != nil {
		return dag
	}
	return &digraph.DAG{Location: dagLocation}
}

// IsSuspended checks if a DAG is currently suspended.
// It takes the ID of the DAG to check.
// Returns true if the DAG is suspended, false otherwise.
func (cli *Client) IsSuspended(_ context.Context, id string) bool {
	return cli.dagStore.IsSuspended(id)
}

// TagList retrieves a list of all tags used in the DAG store.
// It returns a list of tags, a list of errors encountered during retrieval,
// and an error if the operation fails.
func (cli *Client) TagList(ctx context.Context) ([]string, []string, error) {
	return cli.dagStore.TagList(ctx)
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
