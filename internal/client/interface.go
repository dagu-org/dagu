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
	GetRecentHistory(ctx context.Context, name string, n int) []persistence.Run
	UpdateStatus(ctx context.Context, dag *digraph.DAG, status persistence.Status) error
	LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error)
	UpdateDAG(ctx context.Context, name string, spec string) error
	DeleteDAG(ctx context.Context, name string) error
	ListStatus(ctx context.Context, opts ...ListStatusOption) (*persistence.PaginatedResult[DAGStatus], []string, error)
	GetStatus(ctx context.Context, dagLocation string) (DAGStatus, error)
	IsSuspended(ctx context.Context, name string) bool
	ToggleSuspend(ctx context.Context, name string, suspend bool) error
	GetTagList(ctx context.Context) ([]string, []string, error)
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

type ListStatusOption func(*GetAllStatusOptions)

func WithLimit(limit int) ListStatusOption {
	return func(opt *GetAllStatusOptions) {
		opt.Limit = &limit
	}
}

func WithPage(page int) ListStatusOption {
	return func(opt *GetAllStatusOptions) {
		opt.Page = &page
	}
}

func WithName(name string) ListStatusOption {
	return func(opt *GetAllStatusOptions) {
		opt.Name = &name
	}
}

func WithTag(tag string) ListStatusOption {
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
}

// ErrorAsString converts the error to a string if it exists, otherwise returns an empty string.
func (s DAGStatus) ErrorAsString() string {
	if s.Error == nil {
		return ""
	}
	return s.Error.Error()
}

func newDAGStatus(
	dag *digraph.DAG, status persistence.Status, suspended bool, err error,
) DAGStatus {
	return DAGStatus{
		File:      filepath.Base(dag.Location),
		Dir:       filepath.Dir(dag.Location),
		DAG:       dag,
		Status:    status,
		Suspended: suspended,
		Error:     err,
	}
}
