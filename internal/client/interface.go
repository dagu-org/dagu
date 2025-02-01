package client

import (
	"context"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/frontend/gen/restapi/operations/dags"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/model"
)

type Client interface {
	CreateDAG(ctx context.Context, id string) (string, error)
	GetDAGSpec(ctx context.Context, id string) (string, error)
	Grep(ctx context.Context, pattern string) ([]*persistence.GrepResult, []string, error)
	Rename(ctx context.Context, oldID, newID string) error
	Stop(ctx context.Context, dag *digraph.DAG) error
	StartAsync(ctx context.Context, dag *digraph.DAG, opts StartOptions)
	Start(ctx context.Context, dag *digraph.DAG, opts StartOptions) error
	Restart(ctx context.Context, dag *digraph.DAG, opts RestartOptions) error
	Retry(ctx context.Context, dag *digraph.DAG, requestID string) error
	GetCurrentStatus(ctx context.Context, dag *digraph.DAG) (*model.Status, error)
	GetStatusByRequestID(ctx context.Context, dag *digraph.DAG, requestID string) (*model.Status, error)
	GetLatestStatus(ctx context.Context, dag *digraph.DAG) (model.Status, error)
	GetRecentHistory(ctx context.Context, dag *digraph.DAG, n int) []model.StatusFile
	UpdateStatus(ctx context.Context, dag *digraph.DAG, status model.Status) error
	UpdateDAG(ctx context.Context, id string, spec string) error
	DeleteDAG(ctx context.Context, id, loc string) error
	GetAllStatus(ctx context.Context) (statuses []DAGStatus, errs []string, err error)
	GetAllStatusPagination(ctx context.Context, params dags.ListDagsParams) ([]DAGStatus, *DagListPaginationSummaryResult, error)
	GetStatus(ctx context.Context, dagLocation string) (DAGStatus, error)
	IsSuspended(ctx context.Context, id string) bool
	ToggleSuspend(ctx context.Context, id string, suspend bool) error
	GetTagList(ctx context.Context) ([]string, []string, error)
}

type StartOptions struct {
	Params string
	Quiet  bool
}

type RestartOptions struct {
	Quiet bool
}

type DAGStatus struct {
	File      string
	Dir       string
	DAG       *digraph.DAG
	Status    model.Status
	Suspended bool
	Error     error
	ErrorT    *string
}

type DagListPaginationSummaryResult struct {
	PageCount int
	ErrorList []string
}

func newDAGStatus(
	dag *digraph.DAG, status model.Status, suspended bool, err error,
) DAGStatus {
	ret := DAGStatus{
		File:      filepath.Base(dag.Location),
		Dir:       filepath.Dir(dag.Location),
		DAG:       dag,
		Status:    status,
		Suspended: suspended,
		Error:     err,
	}
	if err != nil {
		errT := err.Error()
		ret.ErrorT = &errT
	}
	return ret
}
