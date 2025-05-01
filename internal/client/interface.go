package client

import (
	"context"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/persistence"
)

// DAGClient is an interface for managing DAG files based on their file name.
type DAGClient interface {
	CreateDAG(ctx context.Context, loc string) (string, error)
	GetDAGSpec(ctx context.Context, loc string) (string, error)
	GrepDAG(ctx context.Context, pattern string) ([]*persistence.GrepResult, []string, error)
	MoveDAG(ctx context.Context, oldLoc, newLoc string) error
	UpdateDAG(ctx context.Context, loc string, spec string) error
	DeleteDAG(ctx context.Context, loc string) error
	GetDAGStatus(ctx context.Context, loc string) (DAGStatus, error)
	IsSuspended(ctx context.Context, loc string) bool
	ToggleSuspend(ctx context.Context, loc string, suspend bool) error
	GetTagList(ctx context.Context) ([]string, []string, error)
	ListDAGs(ctx context.Context, opts ...ListDAGOption) (*persistence.PaginatedResult[DAGStatus], []string, error)
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

// RunClient is an interface for managing the execution of DAGs based on their DAG name and request ID.
// Note that DAG name may not be the same as the file name.
type RunClient interface {
	StopDAG(ctx context.Context, dag *digraph.DAG) error
	StartDAG(ctx context.Context, dag *digraph.DAG, opts StartOptions) error
	RestartDAG(ctx context.Context, dag *digraph.DAG, opts RestartOptions) error
	RetryDAG(ctx context.Context, dag *digraph.DAG, requestID string) error
	GetCurrentStatus(ctx context.Context, dag *digraph.DAG) (*persistence.Status, error)
	GetStatusByRequestID(ctx context.Context, dag *digraph.DAG, requestID string) (*persistence.Status, error)
	GetLatestStatus(ctx context.Context, dag *digraph.DAG) (persistence.Status, error)
	GetRecentHistory(ctx context.Context, name string, n int) []persistence.Run
	GetStatus(ctx context.Context, name string, requestID string) (*persistence.Status, error)
	UpdateStatus(ctx context.Context, name string, status persistence.Status) error
	LoadYAML(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error)

	// TODO: Remove the following methods
	// DAGClient methods
	CreateDAG(ctx context.Context, loc string) (string, error)
	GetDAGSpec(ctx context.Context, loc string) (string, error)
	GrepDAG(ctx context.Context, pattern string) ([]*persistence.GrepResult, []string, error)
	MoveDAG(ctx context.Context, oldLoc, newLoc string) error
	UpdateDAG(ctx context.Context, loc string, spec string) error
	DeleteDAG(ctx context.Context, loc string) error
	GetDAGStatus(ctx context.Context, loc string) (DAGStatus, error)
	IsSuspended(ctx context.Context, loc string) bool
	ToggleSuspend(ctx context.Context, loc string, suspend bool) error
	GetTagList(ctx context.Context) ([]string, []string, error)
	ListDAGs(ctx context.Context, opts ...ListDAGOption) (*persistence.PaginatedResult[DAGStatus], []string, error)
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
	var file string
	if dag.Location != "" {
		file = dag.Location
	}
	return DAGStatus{
		File:      file,
		DAG:       dag,
		Status:    status,
		Suspended: suspended,
		Error:     err,
	}
}
