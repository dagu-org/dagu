package client

import (
	"context"

	"github.com/dagu-org/dagu/internal/persistence"
)

// DAGClient is an interface for managing DAG files.
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
