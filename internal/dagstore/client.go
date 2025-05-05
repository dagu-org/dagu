package dagstore

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/runstore"
)

// NewClient creates a new DAG dagClient.
func NewClient(
	runCli runstore.Client,
	dagStore Store,
) Client {
	return Client{
		runClient: runCli,
		dagStore:  dagStore,
	}
}

type Client struct {
	runClient runstore.Client
	dagStore  Store
}

var (
	dagTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)
)

func (cli *Client) GetSpec(ctx context.Context, id string) (string, error) {
	return cli.dagStore.GetSpec(ctx, id)
}

func (cli *Client) Create(ctx context.Context, name string) (string, error) {
	id, err := cli.dagStore.Create(ctx, name, dagTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to create DAG: %w", err)
	}
	return id, nil
}

func (cli *Client) Grep(ctx context.Context, pattern string) (
	[]*GrepResult, []string, error,
) {
	return cli.dagStore.Grep(ctx, pattern)
}

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

func (cli *Client) Update(ctx context.Context, id string, spec string) error {
	return cli.dagStore.UpdateSpec(ctx, id, []byte(spec))
}

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

func (cli *Client) getDAG(ctx context.Context, loc string) (*digraph.DAG, error) {
	dagDetail, err := cli.dagStore.GetDetails(ctx, loc)
	return cli.emptyDAGIfNil(dagDetail, loc), err
}

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

func (cli *Client) ToggleSuspend(_ context.Context, loc string, suspend bool) error {
	return cli.dagStore.ToggleSuspend(loc, suspend)
}

func (*Client) emptyDAGIfNil(dag *digraph.DAG, dagLocation string) *digraph.DAG {
	if dag != nil {
		return dag
	}
	return &digraph.DAG{Location: dagLocation}
}

func (cli *Client) IsSuspended(_ context.Context, id string) bool {
	return cli.dagStore.IsSuspended(id)
}

func (cli *Client) TagList(ctx context.Context) ([]string, []string, error) {
	return cli.dagStore.TagList(ctx)
}

func ptrOf[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
