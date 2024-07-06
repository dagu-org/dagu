package persistence

import (
	"fmt"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/grep"
	"github.com/dagu-dev/dagu/internal/persistence/model"
)

var (
	ErrRequestIDNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

type DataStoreFactory interface {
	NewHistoryStore() HistoryStore
	NewDAGStore() DAGStore
	NewFlagStore() FlagStore
}

type HistoryStore interface {
	Open(dagFile string, t time.Time, reqID string) error
	Write(status *model.Status) error
	Close() error
	Update(dagFile, reqID string, st *model.Status) error
	ReadStatusRecent(dagFile string, n int) []*model.StatusFile
	ReadStatusToday(dagFile string) (*model.Status, error)
	FindByRequestID(dagFile string, reqID string) (*model.StatusFile, error)
	RemoveAll(dagFile string) error
	RemoveOld(dagFile string, retentionDays int) error
	Rename(oldName, newName string) error
}

type DAGStore interface {
	Create(name string, spec []byte) (string, error)
	Delete(name string) error
	List() (ret []*dag.DAG, errs []string, err error)
	GetMetadata(name string) (*dag.DAG, error)
	GetDetails(name string) (*dag.DAG, error)
	Grep(pattern string) (ret []*GrepResult, errs []string, err error)
	Load(name string) (*dag.DAG, error)
	Rename(oldName, newName string) error
	GetSpec(name string) (string, error)
	UpdateSpec(name string, spec []byte) error
	Find(name string) (*dag.DAG, error)
}

type FlagStore interface {
	ToggleSuspend(id string, suspend bool) error
	IsSuspended(id string) bool
}

type GrepResult struct {
	Name    string
	DAG     *dag.DAG
	Matches []*grep.Match
}
