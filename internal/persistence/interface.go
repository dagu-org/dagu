package persistence

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/grep"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"path/filepath"
	"time"
)

var (
	ErrRequestIdNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

type (
	DataStoreFactory interface {
		NewHistoryStore() HistoryStore
		NewDAGStore() DAGStore
		NewFlagStore() FlagStore
	}

	HistoryStore interface {
		Open(dagFile string, t time.Time, requestId string) error
		Write(st *model.Status) error
		Close() error
		Update(dagFile, requestId string, st *model.Status) error
		ReadStatusRecent(dagFile string, n int) []*model.StatusFile
		ReadStatusToday(dagFile string) (*model.Status, error)
		FindByRequestId(dagFile string, requestId string) (*model.StatusFile, error)
		RemoveAll(dagFile string) error
		RemoveOld(dagFile string, retentionDays int) error
		Rename(oldName, newName string) error
	}

	DAGStore interface {
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
	}

	FlagStore interface {
		ToggleSuspend(id string, suspend bool) error
		IsSuspended(id string) bool
	}

	GrepResult struct {
		Name    string
		DAG     *dag.DAG
		Matches []*grep.Match
	}

	DAGStatus struct {
		File      string
		Dir       string
		DAG       *dag.DAG
		Status    *model.Status
		Suspended bool
		Error     error
		ErrorT    *string
	}
)

func NewDAGStatus(d *dag.DAG, s *model.Status, suspended bool, err error) *DAGStatus {
	ret := &DAGStatus{
		File:      filepath.Base(d.Location),
		Dir:       filepath.Dir(d.Location),
		DAG:       d,
		Status:    s,
		Suspended: suspended,
		Error:     err,
	}
	if err != nil {
		errT := err.Error()
		ret.ErrorT = &errT
	}
	return ret
}
