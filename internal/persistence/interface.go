package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/persistence/grep"
	"github.com/dagu-org/dagu/internal/persistence/model"
)

var (
	ErrRequestIDNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

type HistoryStore interface {
	Open(ctx context.Context, key string, timestamp time.Time, requestID string) error
	Write(ctx context.Context, status model.Status) error
	Close(ctx context.Context) error
	Update(ctx context.Context, key, requestID string, status model.Status) error
	ReadStatusRecent(ctx context.Context, key string, itemLimit int) []model.StatusFile
	ReadStatusToday(ctx context.Context, key string) (*model.Status, error)
	FindByRequestID(ctx context.Context, key string, requestID string) (*model.StatusFile, error)
	RemoveAll(ctx context.Context, key string) error
	RemoveOld(ctx context.Context, key string, retentionDays int) error
	Rename(ctx context.Context, oldKey, newKey string) error
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
