package controller

import (
	"path/filepath"

	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/storage"
	"github.com/yohamta/dagu/internal/suspend"
)

type DAGReader struct {
	suspendChecker *suspend.SuspendChecker
}

func NewDAGReader() *DAGReader {
	return &DAGReader{
		suspendChecker: suspend.NewSuspendChecker(
			storage.NewStorage(
				settings.MustGet(settings.SETTING__SUSPEND_FLAGS_DIR),
			),
		),
	}
}

// DAG is the struct to contain DAG configuration and status.
type DAG struct {
	File      string
	Dir       string
	Config    *dag.DAG
	Status    *models.Status
	Suspended bool
	Error     error
	ErrorT    *string
}

// ReadDAG loads DAG from config file.
func (dr *DAGReader) ReadDAG(file string, headOnly bool) (*DAG, error) {
	cl := dag.Loader{}
	var cfg *dag.DAG
	var err error
	if headOnly {
		cfg, err = cl.LoadHeadOnly(file)
	} else {
		cfg, err = cl.LoadWithoutEval(file)
	}
	if err != nil {
		if cfg != nil {
			return dr.newDAG(cfg, defaultStatus(cfg), err), err
		}
		cfg := &dag.DAG{ConfigPath: file}
		cfg.Init()
		return dr.newDAG(cfg, defaultStatus(cfg), err), err
	}
	status, err := New(cfg).GetLastStatus()
	if err != nil {
		return nil, err
	}
	if !headOnly {
		if _, err := scheduler.NewExecutionGraph(cfg.Steps...); err != nil {
			return dr.newDAG(cfg, status, err), err
		}
	}
	return dr.newDAG(cfg, status, err), nil
}

func (dr *DAGReader) newDAG(cfg *dag.DAG, s *models.Status, err error) *DAG {
	ret := &DAG{
		File:      filepath.Base(cfg.ConfigPath),
		Dir:       filepath.Dir(cfg.ConfigPath),
		Config:    cfg,
		Status:    s,
		Suspended: dr.suspendChecker.IsSuspended(cfg),
		Error:     err,
	}
	if err != nil {
		errT := err.Error()
		ret.ErrorT = &errT
	}
	return ret
}
