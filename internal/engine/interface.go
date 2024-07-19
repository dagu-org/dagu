package engine

import (
	"path/filepath"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
)

type Engine interface {
	CreateDAG(id string) (string, error)
	GetDAGSpec(id string) (string, error)
	Grep(pattern string) ([]*persistence.GrepResult, []string, error)
	Rename(oldID, newID string) error
	Stop(dg *dag.DAG) error
	StartAsync(dg *dag.DAG, opts StartOptions)
	Start(dg *dag.DAG, opts StartOptions) error
	Restart(dg *dag.DAG, opts RestartOptions) error
	Retry(dg *dag.DAG, reqID string) error
	GetCurrentStatus(dg *dag.DAG) (*model.Status, error)
	GetStatusByRequestID(dg *dag.DAG, reqID string) (*model.Status, error)
	GetLatestStatus(dg *dag.DAG) (*model.Status, error)
	GetRecentHistory(dg *dag.DAG, n int) []*model.StatusFile
	UpdateStatus(dg *dag.DAG, status *model.Status) error
	UpdateDAG(id string, spec string) error
	DeleteDAG(id, loc string) error
	GetAllStatus() (statuses []*DAGStatus, errs []string, err error)
	GetStatus(dagLocation string) (*DAGStatus, error)
	IsSuspended(id string) bool
	ToggleSuspend(id string, suspend bool) error
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
	DAG       *dag.DAG
	Status    *model.Status
	Suspended bool
	Error     error
	ErrorT    *string
}

func newDAGStatus(
	dg *dag.DAG, s *model.Status, suspended bool, err error,
) *DAGStatus {
	ret := &DAGStatus{
		File:      filepath.Base(dg.Location),
		Dir:       filepath.Dir(dg.Location),
		DAG:       dg,
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
