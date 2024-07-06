package engine

import (
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/model"
)

type Engine interface {
	CreateDAG(name string) (string, error)
	GetDAGSpec(id string) (string, error)
	Grep(pattern string) ([]*persistence.GrepResult, []string, error)
	Rename(oldDAGPath, newDAGPath string) error
	Stop(dg *dag.DAG) error
	StartAsync(dg *dag.DAG, params string)
	Start(dg *dag.DAG, params string) error
	Restart(dg *dag.DAG) error
	Retry(dg *dag.DAG, reqID string) error
	GetCurrentStatus(dg *dag.DAG) (*model.Status, error)
	GetStatusByRequestID(dg *dag.DAG, reqID string) (*model.Status, error)
	GetLatestStatus(dg *dag.DAG) (*model.Status, error)
	GetRecentHistory(dg *dag.DAG, n int) []*model.StatusFile
	UpdateStatus(dg *dag.DAG, status *model.Status) error
	UpdateDAG(id string, spec string) error
	DeleteDAG(name, loc string) error
	GetAllStatus() (statuses []*persistence.DAGStatus, errs []string, err error)
	GetStatus(dagLocation string) (*persistence.DAGStatus, error)
	IsSuspended(id string) bool
	ToggleSuspend(id string, suspend bool) error
}
