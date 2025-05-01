package client

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/sock"
)

// NewDAGClient creates a new DAG dagClient.
func NewDAGClient(
	runClient RunClient,
	dagStore persistence.DAGStore,
	flagStore persistence.FlagStore,
) DAGClient {
	return &dagClient{
		runClient: runClient,
		dagStore:  dagStore,
		flagStore: flagStore,
	}
}

var _ DAGClient = (*dagClient)(nil)

type dagClient struct {
	runClient RunClient
	dagStore  persistence.DAGStore
	flagStore persistence.FlagStore
}

var (
	dagTemplate = []byte(`steps:
  - name: step1
    command: echo hello
`)
)

func (e *dagClient) GetDAGSpec(ctx context.Context, id string) (string, error) {
	return e.dagStore.GetSpec(ctx, id)
}

func (e *dagClient) CreateDAG(ctx context.Context, name string) (string, error) {
	id, err := e.dagStore.Create(ctx, name, dagTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to create DAG: %w", err)
	}
	return id, nil
}

func (e *dagClient) GrepDAG(ctx context.Context, pattern string) (
	[]*persistence.GrepResult, []string, error,
) {
	return e.dagStore.Grep(ctx, pattern)
}

func (e *dagClient) MoveDAG(_ context.Context, oldLoc, newLoc string) error {
	panic("not implemented") // TODO: Implement this function
	// oldDAG, err := e.dagStore.GetMetadata(ctx, oldLoc)
	// if err != nil {
	// 	return fmt.Errorf("failed to get metadata for %s: %w", oldLoc, err)
	// }
	// if err := e.dagStore.Rename(ctx, oldLoc, newLoc); err != nil {
	// 	return err
	// }
	// newDAG, err := e.dagStore.GetMetadata(ctx, newLoc)
	// if err != nil {
	// 	return fmt.Errorf("failed to get metadata for %s: %w", newLoc, err)
	// }
	// if err := e.historyStore.Rename(ctx, oldDAG.Name, newDAG.Name); err != nil {
	// 	return fmt.Errorf("failed to rename history for %s: %w", oldLoc, err)
	// }
	// return nil
}

func (e *dagClient) StopDAG(ctx context.Context, dag *digraph.DAG) error {
	logger.Info(ctx, "Stopping", "name", dag.Name)
	addr := dag.SockAddr("") // FIXME: Should handle the case of dynamic DAG
	if !fileutil.FileExists(addr) {
		logger.Info(ctx, "The DAG is not running", "name", dag.Name)
		return nil
	}
	dagClient := sock.NewClient(addr)
	_, err := dagClient.Request("POST", "/stop")
	return err
}

func (e *dagClient) UpdateDAG(ctx context.Context, id string, spec string) error {
	return e.dagStore.UpdateSpec(ctx, id, []byte(spec))
}

func (e *dagClient) DeleteDAG(ctx context.Context, name string) error {
	return e.dagStore.Delete(ctx, name)
}

func (e *dagClient) ListDAGs(ctx context.Context, opts ...ListDAGOption) (*persistence.PaginatedResult[DAGStatus], []string, error) {
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

	pg := persistence.NewPaginator(*options.Page, *options.Limit)

	dags, errList, err := e.dagStore.List(ctx, persistence.ListOptions{
		Paginator: &pg,
		Name:      ptr(options.Name),
		Tag:       ptr(options.Tag),
	})
	if err != nil {
		return nil, errList, err
	}

	var items []DAGStatus
	for _, d := range dags.Items {
		status, err := e.runClient.GetLatestStatus(ctx, d)
		if err != nil {
			errList = append(errList, err.Error())
		}
		items = append(items, DAGStatus{
			DAG:       d,
			Status:    status,
			Suspended: e.IsSuspended(ctx, d.Location),
			Error:     err,
		})
	}

	r := persistence.NewPaginatedResult(items, dags.TotalCount, pg)
	return &r, errList, nil
}

func (e *dagClient) getDAG(ctx context.Context, loc string) (*digraph.DAG, error) {
	dagDetail, err := e.dagStore.GetDetails(ctx, loc)
	return e.emptyDAGIfNil(dagDetail, loc), err
}

func (e *dagClient) GetDAGStatus(ctx context.Context, loc string) (DAGStatus, error) {
	dag, err := e.getDAG(ctx, loc)
	if dag == nil {
		// TODO: fix not to use location
		dag = &digraph.DAG{Name: loc, Location: loc}
	}
	if err == nil {
		// check the dag is correct in terms of graph
		_, err = scheduler.NewExecutionGraph(dag.Steps...)
	}
	latestStatus, _ := e.runClient.GetLatestStatus(ctx, dag)
	return newDAGStatus(
		dag, latestStatus, e.IsSuspended(ctx, loc), err,
	), err
}

func (e *dagClient) ToggleSuspend(_ context.Context, loc string, suspend bool) error {
	return e.flagStore.ToggleSuspend(loc, suspend)
}

func (*dagClient) emptyDAGIfNil(dag *digraph.DAG, dagLocation string) *digraph.DAG {
	if dag != nil {
		return dag
	}
	return &digraph.DAG{Location: dagLocation}
}

func (e *dagClient) IsSuspended(_ context.Context, id string) bool {
	return e.flagStore.IsSuspended(id)
}

func (e *dagClient) GetTagList(ctx context.Context) ([]string, []string, error) {
	return e.dagStore.TagList(ctx)
}
