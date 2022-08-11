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

// DAGStatus is the struct to contain DAGStatus spec and status.
type DAGStatus struct {
	File      string
	Dir       string
	DAG       *dag.DAG
	Status    *models.Status
	Suspended bool
	Error     error
	ErrorT    *string
}

// ReadDAG loads DAG from config file.
func (dr *DAGReader) ReadDAG(file string, headOnly bool) (*DAGStatus, error) {
	cl := dag.Loader{}
	var d *dag.DAG
	var err error
	if headOnly {
		d, err = cl.LoadHeadOnly(file)
	} else {
		d, err = cl.LoadWithoutEval(file)
	}
	if err != nil {
		if d != nil {
			return dr.newDAG(d, defaultStatus(d), err), err
		}
		d := &dag.DAG{Path: file}
		d.Init()
		return dr.newDAG(d, defaultStatus(d), err), err
	}
	status, err := New(d).GetLastStatus()
	if err != nil {
		return nil, err
	}
	if !headOnly {
		if _, err := scheduler.NewExecutionGraph(d.Steps...); err != nil {
			return dr.newDAG(d, status, err), err
		}
	}
	return dr.newDAG(d, status, err), nil
}

func (dr *DAGReader) newDAG(d *dag.DAG, s *models.Status, err error) *DAGStatus {
	ret := &DAGStatus{
		File:      filepath.Base(d.Path),
		Dir:       filepath.Dir(d.Path),
		DAG:       d,
		Status:    s,
		Suspended: dr.suspendChecker.IsSuspended(d),
		Error:     err,
	}
	if err != nil {
		errT := err.Error()
		ret.ErrorT = &errT
	}
	return ret
}
