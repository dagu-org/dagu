package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/persistence/grep"
)

var (
	ErrRequestIDNotFound = fmt.Errorf("request id not found")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

type HistoryStore interface {
	NewRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string) (Record, error)
	NewSubRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, reqID string, rootDAG digraph.RootDAG) (Record, error)
	Update(ctx context.Context, name, reqID string, status Status) error
	Recent(ctx context.Context, name string, itemLimit int) []Record
	Latest(ctx context.Context, name string) (Record, error)
	FindByRequestID(ctx context.Context, name string, reqID string) (Record, error)
	FindBySubRequestID(ctx context.Context, dagName, reqID string, rootDAG digraph.RootDAG) (Record, error)
	RemoveOld(ctx context.Context, name string, retentionDays int) error
	Rename(ctx context.Context, oldName, newName string) error
}

type Record interface {
	Open(ctx context.Context) error
	Write(ctx context.Context, status Status) error
	Close(ctx context.Context) error
	Read(ctx context.Context) (*StatusFile, error)
	ReadStatus(ctx context.Context) (*Status, error)
}

type DAGStore interface {
	Create(ctx context.Context, name string, spec []byte) (string, error)
	Delete(ctx context.Context, name string) error
	List(ctx context.Context) (ret []*digraph.DAG, errs []string, err error)
	ListPagination(ctx context.Context, params DAGListPaginationArgs) (*DagListPaginationResult, error)
	GetMetadata(ctx context.Context, name string) (*digraph.DAG, error)
	GetDetails(ctx context.Context, name string) (*digraph.DAG, error)
	Grep(ctx context.Context, pattern string) (ret []*GrepResult, errs []string, err error)
	Rename(ctx context.Context, oldID, newID string) error
	GetSpec(ctx context.Context, name string) (string, error)
	UpdateSpec(ctx context.Context, name string, spec []byte) error
	TagList(ctx context.Context) ([]string, []string, error)
}

type DAGListPaginationArgs struct {
	Page  int
	Limit int
	Name  string
	Tag   string
}

type DagListPaginationResult struct {
	DagList   []*digraph.DAG
	Count     int
	ErrorList []string
}

type GrepResult struct {
	Name    string
	DAG     *digraph.DAG
	Matches []*grep.Match
}

type FlagStore interface {
	ToggleSuspend(id string, suspend bool) error
	IsSuspended(id string) bool
}
