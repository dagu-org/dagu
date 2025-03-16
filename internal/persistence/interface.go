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
	NewRecord(ctx context.Context, key string, timestamp time.Time, requestID string) Record
	Update(ctx context.Context, key, requestID string, status Status) error
	Recent(ctx context.Context, key string, itemLimit int) []Record
	Latest(ctx context.Context, key string) (Record, error)
	FindByRequestID(ctx context.Context, key string, requestID string) (Record, error)
	RemoveOld(ctx context.Context, key string, retentionDays int) error
	Rename(ctx context.Context, oldKey, newKey string) error
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
