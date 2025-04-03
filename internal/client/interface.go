package client

import (
	"context"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/persistence"
)

type Client interface {
	CreateDAG(ctx context.Context, name string) (string, error)
	GetDAGSpec(ctx context.Context, name string) (string, error)
	Grep(ctx context.Context, pattern string) ([]*persistence.GrepResult, []string, error)
	Rename(ctx context.Context, oldID, newID string) error
	Stop(ctx context.Context, dag *digraph.DAG) error
	StartAsync(ctx context.Context, dag *digraph.DAG, opts StartOptions)
	Start(ctx context.Context, dag *digraph.DAG, opts StartOptions) error
	Restart(ctx context.Context, dag *digraph.DAG, opts RestartOptions) error
	Retry(ctx context.Context, dag *digraph.DAG, requestID string) error
	GetCurrentStatus(ctx context.Context, dag *digraph.DAG) (*persistence.Status, error)
	GetStatusByRequestID(ctx context.Context, dag *digraph.DAG, requestID string) (*persistence.Status, error)
	GetLatestStatus(ctx context.Context, dag *digraph.DAG) (persistence.Status, error)
	GetRecentHistory(ctx context.Context, dag *digraph.DAG, n int) []persistence.Run
	UpdateStatus(ctx context.Context, dag *digraph.DAG, status persistence.Status) error
	UpdateDAG(ctx context.Context, name string, spec string) error
	DeleteDAG(ctx context.Context, name string) error
	GetAllStatus(ctx context.Context, opts ...GetAllStatusOption) ([]DAGStatus, *DagListPaginationSummaryResult, error)
	GetStatus(ctx context.Context, dagLocation string) (DAGStatus, error)
	IsSuspended(ctx context.Context, name string) bool
	ToggleSuspend(ctx context.Context, name string, suspend bool) error
	GetTagList(ctx context.Context) ([]string, []string, error)
}

type DagListPaginationSummaryResult struct {
	PageCount int
	ErrorList []string
}

type GetAllStatusOptions struct {
	// Number of items to return per page
	Limit *int
	// Page number (for pagination)
	Page *int
	// Filter DAGs by matching name
	Name *string
	// Filter DAGs by matching tag
	Tag *string
}

type GetAllStatusOption func(*GetAllStatusOptions)

func WithLimit(limit int) GetAllStatusOption {
	return func(opt *GetAllStatusOptions) {
		opt.Limit = &limit
	}
}

func WithPage(page int) GetAllStatusOption {
	return func(opt *GetAllStatusOptions) {
		opt.Page = &page
	}
}

func WithName(name string) GetAllStatusOption {
	return func(opt *GetAllStatusOptions) {
		opt.Name = &name
	}
}

func WithTag(tag string) GetAllStatusOption {
	return func(opt *GetAllStatusOptions) {
		opt.Tag = &tag
	}
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
	Status    persistence.Status
	Suspended bool
	Error     error
	ErrorT    *string
}

func newDAGStatus(
	dag *digraph.DAG, status persistence.Status, suspended bool, err error,
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
